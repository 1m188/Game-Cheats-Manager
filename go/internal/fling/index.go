// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"fmt"
	"time"
)

// CacheFetcher 定义缓存获取接口，便于测试注入。
//
// 生产环境中由 store 包实现，调用真实 HTTP 和磁盘 I/O。
// 测试中使用内存 mock 实现替代。
type CacheFetcher interface {
	// FetchAndCache 从指定 URL 抓取内容并缓存到 cachePath。
	FetchAndCache(url, cachePath string) ([]byte, error)
	// LoadFromCache 从 cachePath 加载缓存内容；若缓存超过 maxAge 则返回错误。
	LoadFromCache(cachePath string, maxAge time.Duration) ([]byte, error)
}

const (
	// archiveURL 是 archive.flingtrainer.com 的 URL。
	archiveURL = "https://archive.flingtrainer.com/"
	// mainURL 是 flingtrainer.com 全量修改器列表页的 URL。
	mainURL = "https://flingtrainer.com/all-trainers-a-z/"
	// archiveCache 是 archive 页面的磁盘缓存路径。
	archiveCache = "fling-data/cache/fling_archive.html"
	// mainCache 是 main 页面的磁盘缓存路径。
	mainCache = "fling-data/cache/fling_main.html"
)

// BuildIndex 构建修改器索引，优先使用缓存，过期时重新抓取。
//
// 流程（等价 Python other_threads.py:145-205）：
//  1. 从 config.CacheTTLHours 计算 maxAge
//  2. 尝试从缓存加载 fling_archive.html；未命中或过期则从 archiveURL 抓取
//  3. 尝试从缓存加载 fling_main.html；未命中或过期则从 mainURL 抓取
//  4. 分别调用 ParseArchiveHTML 和 ParseMainHTML 解析
//  5. 返回 Index{Archive, Main, FetchedAt: time.Now()}
//
// 参数:
//   - cache: 缓存获取实现（生产环境为 store 包的 FetchAndCache / LoadFromCache）
//   - config: 应用配置（含缓存 TTL）
//
// 返回值:
//   - *Index: 包含 archive 和 main 两个数据源的修改器索引
//   - error: 抓取或解析失败时返回
func BuildIndex(cache CacheFetcher, config *Config) (*Index, error) {
	maxAge := time.Duration(config.CacheTTLHours) * time.Hour

	// 步骤 1-2: 加载或抓取 archive 页面
	archiveData, err := cache.LoadFromCache(archiveCache, maxAge)
	if err != nil {
		archiveData, err = cache.FetchAndCache(archiveURL, archiveCache)
		if err != nil {
			return nil, fmt.Errorf("网络错误: 获取 archive 页面失败: %w", err)
		}
	}

	// 步骤 3: 加载或抓取 main 页面
	mainData, err := cache.LoadFromCache(mainCache, maxAge)
	if err != nil {
		mainData, err = cache.FetchAndCache(mainURL, mainCache)
		if err != nil {
			return nil, fmt.Errorf("网络错误: 获取 main 页面失败: %w", err)
		}
	}

	// 步骤 4: 解析 HTML
	archive, err := ParseArchiveHTML(archiveData)
	if err != nil {
		return nil, fmt.Errorf("解析错误: 解析 archive 页面失败: %w", err)
	}

	main, err := ParseMainHTML(mainData)
	if err != nil {
		return nil, fmt.Errorf("解析错误: 解析 main 页面失败: %w", err)
	}

	// 步骤 5: 构建索引
	return &Index{
		Archive:   archive,
		Main:      main,
		FetchedAt: time.Now(),
	}, nil
}
