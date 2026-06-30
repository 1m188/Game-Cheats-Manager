// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type (
	// testFetcher 是 HTTPFetcher 接口的测试替身，返回指定内容。
	testFetcher struct {
		body []byte
		err  error
	}

	// serverFetcher 通过 httptest server 客户端代理 HTTP 请求。
	serverFetcher struct {
		baseURL string
		client  *http.Client
	}
)

const (
	// testVersion 是测试夹具中的版本号。
	testVersion = "2024.03.15"
	// testDownloadURL 是测试夹具中的下载链接。
	testDownloadURL = "https://flingtrainer.com/download/trainer/12345"
)

// Get 实现 HTTPFetcher 接口，返回预设的 body 和 err。
func (f *testFetcher) Get(_ string) ([]byte, error) {
	return f.body, f.err
}

// Get 向 httptest server 发起 GET 请求并返回响应体。
func (f *serverFetcher) Get(_ string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, f.baseURL+"/", http.NoBody)
	if err != nil {
		return nil, err //nolint:wrapcheck // test helper
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err //nolint:wrapcheck // test helper
	}
	defer resp.Body.Close() //nolint:errcheck // test helper — 响应体关闭错误不影响测试结果
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err //nolint:wrapcheck // test helper
	}
	return body, nil
}

// newHTTPServerFetcher 创建 httptest 服务器，返回一个通过真实 HTTP 请求获取内容的 fetcher。
func newHTTPServerFetcher(t *testing.T, handler http.HandlerFunc) *serverFetcher {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &serverFetcher{baseURL: srv.URL, client: srv.Client()}
}

// loadFixture 从 testdata 目录加载 HTML 测试夹具。
func loadFixture(t *testing.T, filename string) []byte {
	t.Helper()
	fullPath := filepath.Join("..", "..", "testdata", filepath.Clean(filename))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("读取测试夹具 %s 失败: %v", filename, err)
	}
	return data
}

// TestFetchTrainerDetails_正常路径 测试从详情页提取下载链接和版本号。
func TestFetchTrainerDetails_正常路径(t *testing.T) {
	t.Parallel()

	normalHTML := loadFixture(t, "trainer_page.html")

	tests := []struct {
		name        string
		html        []byte
		wantURL     string
		wantVersion string
	}{
		{
			name:        "完整详情页_提取下载链接和版本号",
			html:        normalHTML,
			wantURL:     testDownloadURL,
			wantVersion: testVersion,
		},
		{
			name: "版本号格式_不完整的月日数字",
			html: []byte(`<html>
<body>
    <div class="entry">
        <p><strong>Options:</strong> God Mode</p>
        <p><strong>Game Version:</strong> v2.0</p>
        <p><strong>Last Updated:</strong> 2024.1.3</p>
    </div>
    <a href="https://flingtrainer.com/download/trainer/12345" target="_self">Download</a>
</body>
</html>`),
			wantURL:     "https://flingtrainer.com/download/trainer/12345",
			wantVersion: "2024.1.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetcher := &testFetcher{body: tt.html}
			downloadURL, version, err := FetchTrainerDetails("https://flingtrainer.com/games/dredge-trainer/", fetcher)

			if err != nil {
				t.Fatalf("不期望返回错误，但 err: %v", err)
			}
			if downloadURL != tt.wantURL {
				t.Errorf("下载链接:\n  got:  %s\n  want: %s", downloadURL, tt.wantURL)
			}
			if version != tt.wantVersion {
				t.Errorf("版本号:\n  got:  %s\n  want: %s", version, tt.wantVersion)
			}
		})
	}
}

// TestFetchTrainerDetails_错误路径 测试各种异常输入返回正确的错误。
func TestFetchTrainerDetails_错误路径(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		html        []byte
		errContains string
	}{
		{
			name: "缺少下载链接_返回错误",
			html: []byte(`<html>
<body>
    <div class="entry">
        <p><strong>Game Version:</strong> v1.5.3</p>
        <p><strong>Last Updated:</strong> 2024.03.15</p>
    </div>
</body>
</html>`),
			errContains: "无法找到修改器下载链接",
		},
		{
			name: "缺少版本信息_div_entry不存在",
			html: []byte(`<html>
<body>
    <a href="https://flingtrainer.com/download/trainer/12345" target="_self">Download</a>
</body>
</html>`),
			errContains: "无法找到版本信息",
		},
		{
			name: "版本信息存在但格式不匹配_返回错误",
			html: []byte(`<html>
<body>
    <div class="entry">
        <p>Some text without a date pattern</p>
    </div>
    <a href="https://flingtrainer.com/download/trainer/12345" target="_self">Download</a>
</body>
</html>`),
			errContains: "无法找到修改器版本号",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fetcher := &testFetcher{body: tt.html}
			_, _, err := FetchTrainerDetails("https://flingtrainer.com/games/dredge-trainer/", fetcher)

			if err == nil {
				t.Fatal("期望返回错误，但 err 为 nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("错误信息不匹配:\n  got:  %s\n  want contains: %s", err.Error(), tt.errContains)
			}
		})
	}
}

// TestFetchTrainerDetails_httptest 使用 httptest.NewServer 测试完整的 HTTP 请求路径。
func TestFetchTrainerDetails_httptest(t *testing.T) {
	t.Parallel()

	normalHTML := loadFixture(t, "trainer_page.html")
	fetcher := newHTTPServerFetcher(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write(normalHTML)
		if err != nil {
			t.Errorf("httptest handler 写入失败: %v", err)
		}
	})

	downloadURL, version, err := FetchTrainerDetails("https://flingtrainer.com/games/dredge-trainer/", fetcher)

	if err != nil {
		t.Fatalf("不期望返回错误，但 err: %v", err)
	}
	const wantURL = testDownloadURL
	if downloadURL != wantURL {
		t.Errorf("下载链接:\n  got:  %s\n  want: %s", downloadURL, wantURL)
	}
	const wantVersion = testVersion
	if version != wantVersion {
		t.Errorf("版本号:\n  got:  %s\n  want: %s", version, wantVersion)
	}
}

// TestFetchTrainerDetails_版本号大小写不敏感 验证 (?i) 标志生效。
func TestFetchTrainerDetails_版本号大小写不敏感(t *testing.T) {
	t.Parallel()

	html := []byte(`<html>
<body>
    <div class="entry">
        <p><strong>OPTIONS:</strong> God Mode</p>
        <p><strong>game version:</strong> v1.0</p>
        <p><strong>LAST UPDATED:</strong> 2024.06.20</p>
    </div>
    <a href="https://flingtrainer.com/download/trainer/42" target="_self">Download</a>
</body>
</html>`)
	fetcher := &testFetcher{body: html}

	downloadURL, version, err := FetchTrainerDetails("https://flingtrainer.com/games/test/", fetcher)

	if err != nil {
		t.Fatalf("不期望返回错误，但 err: %v", err)
	}
	const wantVersion = "2024.06.20"
	if version != wantVersion {
		t.Errorf("版本号:\n  got:  %s\n  want: %s", version, wantVersion)
	}
	const wantURL = "https://flingtrainer.com/download/trainer/42"
	if downloadURL != wantURL {
		t.Errorf("下载链接:\n  got:  %s\n  want: %s", downloadURL, wantURL)
	}
}
