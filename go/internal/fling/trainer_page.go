// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// HTTPFetcher 定义 HTTP GET 请求的抽象接口，用于测试注入。
// 生产环境使用 net/http.Client，测试环境使用 httptest 或 testFetcher。
type HTTPFetcher interface {
	// Get 向指定 URL 发起 GET 请求并返回响应体字节。对应 net/http 语义。
	Get(url string) ([]byte, error)
}

// trainerPageVersionRe 匹配 div.entry 文本中的版本号（YYYY.MM.DD 格式），大小写不敏感。
// 对应 Python download_trainers_thread.py:222:
//
//	pattern = r'options.*game\s*version.*last\s*updated:\s*(\d{4}\.[0-1]?\d\.[0-3]?\d)'
//	match = re.search(pattern, div_entry.get_text(separator=' ', strip=True), re.IGNORECASE)
var trainerPageVersionRe = regexp.MustCompile(`(?i)options.*game\s*version.*last\s*updated:\s*(\d{4}\.[0-1]?\d\.[0-3]?\d)`)

// collapseWhitespace 将所有空白字符（含换行）替换为单个空格，并去除首尾空白。
// 等价于 Python BeautifulSoup 的 get_text(separator=' ', strip=True)。
func collapseWhitespace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// FetchTrainerDetails 抓取修改器详情页，提取下载链接和版本号。
//
// 对应 Python download_trainers_thread.py:212-228 的逻辑：
//  1. 找到 <a target="_self" href="...flingtrainer.com..."> 提取 href 作为下载链接
//  2. 找到 div.entry 提取文本，用正则匹配版本号（YYYY.MM.DD 格式）
//
// 参数:
//   - trainerURL: 详情页完整 URL（如 https://flingtrainer.com/games/dredge-trainer/）
//   - fetcher: HTTP GET 接口（生产环境传入 net/http 适配器，测试环境传入 mock）
//
// 返回 downloadURL（文件下载链接）、version（YYYY.MM.DD 格式）、error。
func FetchTrainerDetails(trainerURL string, fetcher HTTPFetcher) (downloadURL, version string, err error) {
	html, err := fetcher.Get(trainerURL)
	if err != nil {
		return "", "", fmt.Errorf("网络错误: 抓取修改器详情页失败: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return "", "", fmt.Errorf("解析错误: 解析修改器详情页 HTML 失败: %w", err)
	}

	// Step 1: 提取下载链接 — 找到 <a target="_self" href="...flingtrainer.com...">
	// 对应 Python download_trainers_thread.py:216-218
	doc.Find("a[target='_self']").Each(func(_ int, sel *goquery.Selection) {
		// 只在尚未找到时提取，避免覆盖正确结果
		if downloadURL != "" {
			return
		}
		href, exists := sel.Attr("href")
		if exists && strings.Contains(href, "flingtrainer.com") {
			downloadURL = href
		}
	})

	if downloadURL == "" {
		return "", "", errors.New("解析错误: 无法找到修改器下载链接")
	}

	// 如果下载链接是相对路径，基于详情页 URL 拼接为完整 URL
	parsedTrainerURL, parseErr := url.Parse(trainerURL)
	if parseErr == nil {
		parsedDL, dlErr := url.Parse(downloadURL)
		if dlErr == nil {
			downloadURL = parsedTrainerURL.ResolveReference(parsedDL).String()
		}
	}

	// Step 2: 提取版本号 — 从 div.entry 文本中匹配 YYYY.MM.DD 格式
	// 对应 Python download_trainers_thread.py:220-227
	divEntry := doc.Find("div.entry")
	if divEntry.Length() == 0 {
		return "", "", errors.New("解析错误: 无法找到版本信息")
	}

	entryText := collapseWhitespace(divEntry.Text())
	match := trainerPageVersionRe.FindStringSubmatch(entryText)
	if len(match) < 2 {
		return "", "", errors.New("解析错误: 无法找到修改器版本号")
	}

	return downloadURL, match[1], nil
}
