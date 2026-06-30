// Package store 提供 HTTP 抓取、磁盘缓存和配置持久化功能。
// 所有运行时文件收敛于 ./fling-data/ 目录中。
package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// userAgent 模拟 Chrome 96 浏览器的 User-Agent，用于 HTTP 请求。
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110 Safari/537.36"

	// httpTimeout 是 HTTP 请求的总超时时间。
	httpTimeout = 30 * time.Second
)

var (
	// ErrCacheStale 表示缓存文件存在但已过期，需要重新抓取。
	ErrCacheStale = errors.New("缓存已过期")

	// httpClient 是共享的 HTTP 客户端，配置了 30 秒超时。
	httpClient = &http.Client{
		Timeout: httpTimeout,
	}
)

// FetchAndCache 从指定 URL 抓取内容并保存到磁盘缓存。
//
// 参数:
//   - url: 要抓取的网页地址
//   - cachePath: 缓存文件的写入路径（如 ./fling-data/cache/page.html）
//
// 返回值:
//   - []byte: 抓取到的响应体
//   - error: 网络错误、非 200 状态码或文件写入失败时返回
func FetchAndCache(url, cachePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("网络错误: 创建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("网络错误: 请求 %s 失败: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Close 在 defer 中，忽略错误是标准实践

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("网络错误: HTTP %d — 请求 %s 失败", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("网络错误: 读取响应体失败: %w", err)
	}

	// 确保缓存文件的父目录存在
	//nolint:gosec // 缓存目录需对用户可读写，0o755 是合理权限
	err = os.MkdirAll(filepath.Dir(cachePath), 0o755)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 创建缓存文件目录失败: %w", err)
	}

	//nolint:gosec // 缓存文件为 HTML 内容，需对用户可读，0o644 是合理权限
	err = os.WriteFile(cachePath, body, 0o644)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 写入缓存文件 %s 失败: %w", cachePath, err)
	}

	return body, nil
}

// LoadFromCache 从磁盘缓存加载文件内容，并在超过最大年龄时返回 ErrCacheStale。
//
// 参数:
//   - cachePath: 缓存文件的路径
//   - maxAge: 缓存的最大有效时长。为 0 时跳过过期检查（始终返回内容）
//
// 返回值:
//   - []byte: 缓存文件的内容
//   - error: 文件不存在、读取失败、或过期时返回错误（过期错误可通过 errors.Is(err, ErrCacheStale) 判断）
func LoadFromCache(cachePath string, maxAge time.Duration) ([]byte, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 获取缓存文件信息失败 %s: %w", cachePath, err)
	}

	if maxAge > 0 {
		age := time.Since(info.ModTime())
		if age > maxAge {
			return nil, fmt.Errorf("缓存已过期 (年龄 %v, 最大 %v): %w", age, maxAge, ErrCacheStale)
		}
	}

	data, err := os.ReadFile(filepath.Clean(cachePath))
	if err != nil {
		return nil, fmt.Errorf("存储错误: 读取缓存文件失败 %s: %w", cachePath, err)
	}

	return data, nil
}

// CacheExists 检查指定路径的缓存文件是否存在且为普通文件。
//
// 参数:
//   - cachePath: 缓存文件的路径
//
// 返回值:
//   - bool: 文件存在且为普通文件时返回 true，否则返回 false
func CacheExists(cachePath string) bool {
	info, err := os.Stat(cachePath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
