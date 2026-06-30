// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"os"
	"strings"
	"testing"
)

// 测试夹具中用于验证的游戏名常量（避免 goconst 跨文件重复字面量告警）。
const (
	testGameDredge = "DREDGE"
	testGameHades  = "Hades"
	testURLDredge  = "https://flingtrainer.com/trainers/dredge/"
	testURLHades   = "https://flingtrainer.com/trainers/hades/"
)

// loadMainFixture 加载 fling_main.html 测试夹具。
func loadMainFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/fling_main.html")
	if err != nil {
		t.Fatalf("无法读取测试数据: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("测试数据为空")
	}
	return data
}

// assertTrainerField 遍历所有 Trainer，对每个 Trainer 执行断言回调。
func assertTrainerField(t *testing.T, trainers []Trainer, label string, check func(Trainer) bool, msg func(int, Trainer) string) {
	t.Helper()
	for i, tr := range trainers {
		if !check(tr) {
			t.Errorf("Trainer[%d] %s: %s", i, label, msg(i, tr))
		}
	}
}

// TestParseMainHTML正常HTML测试 — 验证 ParseMainHTML 从 flingtrainer.com/all-trainers-a-z/ 页面中正确解析 Trainer。
func TestParseMainHTML正常HTML测试(t *testing.T) {
	t.Parallel()

	// Given: flingtrainer.com/all-trainers-a-z/ 页面的 HTML fixture
	html := loadMainFixture(t)

	// When: 解析主站 HTML
	got, err := ParseMainHTML(html)
	if err != nil {
		t.Fatalf("ParseMainHTML 返回错误: %v", err)
	}

	// Then: 返回非空 Trainer 列表
	if len(got) == 0 {
		t.Fatal("ParseMainHTML 返回空列表")
	}

	t.Run("Trainer列表非空", func(t *testing.T) {
		t.Parallel()
		if len(got) == 0 {
			t.Error("应返回至少一个 Trainer")
		}
	})

	t.Run("所有Trainer的Origin为fling_main", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "Origin", func(tr Trainer) bool {
			return tr.Origin == originMain
		}, func(_ int, tr Trainer) string {
			return "want " + originMain + ", got " + tr.Origin
		})
	})

	t.Run("GameName不包含Trainer后缀", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "GameName后缀", func(tr Trainer) bool {
			return !strings.Contains(tr.GameName, " Trainer")
		}, func(_ int, tr Trainer) string {
			return `不应包含 " Trainer" 后缀: ` + tr.GameName
		})
	})

	t.Run("URL以https://flingtrainer.com/开头", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "URL", func(tr Trainer) bool {
			return strings.HasPrefix(tr.URL, "https://flingtrainer.com/")
		}, func(_ int, tr Trainer) string {
			return "应以 https://flingtrainer.com/ 开头: " + tr.URL
		})
	})

	t.Run("GameName非空", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "GameName", func(tr Trainer) bool {
			return tr.GameName != ""
		}, func(_ int, _ Trainer) string {
			return "GameName 不应为空"
		})
	})

	t.Run("Version为空字符串", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "Version", func(tr Trainer) bool {
			return tr.Version == ""
		}, func(_ int, tr Trainer) string {
			return `Version = "` + tr.Version + `", want ""`
		})
	})

	t.Run("TrainerName为空字符串", func(t *testing.T) {
		t.Parallel()
		assertTrainerField(t, got, "TrainerName", func(tr Trainer) bool {
			return tr.TrainerName == ""
		}, func(_ int, tr Trainer) string {
			return `TrainerName = "` + tr.TrainerName + `", want ""`
		})
	})
}

// TestParseMainHTML空HTML测试 — 验证空 HTML 输入返回空切片且无错误。
func TestParseMainHTML空HTML测试(t *testing.T) {
	t.Parallel()

	got, err := ParseMainHTML([]byte{})
	if err != nil {
		t.Fatalf("ParseMainHTML 不应返回错误, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("空 HTML 应返回空切片, got %d trainers", len(got))
	}
}

// TestParseMainHTML无匹配链接测试 — 验证没有 letter-section 的 HTML 返回空切片。
func TestParseMainHTML无匹配链接测试(t *testing.T) {
	t.Parallel()

	html := []byte(`<html><body><a href="https://flingtrainer.com/">Home</a></body></html>`)
	got, err := ParseMainHTML(html)
	if err != nil {
		t.Fatalf("ParseMainHTML 返回错误: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("无 letter-section 时应返回空切片, got %d trainers", len(got))
	}
}

// TestParseMainHTML无效HTML测试 — 验证完全无效的 HTML 不 panic 且不报错。
func TestParseMainHTML无效HTML测试(t *testing.T) {
	t.Parallel()

	html := []byte(`not even html`)
	got, err := ParseMainHTML(html)
	if err != nil {
		t.Fatalf("ParseMainHTML 对无效 HTML 不应返回错误: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("无效 HTML 应返回空切片, got %d trainers", len(got))
	}
}

// TestParseMainHTML精确游戏名测试 — 验证 fixture 中每个游戏的名称剥离正确。
func TestParseMainHTML精确游戏名测试(t *testing.T) {
	t.Parallel()

	type wantTrainer struct {
		gameName string
		url      string
	}

	tests := []struct {
		name string
		want wantTrainer
	}{
		{
			name: "DREDGE名称不含Trainer后缀",
			want: wantTrainer{gameName: testGameDredge, url: testURLDredge},
		},
		{
			name: "Dead Cells名称不含Trainer后缀",
			want: wantTrainer{gameName: "Dead Cells", url: "https://flingtrainer.com/trainers/dead-cells/"},
		},
		{
			name: "Dark Souls III名称不含Trainer后缀",
			want: wantTrainer{gameName: "Dark Souls III", url: "https://flingtrainer.com/trainers/dark-souls-iii/"},
		},
		{
			name: "Hades名称不含Trainer后缀",
			want: wantTrainer{gameName: testGameHades, url: testURLHades},
		},
		{
			name: "Sekiro冒号标题正确处理",
			want: wantTrainer{gameName: "Sekiro: Shadows Die Twice", url: "https://flingtrainer.com/trainers/sekiro-shadows-die-twice/"},
		},
	}

	html := loadMainFixture(t)
	got, err := ParseMainHTML(html)
	if err != nil {
		t.Fatalf("ParseMainHTML 返回错误: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			found := false
			for _, tr := range got {
				if tr.GameName == tt.want.gameName && tr.URL == tt.want.url {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("未找到 Trainer{GameName: %q, URL: %q}", tt.want.gameName, tt.want.url)
			}
		})
	}
}
