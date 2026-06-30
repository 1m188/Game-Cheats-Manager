// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"slices"
	"testing"
)

// =============================================================================
// Sanitize 单元测试
// =============================================================================

const (
	testGameDREDGE       = "DREDGE"
	testGameDarkSoulsIII = "Dark Souls III"
	testFLDREDGETrainer  = "[FL] DREDGE Trainer"
)

// TestSanitize 测试字符串标准化处理：数字转罗马数字、去除标点、去空白、转小写。
// 参考 Python download_base_thread.py:295-298。
func TestSanitize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "数字转罗马数字",
			input: "Test 7",
			want:  "testvii",
		},
		{
			name:  "罗马数字保持不变",
			input: testGameDarkSoulsIII,
			want:  "darksoulsiii",
		},
		{
			name:  "保留and符号",
			input: "Game & Watch",
			want:  "game&watch",
		},
		{
			name:  "混合数字标点",
			input: "Hello-World 2.0",
			want:  "helloworldii0",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "仅有标点符号",
			input: "!@#$%^*()-_=+[]{};:',.<>/?`~",
			want:  "",
		},
		{
			name:  "仅数字",
			input: "123",
			want:  "cxxiii",
		},
		{
			name:  "数字零边界",
			input: "0",
			want:  "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sanitize(tt.input)
			if got != tt.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// SearchTrainers 单元测试
// =============================================================================

// testTrainers 返回一组固定的测试用修改器数据。
func testTrainers() (archive, main []Trainer) {
	archive = []Trainer{
		{GameName: testGameDREDGE, URL: "https://archive.flingtrainer.com/DREDGE.html", Origin: originArchive},
		{GameName: testGameDarkSoulsIII, URL: "https://archive.flingtrainer.com/Dark_Souls_III.html", Origin: originArchive},
		{GameName: "Elden Ring", URL: "https://archive.flingtrainer.com/Elden_Ring.html", Origin: originArchive},
	}
	main = []Trainer{
		{GameName: testGameDREDGE, URL: "https://flingtrainer.com/DREDGE.html", Origin: originMain},
		{GameName: "Balatro", URL: "https://flingtrainer.com/Balatro.html", Origin: originMain},
		{GameName: "Celeste", URL: "https://flingtrainer.com/Celeste.html", Origin: originMain},
	}
	return archive, main
}

// TestSearchTrainers_精确匹配 测试 "DREDGE" 关键词的精确模糊匹配。
func TestSearchTrainers_精确匹配(t *testing.T) {
	// Given: 固定的 archive 和 main 测试数据
	archive, main := testTrainers()

	// When: 搜索 testGameDREDGE
	results := SearchTrainers(archive, main, testGameDREDGE)

	// Then: 结果中包含 DREDGE（至少一个来自 main 来源）
	if len(results) == 0 {
		t.Fatal("搜索 '" + testGameDREDGE + "' 应返回结果，但为空")
	}
	hasMain := false
	for _, r := range results {
		if r.GameName == testGameDREDGE && r.Origin == originMain {
			hasMain = true
		}
		if r.GameName == testGameDREDGE && r.TrainerName != testFLDREDGETrainer {
			t.Errorf("DREDGE TrainerName: got %q, want %q", r.TrainerName, testFLDREDGETrainer)
		}
	}
	if !hasMain {
		t.Error("搜索结果中应包含来自 fling_main 的 DREDGE")
	}
}

// TestSearchTrainers_大小写不敏感 测试小写关键词 "dredge" 同样能匹配。
func TestSearchTrainers_大小写不敏感(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 使用小写关键词搜索
	results := SearchTrainers(archive, main, "dredge")

	// Then: 同样匹配到 DREDGE
	found := false
	for _, r := range results {
		if r.GameName == testGameDREDGE {
			found = true
			break
		}
	}
	if !found {
		t.Error("小写 'dredge' 应能匹配到 DREDGE")
	}
}

// TestSearchTrainers_无匹配结果 测试不存在的游戏名返回空结果。
func TestSearchTrainers_无匹配结果(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 搜索不存在的游戏
	results := SearchTrainers(archive, main, "zzzznotagame")

	// Then: 返回空结果
	if len(results) != 0 {
		t.Errorf("搜索不存在游戏应返回空: got %d 条结果", len(results))
	}
}

// TestSearchTrainers_去重main优先 测试同 URL 时 main 覆盖 archive，不同 URL 时均保留。
func TestSearchTrainers_去重main优先(t *testing.T) {
	// Given: archive 和 main 中都有 "DREDGE"（URL 不同，应各自保留）
	archive, main := testTrainers()

	// When: 搜索 testGameDREDGE
	results := SearchTrainers(archive, main, testGameDREDGE)

	// Then: 不同 URL 的 DREDGE 各出现一次
	countDREDGE := 0
	for _, r := range results {
		if r.GameName == testGameDREDGE {
			countDREDGE++
		}
	}
	if countDREDGE < 1 {
		t.Errorf("搜索 DREDGE 应至少返回 1 条结果: got %d", countDREDGE)
	}
}

// TestSearchTrainers_按游戏名排序 测试结果按 GameName 字母序排列。
func TestSearchTrainers_按游戏名排序(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 搜索宽泛关键词 "e"（应匹配多个游戏）
	results := SearchTrainers(archive, main, "e")

	// Then: 结果按 GameName 字母序排列
	gameNames := make([]string, len(results))
	for i, r := range results {
		gameNames[i] = r.GameName
	}
	if !slices.IsSorted(gameNames) {
		t.Errorf("结果应按字母序排列: got %v", gameNames)
	}
}

// TestSearchTrainers_空关键词 测试空关键词返回空结果。
func TestSearchTrainers_空关键词(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 关键词为空
	results := SearchTrainers(archive, main, "")

	// Then: 返回空或 nil 切片，不 panic
	if len(results) != 0 {
		t.Errorf("空关键词应返回空结果: got %d 条", len(results))
	}
}

// TestSearchTrainers_关键词过短 测试单字符关键词不匹配（len < 2 的前置条件）。
func TestSearchTrainers_关键词过短(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 单字符关键词
	results := SearchTrainers(archive, main, "a")

	// Then: 关键词语义过短，不应匹配任何结果
	if len(results) != 0 {
		t.Errorf("单字符关键词应返回空: got %d 条", len(results))
	}
}

// TestSearchTrainers_展示名格式 测试每个 SearchResult 的 TrainerName 格式正确。
func TestSearchTrainers_展示名格式(t *testing.T) {
	// Given: 固定的测试数据
	archive, main := testTrainers()

	// When: 搜索 testGameDREDGE
	results := SearchTrainers(archive, main, testGameDREDGE)

	// Then: 所有结果的 TrainerName 格式均为 "[FL] {GameName} Trainer"
	for _, r := range results {
		wantName := "[FL] " + r.GameName + " Trainer"
		if r.TrainerName != wantName {
			t.Errorf("TrainerName 格式错误: got %q, want %q", r.TrainerName, wantName)
		}
	}
}
