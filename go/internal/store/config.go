// Package store 提供 HTTP 请求、磁盘缓存和配置持久化功能。
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"fling-tui/internal/fling"
)

const (
	// defaultCacheTTLHours 是缓存默认过期时间，单位：小时。
	defaultCacheTTLHours = 24
	// defaultDownloadPath 是修改器默认下载路径。
	defaultDownloadPath = "./fling-data/trainers/"
	// configFileName 是配置文件名。
	configFileName = "config.json"
)

// LoadConfig 从可执行文件所在目录下的 fling-data/config.json 加载配置。
// 如果文件不存在，返回默认配置。
func LoadConfig() (*fling.Config, error) {
	configPath, err := configFilePath()
	if err != nil {
		return nil, err
	}
	return readConfigFile(configPath)
}

// SaveConfig 将配置保存到可执行文件所在目录下的 fling-data/config.json。
func SaveConfig(c *fling.Config) error {
	configPath, err := configFilePath()
	if err != nil {
		return err
	}
	return writeConfigFile(configPath, c)
}

// configFilePath 返回可执行文件旁 fling-data/config.json 的完整路径。
func configFilePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("配置错误: 获取可执行文件路径失败: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "fling-data", configFileName), nil
}

// readConfigFile 从指定路径加载配置，文件不存在时返回默认值。
func readConfigFile(path string) (*fling.Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("配置错误: 读取配置文件失败: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("配置错误: 配置文件为空")
	}

	var cfg fling.Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("配置错误: 解析配置文件失败: %w", err)
	}
	return &cfg, nil
}

// writeConfigFile 将配置写入指定路径，自动创建父目录。
func writeConfigFile(path string, c *fling.Config) error {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0o750)
	if err != nil {
		return fmt.Errorf("配置错误: 创建配置目录失败: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("配置错误: 序列化配置失败: %w", err)
	}

	err = os.WriteFile(path, data, 0o600)
	if err != nil {
		return fmt.Errorf("配置错误: 写入配置文件失败: %w", err)
	}
	return nil
}

// defaultConfig 返回包含默认值的配置。
func defaultConfig() *fling.Config {
	return &fling.Config{
		CacheTTLHours: defaultCacheTTLHours,
		DownloadPath:  defaultDownloadPath,
	}
}
