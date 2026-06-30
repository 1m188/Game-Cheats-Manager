// Package store 提供 HTTP 请求、磁盘缓存和配置持久化功能。
package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fling-tui/internal/fling"
)

func TestConfigDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	cfg, err := readConfigFile(configPath)
	if err != nil {
		t.Fatalf("readConfigFile 返回错误: %v", err)
	}
	if cfg == nil {
		t.Fatal("readConfigFile 返回 nil 配置")
	}
	if cfg.CacheTTLHours != 24 {
		t.Errorf("默认 CacheTTLHours = %d, want 24", cfg.CacheTTLHours)
	}
	if cfg.DownloadPath != "./fling-data/trainers/" {
		t.Errorf("默认 DownloadPath = %q, want %q", cfg.DownloadPath, "./fling-data/trainers/")
	}
	if !cfg.LastFetch.IsZero() {
		t.Errorf("默认 LastFetch 应为零值, got %v", cfg.LastFetch)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	lastFetch := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	original := &fling.Config{
		CacheTTLHours: 48,
		DownloadPath:  "/custom/path/trainers/",
		LastFetch:     lastFetch,
	}

	err := writeConfigFile(configPath, original)
	if err != nil {
		t.Fatalf("writeConfigFile 返回错误: %v", err)
	}

	loaded, err := readConfigFile(configPath)
	if err != nil {
		t.Fatalf("readConfigFile 返回错误: %v", err)
	}
	if loaded == nil {
		t.Fatal("readConfigFile 返回 nil 配置")
	}
	if loaded.CacheTTLHours != original.CacheTTLHours {
		t.Errorf("CacheTTLHours = %d, want %d", loaded.CacheTTLHours, original.CacheTTLHours)
	}
	if loaded.DownloadPath != original.DownloadPath {
		t.Errorf("DownloadPath = %q, want %q", loaded.DownloadPath, original.DownloadPath)
	}
	if !loaded.LastFetch.Equal(original.LastFetch) {
		t.Errorf("LastFetch = %v, want %v", loaded.LastFetch, original.LastFetch)
	}
}

func TestConfigInvalidJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	invalidJSON := []byte("{invalid json content [[[")
	err := os.WriteFile(configPath, invalidJSON, 0o600)
	if err != nil {
		t.Fatalf("写入无效 JSON 失败: %v", err)
	}

	cfg, err := readConfigFile(configPath)
	if err == nil {
		t.Error("预期 readConfigFile 对无效 JSON 返回错误, 但没有")
	}
	if cfg != nil {
		t.Errorf("预期 readConfigFile 对无效 JSON 返回 nil 配置, got %+v", cfg)
	}
}

func TestConfigEmptyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	emptyFile := []byte("")
	err := os.WriteFile(configPath, emptyFile, 0o600)
	if err != nil {
		t.Fatalf("写入空文件失败: %v", err)
	}

	cfg, err := readConfigFile(configPath)
	if err == nil {
		t.Error("预期 readConfigFile 对空文件返回错误, 但没有")
	}
	if cfg != nil {
		t.Errorf("预期 readConfigFile 对空文件返回 nil 配置, got %+v", cfg)
	}
}
