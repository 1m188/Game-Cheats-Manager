// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"encoding/json"
	"testing"
	"time"
)

// TestConfigJSON往返测试 — 验证 Config 结构体的 JSON 序列化与反序列化正确。
func TestConfigJSON往返测试(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "默认值配置",
			config: Config{
				CacheTTLHours: 24,
				DownloadPath:  "./fling-data/trainers/",
				LastFetch:     time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "自定义配置",
			config: Config{
				CacheTTLHours: 48,
				DownloadPath:  "/custom/path/",
				LastFetch:     time.Time{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Given: 一个 Config 结构体
			// When: 序列化为 JSON 再反序列化回来
			data, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("json.Marshal 失败: %v", err)
			}

			var got Config
			err = json.Unmarshal(data, &got)
			if err != nil {
				t.Fatalf("json.Unmarshal 失败: %v", err)
			}

			// Then: 字段值应保持一致
			if got.CacheTTLHours != tt.config.CacheTTLHours {
				t.Errorf("CacheTTLHours: got %d, want %d", got.CacheTTLHours, tt.config.CacheTTLHours)
			}
			if got.DownloadPath != tt.config.DownloadPath {
				t.Errorf("DownloadPath: got %s, want %s", got.DownloadPath, tt.config.DownloadPath)
			}
			if !got.LastFetch.Equal(tt.config.LastFetch) {
				t.Errorf("LastFetch: got %v, want %v", got.LastFetch, tt.config.LastFetch)
			}
		})
	}
}

// TestTrainerJSON标签测试 — 验证 Trainer 结构体的 JSON tag 输出正确。
func TestTrainerJSON标签测试(t *testing.T) {
	t.Parallel()

	trainer := Trainer{
		GameName:    testGameDredge,
		URL:         "https://flingtrainer.com/trainers/dredge/",
		Origin:      "fling_main",
		Version:     "2024.03.15",
		TrainerName: "[FL] DREDGE Trainer",
	}

	// When: 序列化为 JSON
	data, err := json.Marshal(trainer)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// Then: JSON 键名应使用蛇形命名（json tag）
	var raw map[string]any
	err = json.Unmarshal(data, &raw)
	if err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}

	expectedKeys := []string{"game_name", "url", "origin", "version", "trainer_name"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("JSON 输出中缺少键: %s", key)
		}
	}

	// 额外验证: 不应出现 Go 字段名形式的键
	if _, ok := raw["GameName"]; ok {
		t.Error("JSON 输出中不应出现 GameName 键")
	}
}

// TestDownloadProgressJSON测试 — 验证 DownloadProgress 结构体正确序列化。
func TestDownloadProgressJSON测试(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		progress DownloadProgress
		wantJSON string
	}{
		{
			name: "下载进行中",
			progress: DownloadProgress{
				BytesDownloaded: 500_000,
				TotalBytes:      1_000_000,
				PercentComplete: 50.0,
			},
			wantJSON: `{"bytes_downloaded":500000,"total_bytes":1000000,"percent_complete":50}`,
		},
		{
			name: "下载完成",
			progress: DownloadProgress{
				BytesDownloaded: 1_000_000,
				TotalBytes:      1_000_000,
				PercentComplete: 100.0,
			},
			wantJSON: `{"bytes_downloaded":1000000,"total_bytes":1000000,"percent_complete":100}`,
		},
		{
			name: "下载开始",
			progress: DownloadProgress{
				BytesDownloaded: 0,
				TotalBytes:      1_000_000,
				PercentComplete: 0.0,
			},
			wantJSON: `{"bytes_downloaded":0,"total_bytes":1000000,"percent_complete":0}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// When: 序列化为 JSON
			data, err := json.Marshal(tt.progress)
			if err != nil {
				t.Fatalf("json.Marshal 失败: %v", err)
			}

			// Then: JSON 字符串应匹配预期
			if string(data) != tt.wantJSON {
				t.Errorf("JSON = %s, want %s", string(data), tt.wantJSON)
			}
		})
	}
}

// TestSearchResult别名测试 — 验证 SearchResult 是 Trainer 的类型别名，可直接互转。
func TestSearchResult别名测试(t *testing.T) {
	t.Parallel()

	// Given: SearchResult 是 Trainer 的别名，可以直接使用相同的字段集
	sr := SearchResult{
		GameName:    "Hades",
		URL:         "https://flingtrainer.com/trainers/hades/",
		Origin:      "fling_archive",
		Version:     "2023.05.20",
		TrainerName: "[FL] Hades Trainer",
	}

	// When: 序列化为 JSON
	data, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// Then: 输出应与 Trainer 的 JSON 格式相同，可反序列化回 Trainer
	var got Trainer
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}

	if got.GameName != sr.GameName {
		t.Errorf("GameName: got %s, want %s", got.GameName, sr.GameName)
	}
	if got.Origin != sr.Origin {
		t.Errorf("Origin: got %s, want %s", got.Origin, sr.Origin)
	}
}

// TestIndexJSON往返测试 — 验证 Index 结构体的 JSON 序列化与反序列化正确。
func TestIndexJSON往返测试(t *testing.T) {
	t.Parallel()

	fetchedAt := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	index := Index{
		Archive: []Trainer{
			{
				GameName:    testGameDredge,
				URL:         "https://flingtrainer.com/trainers/dredge/",
				Origin:      "fling_archive",
				Version:     "2024.03.15",
				TrainerName: "[FL] DREDGE Trainer",
			},
		},
		Main: []Trainer{
			{
				GameName:    "Hades",
				URL:         "https://flingtrainer.com/trainers/hades/",
				Origin:      "fling_main",
				Version:     "2023.05.20",
				TrainerName: "[FL] Hades Trainer",
			},
		},
		FetchedAt: fetchedAt,
	}

	// When: 序列化为 JSON 再反序列化
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	var got Index
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}

	// Then: 数组长度一致
	if len(got.Archive) != len(index.Archive) {
		t.Errorf("Archive 长度: got %d, want %d", len(got.Archive), len(index.Archive))
	}
	if len(got.Main) != len(index.Main) {
		t.Errorf("Main 长度: got %d, want %d", len(got.Main), len(index.Main))
	}
	// Then: 时间戳一致
	if !got.FetchedAt.Equal(fetchedAt) {
		t.Errorf("FetchedAt: got %v, want %v", got.FetchedAt, fetchedAt)
	}
}

// TestIndex空索引测试 — 验证空 Index 可正常序列化。
func TestIndex空索引测试(t *testing.T) {
	t.Parallel()

	index := Index{
		Archive: nil,
		Main:    nil,
	}

	// When: 序列化为 JSON
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("json.Marshal 失败: %v", err)
	}

	// Then: 空切片应序列化为 null（Go 默认行为）
	var got Index
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("json.Unmarshal 失败: %v", err)
	}

	if got.Archive != nil {
		t.Errorf("Archive: got %v, want nil", got.Archive)
	}
	if got.Main != nil {
		t.Errorf("Main: got %v, want nil", got.Main)
	}
}
