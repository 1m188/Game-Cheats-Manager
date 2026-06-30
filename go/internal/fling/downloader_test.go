// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// extractFilename 单元测试
// =============================================================================

const testFilenameZip = "dredge.zip"

// TestExtractFilename_标准字段 测试 filename= 和 filename*= 的各种格式。
func TestExtractFilename_标准字段(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentDisp string
		requestURL  string
		want        string
	}{
		{
			name:        "标准filename_提取文件名",
			contentDisp: `attachment; filename="dredge.zip"`,
			requestURL:  testDownloadURL,
			want:        testFilenameZip,
		},
		{
			name:        "filename_不带引号",
			contentDisp: `attachment; filename=dredge.zip`,
			requestURL:  testDownloadURL,
			want:        testFilenameZip,
		},
		{
			name:        "RFC5987_filename星号_UTF8编码",
			contentDisp: `attachment; filename*=UTF-8''%E6%B5%8B%E8%AF%95.zip`,
			requestURL:  testDownloadURL,
			want:        "测试.zip",
		},
		{
			name:        "RFC5987_filename星号_空格编码为20",
			contentDisp: `attachment; filename*=UTF-8''My%20Trainer%20v1.0.zip`,
			requestURL:  testDownloadURL,
			want:        "My Trainer v1.0.zip",
		},
		{
			name:        "同时存在filename星号和filename_优先filename星号",
			contentDisp: `attachment; filename="fallback.zip"; filename*=UTF-8''%E4%BC%98%E5%85%88.zip`,
			requestURL:  testDownloadURL,
			want:        "优先.zip",
		},
		{
			name:        "filename星号_非UTF8编码_回退到filename",
			contentDisp: `attachment; filename="fallback.zip"; filename*=ISO-8859-1''broken.zip`,
			requestURL:  testDownloadURL,
			want:        "fallback.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				Header:  http.Header{},
				Request: httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.requestURL, http.NoBody),
			}
			if tt.contentDisp != "" {
				resp.Header.Set("Content-Disposition", tt.contentDisp)
			}

			got := extractFilename(resp, tt.requestURL)

			if got != tt.want {
				t.Errorf("extractFilename = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractFilename_URL回退 测试无 Content-Disposition 时从 URL 提取文件名。
func TestExtractFilename_URL回退(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		requestURL string
		want       string
	}{
		{
			name:       "从URL提取basename",
			requestURL: "https://flingtrainer.com/download/trainer/12345/dredge.zip",
			want:       testFilenameZip,
		},
		{
			name:       "URL带查询参数",
			requestURL: "https://example.com/download?id=42&file=dredge.zip",
			want:       "download",
		},
		{
			name:       "URL路径为空",
			requestURL: "https://example.com",
			want:       "",
		},
		{
			name:       "空Content-Disposition_回退到URL",
			requestURL: "https://flingtrainer.com/download/trainer/12345/file.zip",
			want:       "file.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{
				Header:  http.Header{},
				Request: httptest.NewRequestWithContext(context.Background(), http.MethodGet, tt.requestURL, http.NoBody),
			}

			got := extractFilename(resp, tt.requestURL)

			if got != tt.want {
				t.Errorf("extractFilename = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractFilename_fallbackURL 验证 fallbackURL 参数优先于 resp.Request.URL。
func TestExtractFilename_fallbackURL(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		Header:  http.Header{},
		Request: httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://other.example.com/other.zip", http.NoBody),
	}
	fallbackURL := "https://primary.example.com/expected.zip"

	got := extractFilename(resp, fallbackURL)

	const want = "expected.zip"
	if got != want {
		t.Errorf("extractFilename = %q, want %q", got, want)
	}
}

// =============================================================================
// DownloadFile 单元测试
// =============================================================================

func newDownloadServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("Content-Disposition", `attachment; filename="test.zip"`)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(body)
		if err != nil {
			t.Errorf("httptest handler 写入失败: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func makeTestContent(size int) []byte {
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}
	return content
}

// TestDownloadFile_正常下载 测试不同大小文件的流式下载流程。
func TestDownloadFile_正常下载(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size int
	}{
		{name: "64KB文件_完整下载", size: 64 * 1024},
		{name: "1字节文件_最小下载", size: 1},
		{name: "128KB文件_大文件下载", size: 128 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newDownloadServer(t, makeTestContent(tt.size))
			destPath := filepath.Join(t.TempDir(), "downloaded.zip")
			progress := make(chan DownloadProgress, (tt.size/32768)+5)

			_, err := DownloadFile(srv.URL, destPath, progress)
			close(progress)

			if err != nil {
				t.Fatalf("DownloadFile 返回错误: %v", err)
			}

			info, err := os.Stat(destPath)
			if err != nil {
				t.Fatalf("stat 下载文件失败: %v", err)
			}
			if info.Size() != int64(tt.size) {
				t.Errorf("文件大小: got %d, want %d", info.Size(), tt.size)
			}

			var updates []DownloadProgress
			for p := range progress {
				updates = append(updates, p)
			}
			if len(updates) == 0 {
				t.Error("进度 channel 未收到任何更新")
			}
		})
	}
}

// TestDownloadFile_进度更新非零 验证进度 channel 收到有意义的更新。
func TestDownloadFile_进度更新非零(t *testing.T) {
	t.Parallel()

	const contentSize = 256 * 1024
	srv := newDownloadServer(t, makeTestContent(contentSize))
	destPath := filepath.Join(t.TempDir(), "progress_test.zip")
	progress := make(chan DownloadProgress, (contentSize/32768)+5)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	var updates []DownloadProgress
	for p := range progress {
		updates = append(updates, p)
	}

	if len(updates) < 2 {
		t.Errorf("进度更新次数: got %d, want >= 2", len(updates))
	}

	for i := 1; i < len(updates); i++ {
		if updates[i].BytesDownloaded <= updates[i-1].BytesDownloaded {
			t.Errorf("进度 %d BytesDownloaded 未递增: %d → %d",
				i, updates[i-1].BytesDownloaded, updates[i].BytesDownloaded)
		}
	}

	if len(updates) > 0 {
		last := updates[len(updates)-1]
		if last.TotalBytes != contentSize {
			t.Errorf("TotalBytes: got %d, want %d", last.TotalBytes, contentSize)
		}
		if last.PercentComplete < 99.0 || last.PercentComplete > 101.0 {
			t.Errorf("PercentComplete: got %.2f, want in [99, 101]", last.PercentComplete)
		}
	}
}

// TestDownloadFile_HTTP错误 测试 HTTP 非 200 状态码时返回错误并清理部分文件。
func TestDownloadFile_HTTP错误(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
	}{
		{name: "HTTP_404_返回错误", statusCode: http.StatusNotFound},
		{name: "HTTP_500_返回错误", statusCode: http.StatusInternalServerError},
		{name: "HTTP_403_返回错误", statusCode: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			t.Cleanup(srv.Close)

			destPath := filepath.Join(t.TempDir(), "error_test.zip")
			progress := make(chan DownloadProgress, 10)

			_, err := DownloadFile(srv.URL, destPath, progress)
			close(progress)

			if err == nil {
				t.Fatal("期望返回错误，但 err 为 nil")
			}

			_, statErr := os.Stat(destPath)
			if statErr == nil {
				t.Error("下载失败后部分文件应被清理，但文件仍存在")
			}
		})
	}
}

// TestDownloadFile_保存到正确路径 验证文件保存到指定的 destPath（含子目录自动创建）。
func TestDownloadFile_保存到正确路径(t *testing.T) {
	t.Parallel()

	const contentSize = 1024
	srv := newDownloadServer(t, makeTestContent(contentSize))

	destDir := filepath.Join(t.TempDir(), "sub", "dir")
	destPath := filepath.Join(destDir, "custom_name.bin")
	progress := make(chan DownloadProgress, 10)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	data, err := os.ReadFile(destPath) //nolint:gosec // 测试文件路径由 t.TempDir() 生成，安全
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if len(data) != contentSize {
		t.Errorf("文件大小: got %d, want %d", len(data), contentSize)
	}
}

// TestDownloadFile_下载内容完整性 验证逐字节比对下载内容与原始数据一致。
func TestDownloadFile_下载内容完整性(t *testing.T) {
	t.Parallel()

	const contentSize = 16 * 1024
	content := make([]byte, contentSize)
	for i := range content {
		content[i] = byte(255 - (i % 256))
	}
	srv := newDownloadServer(t, content)

	destPath := filepath.Join(t.TempDir(), "integrity_test.bin")
	progress := make(chan DownloadProgress, (contentSize/32768)+5)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	got, err := os.ReadFile(destPath) //nolint:gosec // 测试文件路径由 t.TempDir() 生成，安全
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if len(got) != len(content) {
		t.Fatalf("文件大小不匹配: got %d, want %d", len(got), len(content))
	}
	for i := range content {
		if got[i] != content[i] {
			t.Fatalf("字节 %d 不匹配: got 0x%02x, want 0x%02x", i, got[i], content[i])
		}
	}
}

// TestDownloadFile_大缓冲区channel 验证非阻塞进度发送在足够大缓冲区时不丢失数据。
func TestDownloadFile_大缓冲区channel(t *testing.T) {
	t.Parallel()

	const contentSize = 512 * 1024
	srv := newDownloadServer(t, makeTestContent(contentSize))

	destPath := filepath.Join(t.TempDir(), "buffer_test.bin")
	progress := make(chan DownloadProgress, (contentSize/32768)+100)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	count := 0
	for range progress {
		count++
	}
	if count < 2 {
		t.Errorf("进度更新次数过少: got %d, want >= 2", count)
	}
}

// TestDownloadFile_nilChannel 验证 progress channel 为 nil 时不 panic。
func TestDownloadFile_nilChannel(t *testing.T) {
	t.Parallel()

	const contentSize = 1024
	srv := newDownloadServer(t, makeTestContent(contentSize))

	destPath := filepath.Join(t.TempDir(), "nil_channel_test.bin")

	_, err := DownloadFile(srv.URL, destPath, nil)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat 下载文件失败: %v", err)
	}
	if info.Size() != contentSize {
		t.Errorf("文件大小: got %d, want %d", info.Size(), contentSize)
	}
}

// TestDownloadFile_完整HTTP流程 测试真实的 HTTP 请求/响应流程，含 User-Agent 验证。
func TestDownloadFile_完整HTTP流程(t *testing.T) {
	t.Parallel()

	const contentSize = 4 * 1024
	content := makeTestContent(contentSize)

	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Length", strconv.Itoa(contentSize))
		w.Header().Set("Content-Disposition", `attachment; filename="full_test.zip"`)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(content)
		if err != nil {
			t.Errorf("httptest handler 写入失败: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	destPath := filepath.Join(t.TempDir(), "full_test.zip")
	progress := make(chan DownloadProgress, 10)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	if gotUserAgent == "" {
		t.Error("HTTP 请求未携带 User-Agent 头部")
	}
	if !strings.Contains(gotUserAgent, "Chrome") {
		t.Errorf("User-Agent 不包含 Chrome: %s", gotUserAgent)
	}

	got, err := os.ReadFile(destPath) //nolint:gosec // 测试文件路径由 t.TempDir() 生成，安全
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if len(got) != contentSize {
		t.Errorf("文件大小: got %d, want %d", len(got), contentSize)
	}
}

// TestDownloadFile_ContentLength缺失 测试响应缺少 Content-Length 头部时的行为。
func TestDownloadFile_ContentLength缺失(t *testing.T) {
	t.Parallel()

	const contentSize = 8 * 1024
	content := make([]byte, contentSize)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(content)
		if err != nil {
			t.Errorf("httptest handler 写入失败: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	destPath := filepath.Join(t.TempDir(), "no_cl_test.bin")
	progress := make(chan DownloadProgress, 10)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		t.Fatalf("DownloadFile 返回错误: %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat 下载文件失败: %v", err)
	}
	if info.Size() != contentSize {
		t.Errorf("文件大小: got %d, want %d", info.Size(), contentSize)
	}

	var updates []DownloadProgress
	for p := range progress {
		updates = append(updates, p)
	}
	if len(updates) > 0 {
		last := updates[len(updates)-1]
		if last.TotalBytes != 0 {
			t.Errorf("无 Content-Length 时 TotalBytes 应为 0: got %d", last.TotalBytes)
		}
	}
}

// TestDownloadFile_服务器中途断开 测试服务器发送数据少于声明 Content-Length 时的行为。
func TestDownloadFile_服务器中途断开(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "10240")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, "short")
		if err != nil {
			return
		}
	}))
	t.Cleanup(srv.Close)

	destPath := filepath.Join(t.TempDir(), "partial_test.bin")
	progress := make(chan DownloadProgress, 10)

	_, err := DownloadFile(srv.URL, destPath, progress)
	close(progress)

	if err != nil {
		_, statErr := os.Stat(destPath)
		if statErr == nil {
			t.Logf("下载错误: %v，文件可能存在", err)
			_ = os.Remove(destPath) //nolint:errcheck // 测试辅助清理，文件可能已不存在
		}
	}
}
