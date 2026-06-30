// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import "time"

type (
	// Trainer 表示从 FLiNG 网站解析出的单个修改器信息。
	Trainer struct {
		GameName    string `json:"game_name"`    // 清洗后的游戏名（去除版本号）
		URL         string `json:"url"`          // flingtrainer.com 完整 URL
		Origin      string `json:"origin"`       // "fling_main" 或 "fling_archive"
		Version     string `json:"version"`      // YYYY.MM.DD 格式版本号
		TrainerName string `json:"trainer_name"` // 展示名: "[FL] Game Name Trainer"
	}

	// SearchResult 是搜索结果的展示类型，与 Trainer 结构相同（别名）。
	SearchResult = Trainer

	// Index 表示从 FLiNG 网站解析出的修改器索引，供搜索使用。
	Index struct {
		Archive   []Trainer `json:"archive"`    // archive.flingtrainer.com 的解析结果
		Main      []Trainer `json:"main"`       // flingtrainer.com/all-trainers-a-z/ 的解析结果
		FetchedAt time.Time `json:"fetched_at"` // 数据抓取时间戳
	}

	// Config 是应用运行时配置，持久化到 ./fling-data/config.json。
	Config struct {
		CacheTTLHours int       `json:"cache_ttl_hours"` // 缓存默认过期时间，单位：小时，默认 24
		DownloadPath  string    `json:"download_path"`   // 修改器下载路径，默认 "./fling-data/trainers/"
		LastFetch     time.Time `json:"last_fetch"`      // 上次抓取时间戳
	}

	// DownloadProgress 表示下载进度，通过 channel 从 downloader 传递到 TUI。
	DownloadProgress struct {
		BytesDownloaded int64   `json:"bytes_downloaded"` // 已下载字节数
		TotalBytes      int64   `json:"total_bytes"`      // 总字节数
		PercentComplete float64 `json:"percent_complete"` // 完成百分比 (0.0-100.0)
	}
)
