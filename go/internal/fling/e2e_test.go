// Package fling 提供 FLiNG 修改器网站 E2E 集成烟雾测试。
//
// 本文件包含搜索管线、缓存、配置往返、符号替换和清洗的端到端测试。
package fling

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mock HTTP 服务器路径常量，供 index_test.go 和 e2e_test.go 共用。
const (
	mockPathArchive = "/archive"
	mockPathMain    = "/main"
)

// =============================================================================
// E2E test fixtures — mock server serving testdata HTML
// =============================================================================

// newE2EFLiNGServer 创建一个 httptest.Server，返回 testdata 中的 HTML fixtures。
//
// 在 /archive 路径返回 fling_archive.html，/main 路径返回 fling_main.html。
func newE2EFLiNGServer(t *testing.T) *httptest.Server {
	t.Helper()
	archiveHTML := loadFixture(t, "fling_archive.html")
	mainHTML := loadFixture(t, "fling_main.html")

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case mockPathArchive:
			_, err := w.Write(archiveHTML)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case mockPathMain:
			_, err := w.Write(mainHTML)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

// =============================================================================
// Scenario 1: Happy Search — "DREDGE" keyword
// =============================================================================

// TestE2E_HappySearchDREDGE 测试从 FLiNG 主站搜索 "DREDGE" 的完整流程。
//
// Given: mock HTTP 服务器提供 testdata 中的真实 HTML fixtures
// When: 构建索引并搜索 "DREDGE"
// Then: 至少 1 条结果，且包含 Origin 为 "fling_main" 的条目
func TestE2E_HappySearchDREDGE(t *testing.T) {
	server := newE2EFLiNGServer(t)
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	idx, err := BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("BuildIndex 失败: %v", err)
	}

	results := SearchTrainers(idx.Archive, idx.Main, "DREDGE")
	if len(results) == 0 {
		t.Fatal("搜索 DREDGE 应返回结果，但为空")
	}

	foundMain := false
	for _, r := range results {
		if r.Origin == originMain {
			foundMain = true
			break
		}
	}
	if !foundMain {
		t.Errorf("搜索结果中应包含 Origin 为 %q 的条目，got %d 条结果", originMain, len(results))
	}
}

// =============================================================================
// Scenario 2: No results — "zzzznotagame" keyword
// =============================================================================

// TestE2E_NoResults 测试不存在的游戏名返回空结果。
//
// Given: mock HTTP 服务器提供 testdata fixtures
// When: 构建索引并搜索 "zzzznotagame"
// Then: 返回空结果
func TestE2E_NoResults(t *testing.T) {
	server := newE2EFLiNGServer(t)
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	idx, err := BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("BuildIndex 失败: %v", err)
	}

	results := SearchTrainers(idx.Archive, idx.Main, "zzzznotagame")
	if len(results) != 0 {
		t.Errorf("搜索不存在的游戏应返回空: got %d 条结果", len(results))
	}
}

// =============================================================================
// Scenario 3: Cache hit — BuildIndex twice, second call uses cache
// =============================================================================

// TestE2E_CacheHit 测试第二次 BuildIndex 命中缓存，无 HTTP 请求。
//
// Given: mock HTTP 服务器和 mock fetcher
// When: 调用 BuildIndex 两次（缓存未过期）
// Then: 第二次调用不发起新的 HTTP 请求（fetchCount 保持为 1）
func TestE2E_CacheHit(t *testing.T) {
	server := newE2EFLiNGServer(t)
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	// 第一次 BuildIndex：发起 HTTP 请求
	_, err := BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("首次 BuildIndex 失败: %v", err)
	}

	archiveCount1 := mock.FetchCount("fling-data/cache/fling_archive.html")
	mainCount1 := mock.FetchCount("fling-data/cache/fling_main.html")
	if archiveCount1 != 1 {
		t.Errorf("首次 archive 抓取次数: got %d, want 1", archiveCount1)
	}
	if mainCount1 != 1 {
		t.Errorf("首次 main 抓取次数: got %d, want 1", mainCount1)
	}

	// 第二次 BuildIndex：命中缓存，不发起 HTTP 请求
	_, err = BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("第二次 BuildIndex 失败: %v", err)
	}

	if got := mock.FetchCount("fling-data/cache/fling_archive.html"); got != 1 {
		t.Errorf("缓存命中后 archive 抓取次数: got %d, want 1", got)
	}
	if got := mock.FetchCount("fling-data/cache/fling_main.html"); got != 1 {
		t.Errorf("缓存命中后 main 抓取次数: got %d, want 1", got)
	}
}

// =============================================================================
// Scenario 4: Config roundtrip — JSON 序列化/反序列化往返
// =============================================================================

// TestE2E_ConfigRoundtrip 测试配置 JSON 序列化和反序列化的往返一致性。
//
// Given: 自定义配置值（CacheTTLHours=48, DownloadPath="/custom/", LastFetch=指定时间）
// When: JSON 序列化 → JSON 反序列化
// Then: 所有字段保留原值
//
// 本测试覆盖 Config 结构体的 JSON 往返正确性，等价于 store 包的 SaveConfig/LoadConfig
// 核心逻辑（SaveConfig/LoadConfig 是对 MarshalIndent/Unmarshal 的薄封装）。
// 由于 store 包导入 fling 包（使用 Config 类型），而 fling 包测试无法反向导入 store
// 包（会形成导入循环），故此处直接测试 JSON 往返。
func TestE2E_ConfigRoundtrip(t *testing.T) {
	lastFetch := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	original := &Config{
		CacheTTLHours: 48,
		DownloadPath:  "/custom/path/trainers/",
		LastFetch:     lastFetch,
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	var loaded Config
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("JSON 反序列化失败: %v", err)
	}

	if loaded.CacheTTLHours != original.CacheTTLHours {
		t.Errorf("CacheTTLHours: got %d, want %d", loaded.CacheTTLHours, original.CacheTTLHours)
	}
	if loaded.DownloadPath != original.DownloadPath {
		t.Errorf("DownloadPath: got %q, want %q", loaded.DownloadPath, original.DownloadPath)
	}
	if !loaded.LastFetch.Equal(original.LastFetch) {
		t.Errorf("LastFetch: got %v, want %v", loaded.LastFetch, original.LastFetch)
		if loaded.LastFetch.IsZero() {
			t.Log("提示: LastFetch 为零值，JSON 反序列化可能丢失了时间字段")
		}
	}
}

// =============================================================================
// Scenario 5: Symbol replacement
// =============================================================================

// TestE2E_SymbolReplacement 测试特殊字符到文件系统安全字符的转换。
//
// Given: 包含特殊字符的游戏名 "Game: Sub/Title?"
// When: 调用 SymbolReplacement
// Then: 返回 "Game - Sub_Title"（冒号+空格→破折号、斜杠→下划线、问号→空）
func TestE2E_SymbolReplacement(t *testing.T) {
	input := "Game: Sub/Title?"
	want := "Game - Sub_Title"

	got := SymbolReplacement(input)
	if got != want {
		t.Errorf("SymbolReplacement(%q) = %q, want %q", input, got, want)
	}
}

// =============================================================================
// Scenario 6: Sanitize
// =============================================================================

// TestE2E_Sanitize 测试字符串标准化处理（数字转罗马数字、去标点、转小写）。
//
// Given: 输入 "Test 7"
// When: 调用 Sanitize
// Then: 返回 "testvii"（7→vii、空格去掉、转小写）
func TestE2E_Sanitize(t *testing.T) {
	input := "Test 7"
	want := "testvii"

	got := Sanitize(input)
	if got != want {
		t.Errorf("Sanitize(%q) = %q, want %q", input, got, want)
	}
}
