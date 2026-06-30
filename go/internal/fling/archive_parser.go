// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	// originArchive 是 archive.flingtrainer.com 来源标识。
	originArchive = "fling_archive"
)

var (
	// versionRemovalRe 是去除修改器名称中版本后缀的正则表达式。
	// 精确复制自 Python download_display_thread.py:166。
	versionRemovalRe = regexp.MustCompile(` v[\d.]+.*|\.\bv.*| \d+\.\d+\.\d+.*| Plus\s\d+.*|Build\s\d+.*|(\d+\.\d+-Update.*)|Update\s\d+.*|\(Update\s.*| Early Access .*|\.Early.Access.*`)

	// ignoredTrainers 是硬编码的忽略修改器列表。
	// 精确复制自 Python download_display_thread.py:168-173。
	ignoredTrainers = map[string]bool{
		"Dying Light The Following Enhanced Edition": true,
		"Monster Hunter World":                       true,
		"Street Fighter V":                           true,
		"World War Z":                                true,
	}
)

// ParseArchiveHTML 解析 archive.flingtrainer.com 页面的 HTML，返回所有解析出的 Trainer 列表。
//
// 解析流程（等价 Python download_display_thread.py:152-192）：
//   - 使用 CSS 选择器 a[target="_self"] 提取所有链接
//   - 正则去除版本后缀（精确复制 Python 第 166 行正则）
//   - 下划线替换为冒号空格
//   - 跳过硬编码忽略列表中的 4 个游戏
//   - "Bright.Memory.Episode.1" 特殊转换为 "Bright Memory: Episode 1"
//   - 使用 url.JoinPath 拼接完整 URL
//   - Origin 固定为 "fling_archive"，Version 和 TrainerName 为空
//
// 参数 html 是 archive.flingtrainer.com 页面的字节内容。
// 返回解析出的 Trainer 切片（空 HTML 返回空切片）以及 parse error。
func ParseArchiveHTML(html []byte) ([]Trainer, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("解析错误: goquery 解析 HTML 失败: %w", err)
	}

	baseURL, err := url.Parse("https://archive.flingtrainer.com/")
	if err != nil {
		// 硬编码 baseURL 不应该解析失败
		return nil, fmt.Errorf("解析错误: 解析 fling_archive 基础 URL 失败: %w", err)
	}

	trainers := make([]Trainer, 0)

	// 对应 Python: archiveData.find_all(target="_self")
	doc.Find("a[target='_self']").Each(func(_ int, link *goquery.Selection) {
		// 1. rawTrainerName = link.get_text()
		rawTrainerName := link.Text()

		// 2. 保留去除版本后缀的干净名，用于忽略列表匹配
		strippedName := versionRemovalRe.ReplaceAllString(rawTrainerName, "")
		strippedName = strings.ReplaceAll(strippedName, "_", ": ")
		strippedName = strings.TrimSpace(strippedName)

		// 3. GameName 保留版本信息（仅替换 _ → : ），便于搜索结果中区分不同版本
		gameName := strings.ReplaceAll(rawTrainerName, "_", ": ")
		gameName = strings.TrimSpace(gameName)

		// 4. 特殊处理 "Bright.Memory.Episode.1" → "Bright Memory: Episode 1"
		if strippedName == "Bright.Memory.Episode.1" {
			gameName = "Bright Memory: Episode 1"
		}

		// 5. 跳过忽略列表中的游戏（使用去除版本后缀后的名判断）
		if ignoredTrainers[strippedName] {
			return
		}

		// 5. url = urljoin("https://archive.flingtrainer.com/", link.get("href"))
		href := link.AttrOr("href", "")
		trainerURL := baseURL.JoinPath(href).String()

		trainers = append(trainers, Trainer{
			GameName:    gameName,
			URL:         trainerURL,
			Origin:      originArchive,
			Version:     "",
			TrainerName: "",
		})
	})

	return trainers, nil
}
