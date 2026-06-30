// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import "strings"

// SymbolReplacement 将游戏名中的特殊字符转换为文件系统安全字符。
// 参考 Python download_base_thread.py:300-302。
func SymbolReplacement(name string) string {
	name = strings.ReplaceAll(name, ": ", " - ")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "?", "")
	return name
}
