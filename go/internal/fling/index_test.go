// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// mockFetcher — CacheFetcher 接口的测试用实现
// =============================================================================

// mockFetcher 是 CacheFetcher 的测试替身，将 FetchAndCache 转发到
// httptest.NewServer 的 mock HTTP 服务器，并在内存中维护缓存状态。
type mockFetcher struct {
	serverURL  string
	fetchCount map[string]int // keyed by cachePath
	cache      map[string][]byte
	modTimes   map[string]time.Time
}

const (
	// archiveHTMLFixture 是可供 ParseArchiveHTML 成功解析的最小 HTML。
	// 使用 a[target="_self"] 选择器匹配。
	archiveHTMLFixture = `<html><body><a href="test-game.html" target="_self">TestGame</a></body></html>`

	// mainHTMLFixture 是可供 ParseMainHTML 成功解析的最小 HTML。
	// 使用 div.letter-section ul li a 选择器匹配，带 " Trainer" 后缀。
	mainHTMLFixture = `<html><body><div class="letter-section"><ul><li><a href="https://flingtrainer.com/test-game.html">TestGame Trainer</a></li></ul></div></body></html>`
)

// newMockFetcher 创建一个新的 mock fetcher 实例。
// serverURL 是 httptest.NewServer 返回的基础 URL。
func newMockFetcher(serverURL string) *mockFetcher {
	return &mockFetcher{
		serverURL:  serverURL,
		fetchCount: make(map[string]int),
		cache:      make(map[string][]byte),
		modTimes:   make(map[string]time.Time),
	}
}

// FetchAndCache 从 mock HTTP 服务器获取内容并存入内存缓存。
// url 参数被忽略，实际请求使用 serverURL 加上根据 cachePath 推导的路径。
func (m *mockFetcher) FetchAndCache(_, cachePath string) ([]byte, error) {
	m.fetchCount[cachePath]++

	var path string
	switch {
	case strings.Contains(cachePath, "fling_archive"):
		path = mockPathArchive
	case strings.Contains(cachePath, "fling_main"):
		path = mockPathMain
	default:
		return nil, fmt.Errorf("未知缓存路径: %s", cachePath)
	}

	ctx := context.TODO()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.serverURL+path, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("mock 创建 HTTP 请求失败: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mock HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // 测试 mock 中 Close 错误忽略是标准实践

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mock 读取响应体失败: %w", err)
	}

	m.cache[cachePath] = data
	m.modTimes[cachePath] = time.Now()
	return data, nil
}

// LoadFromCache 从内存缓存中加载数据，并在超过 maxAge 时返回错误。
func (m *mockFetcher) LoadFromCache(cachePath string, maxAge time.Duration) ([]byte, error) {
	data, ok := m.cache[cachePath]
	if !ok {
		return nil, errors.New("缓存未命中")
	}
	if maxAge > 0 {
		age := time.Since(m.modTimes[cachePath])
		if age > maxAge {
			return nil, errors.New("缓存已过期")
		}
	}
	return data, nil
}

// FetchCount 返回指定缓存路径的抓取次数。
func (m *mockFetcher) FetchCount(cachePath string) int {
	return m.fetchCount[cachePath]
}

// ageCache 将内存中所有缓存条目的修改时间前移 d 时长，模拟时间流逝。
func (m *mockFetcher) ageCache(d time.Duration) {
	for k := range m.modTimes {
		m.modTimes[k] = m.modTimes[k].Add(-d)
	}
}

// =============================================================================
// mock HTTP 服务器 — 提供可供解析器识别的 HTML 内容
// =============================================================================

// newMockFLiNGServer 创建一个 httptest.Server，在 /archive 和 /main 路径
// 分别返回可供 ParseArchiveHTML 和 ParseMainHTML 解析的最小 HTML。
func newMockFLiNGServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case mockPathArchive:
			_, err := w.Write([]byte(archiveHTMLFixture))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case mockPathMain:
			_, err := w.Write([]byte(mainHTMLFixture))
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
// BuildIndex 测试
// =============================================================================

// TestBuildIndex_非空返回 测试首次 BuildIndex 调用返回非空的 Archive 和 Main 索引。
func TestBuildIndex_非空返回(t *testing.T) {
	// Given: 一个 mock HTTP 服务器和新的 mock fetcher
	server := newMockFLiNGServer()
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	// When: 首次调用 BuildIndex
	idx, err := BuildIndex(mock, cfg)

	// Then: 未出错，且 Archive 和 Main 均非空
	if err != nil {
		t.Fatalf("BuildIndex 失败: %v", err)
	}
	if idx == nil {
		t.Fatal("BuildIndex 返回 nil Index")
	}
	if len(idx.Archive) == 0 {
		t.Error("Archive 不应为空，但得到空切片")
	}
	if len(idx.Main) == 0 {
		t.Error("Main 不应为空，但得到空切片")
	}
	// Archive 应有 1 条训练器，来自 archiveHTMLFixture
	if len(idx.Archive) != 1 {
		t.Errorf("Archive 长度: got %d, want 1", len(idx.Archive))
	}
	// Main 应有 1 条训练器，来自 mainHTMLFixture
	if len(idx.Main) != 1 {
		t.Errorf("Main 长度: got %d, want 1", len(idx.Main))
	}
}

// TestBuildIndex_时间戳设置 测试 BuildIndex 返回的 FetchedAt 时间戳已设置。
func TestBuildIndex_时间戳设置(t *testing.T) {
	// Given: mock HTTP 服务器和 fetcher
	server := newMockFLiNGServer()
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	// When: 调用 BuildIndex
	before := time.Now()
	idx, err := BuildIndex(mock, cfg)
	after := time.Now()

	// Then: FetchedAt 设置在调用时间 ± 1 秒内
	if err != nil {
		t.Fatalf("BuildIndex 失败: %v", err)
	}
	if idx.FetchedAt.IsZero() {
		t.Error("FetchedAt 不应为零值")
	}
	if idx.FetchedAt.Before(before.Add(-time.Second)) || idx.FetchedAt.After(after.Add(time.Second)) {
		t.Errorf("FetchedAt %v 不在预期时间范围 [%v, %v] 内", idx.FetchedAt, before, after)
	}
}

// TestBuildIndex_缓存命中 测试第二次调用 BuildIndex 且缓存未过期时，
// 不应再发起 HTTP 请求（fetchCount 保持 1）。
func TestBuildIndex_缓存命中(t *testing.T) {
	// Given: mock HTTP 服务器、fetcher 和 24 小时 TTL
	server := newMockFLiNGServer()
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	// When: 第一次 BuildIndex
	_, err := BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("首次 BuildIndex 失败: %v", err)
	}

	// Then: 应已抓取一次
	archiveCount1 := mock.FetchCount("fling-data/cache/fling_archive.html")
	mainCount1 := mock.FetchCount("fling-data/cache/fling_main.html")
	if archiveCount1 != 1 {
		t.Errorf("首次后 archive 抓取次数: got %d, want 1", archiveCount1)
	}
	if mainCount1 != 1 {
		t.Errorf("首次后 main 抓取次数: got %d, want 1", mainCount1)
	}

	// When: 第二次 BuildIndex（缓存未过期）
	_, err = BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("第二次 BuildIndex 失败: %v", err)
	}

	// Then: 抓取次数未增加（命中缓存）
	if got := mock.FetchCount("fling-data/cache/fling_archive.html"); got != 1 {
		t.Errorf("缓存命中后 archive 抓取次数: got %d, want 1", got)
	}
	if got := mock.FetchCount("fling-data/cache/fling_main.html"); got != 1 {
		t.Errorf("缓存命中后 main 抓取次数: got %d, want 1", got)
	}
}

// TestBuildIndex_过期重新抓取 测试缓存 TTL 过期后，BuildIndex 重新发起 HTTP 请求。
func TestBuildIndex_过期重新抓取(t *testing.T) {
	// Given: mock HTTP 服务器、fetcher 和 1 小时 TTL
	server := newMockFLiNGServer()
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 1}

	// When: 第一次 BuildIndex
	_, err := BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("首次 BuildIndex 失败: %v", err)
	}

	// 模拟缓存过期：将缓存修改时间前移 2 小时
	mock.ageCache(2 * time.Hour)

	// When: 第二次 BuildIndex（缓存已过期）
	_, err = BuildIndex(mock, cfg)
	if err != nil {
		t.Fatalf("第二次 BuildIndex 失败: %v", err)
	}

	// Then: 抓取次数增加（重新抓取）
	if got := mock.FetchCount("fling-data/cache/fling_archive.html"); got != 2 {
		t.Errorf("过期后 archive 抓取次数: got %d, want 2", got)
	}
	if got := mock.FetchCount("fling-data/cache/fling_main.html"); got != 2 {
		t.Errorf("过期后 main 抓取次数: got %d, want 2", got)
	}
}

// TestBuildIndex_解析结果正确 测试 BuildIndex 返回的 Trainer 字段与 HTML fixture 一致。
func TestBuildIndex_解析结果正确(t *testing.T) {
	// Given: mock HTTP 服务器和 fetcher
	server := newMockFLiNGServer()
	defer server.Close()
	mock := newMockFetcher(server.URL)
	cfg := &Config{CacheTTLHours: 24}

	// When: 调用 BuildIndex
	idx, err := BuildIndex(mock, cfg)

	// Then: 解析出的 Trainer 字段正确
	if err != nil {
		t.Fatalf("BuildIndex 失败: %v", err)
	}

	// Archive 检查
	if len(idx.Archive) == 0 {
		t.Fatal("Archive 为空，无法检查字段")
	}
	arch := idx.Archive[0]
	if arch.GameName != "TestGame" {
		t.Errorf("Archive GameName: got %q, want %q", arch.GameName, "TestGame")
	}
	if arch.Origin != originArchive {
		t.Errorf("Archive Origin: got %q, want %q", arch.Origin, originArchive)
	}

	// Main 检查
	if len(idx.Main) == 0 {
		t.Fatal("Main 为空，无法检查字段")
	}
	main := idx.Main[0]
	if main.GameName != "TestGame" {
		t.Errorf("Main GameName: got %q, want %q", main.GameName, "TestGame")
	}
	if main.Origin != originMain {
		t.Errorf("Main Origin: got %q, want %q", main.Origin, originMain)
	}
}
