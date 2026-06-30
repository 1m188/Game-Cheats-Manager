// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"os"
	"strings"
	"testing"
)

// TestParseArchive_完整解析测试 — 使用 testdata/fling_archive.html 作为真实抓取的 fixture 进行解析。
func TestParseArchive_完整解析测试(t *testing.T) {
	// Given: archive.flingtrainer.com 页面的 HTML fixture
	html, err := os.ReadFile("../../testdata/fling_archive.html")
	if err != nil {
		t.Fatalf("读取 testdata/fling_archive.html 失败: %v", err)
	}

	// When: 解析 HTML
	trainers, err := ParseArchiveHTML(html)

	// Then: 无错误，返回非空列表
	if err != nil {
		t.Fatalf("ParseArchiveHTML 返回错误: %v", err)
	}
	if len(trainers) == 0 {
		t.Fatal("解析结果不应为空")
	}

	// Then: DREDGE 出现在结果中（含版本号），URL 正确
	foundDredge := false
	for _, tr := range trainers {
		if tr.GameName != "DREDGE v1.2.0" {
			continue
		}
		foundDredge = true
		if tr.URL != "https://archive.flingtrainer.com/DREDGE_v1.2.0.html" {
			t.Errorf("DREDGE URL: got %s, want https://archive.flingtrainer.com/DREDGE_v1.2.0.html", tr.URL)
		}
		if tr.Origin != originArchive {
			t.Errorf("DREDGE Origin: got %s, want fling_archive", tr.Origin)
		}
		if tr.Version != "" {
			t.Errorf("DREDGE Version: got %s, want empty string", tr.Version)
		}
		if tr.TrainerName != "" {
			t.Errorf("DREDGE TrainerName: got %s, want empty string", tr.TrainerName)
		}
		break
	}
	if !foundDredge {
		t.Error("结果中应包含 DREDGE")
	}
}

// TestParseArchive_忽略列表测试 — 验证硬编码的 4 个忽略游戏不出现在结果中。
func TestParseArchive_忽略列表测试(t *testing.T) {
	// Given: 含忽略列表游戏的 HTML fixture
	html, err := os.ReadFile("../../testdata/fling_archive.html")
	if err != nil {
		t.Fatalf("读取 testdata 失败: %v", err)
	}

	// When: 解析 HTML
	trainers, err := ParseArchiveHTML(html)
	if err != nil {
		t.Fatalf("ParseArchiveHTML 返回错误: %v", err)
	}

	// Then: 忽略列表中的游戏不应出现
	ignoredNames := []string{
		"Dying Light The Following Enhanced Edition",
		"Monster Hunter World",
		"Street Fighter V",
		"World War Z",
	}
	for _, name := range ignoredNames {
		for _, tr := range trainers {
			if tr.GameName == name {
				t.Errorf("被忽略的游戏 %q 不应出现在结果中", name)
			}
		}
	}
}

// TestParseArchive_BrightMemory特殊处理测试 — 验证 "Bright.Memory.Episode.1" 特殊转换为 "Bright Memory: Episode 1"。
func TestParseArchive_BrightMemory特殊处理测试(t *testing.T) {
	// Given: 含 Bright.Memory.Episode.1 的 HTML fixture
	html, err := os.ReadFile("../../testdata/fling_archive.html")
	if err != nil {
		t.Fatalf("读取 testdata 失败: %v", err)
	}

	// When: 解析 HTML
	trainers, err := ParseArchiveHTML(html)
	if err != nil {
		t.Fatalf("ParseArchiveHTML 返回错误: %v", err)
	}

	// Then: "Bright Memory: Episode 1" 出现在结果中
	wantName := "Bright Memory: Episode 1"
	found := false
	for _, tr := range trainers {
		if tr.GameName == wantName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("结果中应包含 %q", wantName)
	}

	// Then: 不应包含原始的带点号名称
	for _, tr := range trainers {
		if tr.GameName == "Bright.Memory.Episode.1" {
			t.Error("结果中不应包含未处理的 Bright.Memory.Episode.1")
		}
	}
}

// TestParseArchive_版本号保留测试 — 验证版本信息保留在 GameName 中，下划线被替换。
func TestParseArchive_版本号去除测试(t *testing.T) {
	// Given: 含各种版本后缀的 HTML fixture
	html, err := os.ReadFile("../../testdata/fling_archive.html")
	if err != nil {
		t.Fatalf("读取 testdata 失败: %v", err)
	}

	// When: 解析 HTML
	trainers, err := ParseArchiveHTML(html)
	if err != nil {
		t.Fatalf("ParseArchiveHTML 返回错误: %v", err)
	}

	// Then: 版本号保留在 GameName 中，下划线被替换为冒号空格
	checks := map[string]bool{
		"Elden: Ring v1.10":                 true,
		"Balatro 1.0.3":                     true,
		"Hades: II Early Access v0.9":       true,
		"Sekiro: Shadows: Die: Twice":       true,
	}

	for _, tr := range trainers {
		delete(checks, tr.GameName)
	}

	if len(checks) > 0 {
		remaining := make([]string, 0, len(checks))
		for name := range checks {
			remaining = append(remaining, name)
		}
		t.Errorf("以下预期游戏名未找到: %v", remaining)
	}
}

// TestParseArchive_URL格式测试 — 验证所有 URL 都以 "https://archive.flingtrainer.com/" 开头。
func TestParseArchive_URL格式测试(t *testing.T) {
	// Given: archive 页面的 HTML fixture
	html, err := os.ReadFile("../../testdata/fling_archive.html")
	if err != nil {
		t.Fatalf("读取 testdata 失败: %v", err)
	}

	// When: 解析 HTML
	trainers, err := ParseArchiveHTML(html)
	if err != nil {
		t.Fatalf("ParseArchiveHTML 返回错误: %v", err)
	}

	// Then: 每个 Trainer 的 URL 以期望前缀开头
	prefix := "https://archive.flingtrainer.com/"
	for _, tr := range trainers {
		if !strings.HasPrefix(tr.URL, prefix) {
			t.Errorf("Trainer %q 的 URL %q 不以 %q 开头", tr.GameName, tr.URL, prefix)
		}
	}
}

// TestParseArchive_空HTML测试 — 验证空 HTML 返回空切片，无错误。
func TestParseArchive_空HTML测试(t *testing.T) {
	// Given: 空 HTML
	// When: 解析空 HTML
	trainers, err := ParseArchiveHTML([]byte(""))

	// Then: 无错误，返回空切片
	if err != nil {
		t.Fatalf("空 HTML 不应返回错误: %v", err)
	}
	if trainers == nil || len(trainers) != 0 {
		t.Errorf("空 HTML 应返回空切片而非 nil: got len=%d", len(trainers))
	}
}

// TestParseArchive_无匹配选择器的HTML测试 — 验证不含 target="_self" 链接的 HTML 返回空切片。
func TestParseArchive_无匹配选择器的HTML测试(t *testing.T) {
	// Given: 不包含 target="_self" 的 HTML
	html := []byte("<html><body><a href='/foo'>No target</a></body></html>")

	// When: 解析
	trainers, err := ParseArchiveHTML(html)

	// Then: 无错误，空结果
	if err != nil {
		t.Fatalf("不应返回错误: %v", err)
	}
	if len(trainers) != 0 {
		t.Errorf("应返回空切片: got %d 条结果", len(trainers))
	}
}
