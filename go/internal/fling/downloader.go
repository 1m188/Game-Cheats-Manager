// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	nurl "net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	downloadUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/96.0.4664.110 Safari/537.36"
	downloadTimeout   = 30 * time.Second
	chunkSize         = 32 * 1024
)

var (
	downloadClient = &http.Client{
		Timeout: downloadTimeout,
	}
	errBadStatus = errors.New("HTTP 状态码非 200")
)

// extractFilename 从 HTTP 响应头的 Content-Disposition 字段提取文件名。
//
// 优先级（与 Python download_base_thread.py:183-197 一致）：
//  1. filename*=（RFC 5987 编码）
//  2. filename=（标准文件名）
//  3. fallbackURL 路径的 basename
func extractFilename(resp *http.Response, fallbackURL string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return fallbackBasename(fallbackURL)
	}

	if filenameStar := extractFilenameStar(cd); filenameStar != "" {
		return filenameStar
	}

	if filename := extractFilenameParam(cd); filename != "" {
		return filename
	}

	return fallbackBasename(fallbackURL)
}

// extractFilenameStar 解析 RFC 5987 格式的 filename*= 参数（仅支持 UTF-8）。
func extractFilenameStar(cd string) string {
	const prefix = "filename*="
	idx := strings.Index(cd, prefix)
	if idx == -1 {
		return ""
	}

	raw := cd[idx+len(prefix):]
	raw = strings.TrimSpace(raw)
	if semiIdx := strings.IndexByte(raw, ';'); semiIdx != -1 {
		raw = raw[:semiIdx]
	}
	raw = strings.Trim(raw, `";`)

	parts := strings.SplitN(raw, "'", 3)
	if len(parts) < 3 {
		return ""
	}

	if !strings.EqualFold(parts[0], "UTF-8") {
		return ""
	}

	decoded, err := nurl.QueryUnescape(parts[2])
	if err != nil {
		return ""
	}

	return decoded
}

// extractFilenameParam 解析标准 filename= 参数（带或不带引号）。
func extractFilenameParam(cd string) string {
	const prefix = "filename="
	idx := strings.Index(cd, prefix)
	if idx == -1 {
		return ""
	}

	raw := cd[idx+len(prefix):]
	raw = strings.TrimSpace(raw)

	if strings.HasPrefix(raw, `"`) {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			if name, ok := params["filename"]; ok {
				return name
			}
		}
	}

	if semiIdx := strings.IndexByte(raw, ';'); semiIdx != -1 {
		raw = raw[:semiIdx]
	}
	raw = strings.Trim(raw, `"; `)

	return raw
}

// fallbackBasename 从 URL 路径提取 basename 作为文件名回退。
func fallbackBasename(rawURL string) string {
	parsed, err := nurl.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

// DownloadFile 从指定 URL 流式下载文件，返回实际文件名（优先 Content-Disposition）。
//
// 使用 32KB 分块流式读取，避免将整个文件加载到内存。
// 进度更新通过 select+default 非阻塞发送。
// 下载失败时自动清理部分写入的文件。
// 返回 (actualFilename, error)：actualFilename 优先取 Content-Disposition header，
// 不存在时回退到 destPath 的 basename。
func DownloadFile(url, destPath string, progress chan<- DownloadProgress) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("网络错误: 创建 HTTP 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", downloadUserAgent)

	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("网络错误: HTTP 请求失败 (%s): %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // Close 在 defer 中，忽略错误是标准实践

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("网络错误: %w (HTTP %d, %s)", errBadStatus, resp.StatusCode, url)
	}

	//nolint:gosec // 下载目录需对用户可读写，0o755 是合理权限
	err = os.MkdirAll(filepath.Dir(destPath), 0o755)
	if err != nil {
		return "", fmt.Errorf("存储错误: 创建下载目录失败: %w", err)
	}

	//nolint:gosec // 下载文件需对用户可读写，0o644 是合理权限
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("存储错误: 创建下载文件失败 %s: %w", destPath, err)
	}
	defer file.Close() //nolint:errcheck // Close 在 defer 中

	totalBytes := resp.ContentLength
	if totalBytes < 0 {
		totalBytes = 0
	}

	err = streamDownload(resp.Body, file, totalBytes, progress)
	if err != nil {
		cleanupPartialFile(destPath)
		return "", err
	}

	// 优先使用 Content-Disposition 文件名，回退到 destPath basename
	actualName := extractFilename(resp, url)
	return actualName, nil
}

// streamDownload 从 reader 流式读取数据写入 writer，同时发送进度更新。
func streamDownload(reader io.Reader, writer io.Writer, totalBytes int64, progress chan<- DownloadProgress) error {
	var bytesDownloaded int64
	buf := make([]byte, chunkSize)

	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			_, writeErr := writer.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("存储错误: 写入下载文件失败: %w", writeErr)
			}
			bytesDownloaded += int64(n)
			sendProgress(progress, bytesDownloaded, totalBytes)
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return fmt.Errorf("网络错误: 读取响应体失败: %w", readErr)
		}
	}
}

// sendProgress 非阻塞发送下载进度到 channel。
func sendProgress(progress chan<- DownloadProgress, downloaded, total int64) {
	if progress == nil {
		return
	}
	msg := DownloadProgress{
		BytesDownloaded: downloaded,
		TotalBytes:      total,
	}
	if total > 0 {
		msg.PercentComplete = float64(downloaded) / float64(total) * 100.0
	}
	select {
	case progress <- msg:
	default:
	}
}

// cleanupPartialFile 删除下载失败时产生的部分文件（忽略文件不存在的错误）。
func cleanupPartialFile(filePath string) {
	_ = os.Remove(filePath) //nolint:errcheck // 清理操作，文件可能已不存在
}
