// Package store_test 提供 store 包的 HTTP 缓存功能黑盒测试。
// 使用 httptest.NewServer 模拟 HTTP 服务端，不依赖外部网络。
//
//nolint:errcheck,gosec // 测试文件中，mock writer 写入和临时文件权限忽略错误是标准实践
package store_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fling-tui/internal/store"
)

// TestFetchAndCache 测试 HTTP 抓取并保存到磁盘缓存。
func TestFetchAndCache(t *testing.T) {
	tests := []struct {
		name     string
		handler  func(w http.ResponseWriter, _ *http.Request)
		wantBody string
		wantErr  bool
	}{
		{
			name: "成功抓取并保存到磁盘",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// 验证 Chrome 96 User-Agent
				ua := r.Header.Get("User-Agent")
				if ua == "" {
					t.Error("请求缺少 User-Agent 头")
				}
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("<html><body>test page</body></html>"))
			},
			wantBody: "<html><body>test page</body></html>",
			wantErr:  false,
		},
		{
			name: "HTTP非200状态码返回错误",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantBody: "",
			wantErr:  true,
		},
		{
			name: "HTTP服务器错误返回错误",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("server error"))
			},
			wantBody: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: 启动模拟 HTTP 服务器
			srv := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer srv.Close()

			tmpDir := t.TempDir()
			cachePath := filepath.Join(tmpDir, "test_cache.html")

			// When: 调用 FetchAndCache
			got, err := store.FetchAndCache(srv.URL, cachePath)

			// Then: 验证结果
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误，实际为 nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("未预期的错误: %v", err)
			}
			if string(got) != tt.wantBody {
				t.Errorf("响应体 = %q, 期望 %q", got, tt.wantBody)
			}

			// 验证缓存文件已写入磁盘且内容正确
			saved, err := os.ReadFile(filepath.Clean(cachePath))
			if err != nil {
				t.Fatalf("缓存文件未写入: %v", err)
			}
			if string(saved) != tt.wantBody {
				t.Errorf("缓存内容 = %q, 期望 %q", saved, tt.wantBody)
			}
		})
	}
}

// TestFetchAndCache_网络不可达 测试服务器不可用时返回错误。
func TestFetchAndCache_网络不可达(t *testing.T) {
	// Given: 启动服务器后立即关闭，模拟网络不可达
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	// 关闭服务器使后续请求失败
	srv.Close()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "unreachable.html")

	// When
	_, err := store.FetchAndCache(srv.URL, cachePath)

	// Then
	if err == nil {
		t.Fatal("期望网络错误，实际为 nil")
	}
}

// TestLoadFromCache 测试从磁盘缓存加载 HTML 内容（含 TTL 过期检查）。
func TestLoadFromCache(t *testing.T) { //nolint:gocognit,funlen // 表驱动测试含 setup 闭包，复杂度合理
	tests := []struct {
		name       string
		setup      func(dir string) (cachePath string, maxAge time.Duration)
		want       string
		wantErr    bool
		checkStale bool // true: 必须满足 errors.Is(err, ErrCacheStale)
	}{
		{
			name: "新鲜缓存返回内容",
			setup: func(dir string) (string, time.Duration) {
				path := filepath.Join(dir, "fresh.html")
				err := os.WriteFile(path, []byte("<html>cached content</html>"), 0o644)
				if err != nil {
					t.Fatalf("写入测试文件失败: %v", err)
				}
				return path, 1 * time.Hour
			},
			want:       "<html>cached content</html>",
			wantErr:    false,
			checkStale: false,
		},
		{
			name: "过期缓存返回ErrCacheStale",
			setup: func(dir string) (string, time.Duration) {
				path := filepath.Join(dir, "stale.html")
				err := os.WriteFile(path, []byte("<html>stale</html>"), 0o644)
				if err != nil {
					t.Fatalf("写入测试文件失败: %v", err)
				}
				oldTime := time.Now().Add(-2 * time.Hour)
				err = os.Chtimes(path, oldTime, oldTime)
				if err != nil {
					t.Fatalf("修改文件时间失败: %v", err)
				}
				return path, 1 * time.Hour
			},
			want:       "",
			wantErr:    true,
			checkStale: true,
		},
		{
			name: "文件不存在返回错误",
			setup: func(dir string) (string, time.Duration) {
				return filepath.Join(dir, "missing.html"), 1 * time.Hour
			},
			want:       "",
			wantErr:    true,
			checkStale: false,
		},
		{
			name: "maxAge为零始终返回内容",
			setup: func(dir string) (string, time.Duration) {
				path := filepath.Join(dir, "zero_maxage.html")
				err := os.WriteFile(path, []byte("<html>no expiry</html>"), 0o644)
				if err != nil {
					t.Fatalf("写入测试文件失败: %v", err)
				}
				return path, 0
			},
			want:       "<html>no expiry</html>",
			wantErr:    false,
			checkStale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			cachePath, maxAge := tt.setup(tmpDir)

			// When
			got, err := store.LoadFromCache(cachePath, maxAge)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误，实际为 nil")
				}
				if tt.checkStale && !errors.Is(err, store.ErrCacheStale) {
					t.Errorf("错误类型 = %v, 期望 errors.Is(ErrCacheStale)", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("未预期的错误: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("内容 = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// TestCacheExists 测试缓存文件存在性检查。
func TestCacheExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(dir string) string
		want  bool
	}{
		{
			name: "文件存在返回true",
			setup: func(dir string) string {
				path := filepath.Join(dir, "exists.html")
				err := os.WriteFile(path, []byte("data"), 0o644)
				if err != nil {
					t.Fatalf("写入测试文件失败: %v", err)
				}
				return path
			},
			want: true,
		},
		{
			name: "文件不存在返回false",
			setup: func(dir string) string {
				return filepath.Join(dir, "missing.html")
			},
			want: false,
		},
		{
			name: "路径是目录返回false",
			setup: func(dir string) string {
				subdir := filepath.Join(dir, "subdir")
				err := os.Mkdir(subdir, 0o755)
				if err != nil {
					t.Fatalf("创建子目录失败: %v", err)
				}
				return subdir
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			cachePath := tt.setup(tmpDir)

			// When
			got := store.CacheExists(cachePath)

			// Then
			if got != tt.want {
				t.Errorf("CacheExists(%q) = %v, 期望 %v", cachePath, got, tt.want)
			}
		})
	}
}
