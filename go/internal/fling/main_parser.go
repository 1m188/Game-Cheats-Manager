// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// mainBaseURL 是主站的基础 URL，用于拼接可能为相对路径的 href。
const mainBaseURL = "https://flingtrainer.com/"

// originMain 是 flingtrainer.com 主站来源标识。
const originMain = "fling_main"

// ParseMainHTML 解析 flingtrainer.com/all-trainers-a-z/ 页面的 HTML，
// 返回所有解析出的 Trainer 列表。
//
// 对应 Python download_display_thread.py:236-257 的逻辑：
//   - 遍历所有 div.letter-section > ul > li > a
//   - 去除链接文本中末尾的 " Trainer" 后缀得到 GameName
//   - 从 href 属性直接获取完整 URL（主站链接为绝对路径）
//   - Origin 固定为 "fling_main"
//
// 参数 html 是 flingtrainer.com/all-trainers-a-z/ 页面的字节内容。
// 返回解析出的 Trainer 切片（可能为空，但不会是 nil）以及 error（始终为 nil）。
func ParseMainHTML(html []byte) ([]Trainer, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		// goquery 对无效 HTML 容错性极高，实际上不会返回 error
		return []Trainer{}, fmt.Errorf("解析错误: 解析主站 HTML 失败: %w", err)
	}

	var trainers []Trainer

	// 对应 Python: mainSiteData.find_all(class_='letter-section')
	doc.Find("div.letter-section ul li a").Each(func(_ int, link *goquery.Selection) {
		// 1. rawTrainerName = link.get_text().strip()
		rawTrainerName := strings.TrimSpace(link.Text())

		// 2. gameName = rawTrainerName.rsplit(" Trainer", 1)[0]
		//
		//    使用 strings.LastIndex 从右侧查找 " Trainer" 并截断，
		//    等价于 Python 的 rsplit(" Trainer", 1)[0]。
		//    对于 "Sekiro: Shadows Die Twice Trainer" 这种名称中
		//    不含其他 " Trainer" 的游戏，也正确处理。
		var gameName string
		if idx := strings.LastIndex(rawTrainerName, " Trainer"); idx > 0 {
			gameName = rawTrainerName[:idx]
		} else {
			// 不含 " Trainer" 后缀时，使用原始名称
			gameName = rawTrainerName
		}

		// 3. url = link.get("href")
		//    如果 href 是相对路径（以 "/" 开头），拼接主站基础 URL
		rawURL := link.AttrOr("href", "")
		parsedURL := rawURL
		if strings.HasPrefix(rawURL, "/") {
			parsed, err := url.JoinPath(mainBaseURL, rawURL)
			if err == nil {
				parsedURL = parsed
			}
		}

		// 4. 跳过空名称的条目（对应 Python 的 if gameName and ...）
		if gameName == "" {
			return
		}

		trainers = append(trainers, Trainer{
			GameName:    gameName,
			URL:         parsedURL,
			Origin:      originMain,
			Version:     "",
			TrainerName: "",
		})
	})

	return trainers, nil
}
