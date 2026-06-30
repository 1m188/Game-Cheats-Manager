// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gcmInfoData 表示 gcm_info.json 的数据结构。
type gcmInfoData struct {
	GameName string `json:"game_name"`
	Origin   string `json:"origin"`
	Version  string `json:"version,omitempty"`
}

// 多版本拆分用正则，对标 Python download_trainers_thread.py:296-304。
var (
	reMainVer    = regexp.MustCompile(`(?i)trainer(.*)\.exe`)
	reArchiveVer = regexp.MustCompile(`\s+Update.*|\s+v\d+.*`)
)

// sentinel errors for organizer
var (
	errNoExeFiles           = errors.New("exeFiles 列表为空")
	ErrTrainerAlreadyExists = errors.New("修改器已存在，跳过下载")
)

// trainerAlreadyInstalled 检查目标目录是否已安装该修改器。
func trainerAlreadyInstalled(downloadPath, safeName string) bool {
	targetDir := filepath.Join(downloadPath, safeName)
	infoPath := filepath.Join(targetDir, "gcm_info.json")
	_, err := os.Stat(infoPath)
	return err == nil
}

// OrganizeTrainer 将解压后的修改器文件组织到目标目录。
// exeFiles: 提取出的 .exe 文件名列表（仅文件名，来自 extractor 的输出）
// tempDir: 临时解压目录（解压文件所在位置）
// trainerName: 修改器展示名（如 "[FL] DREDGE Trainer"）
// origin: 来源标识（"fling_main" 或 "fling_archive"）
// version: 版本号（YYYY.MM.DD 格式，可为空）
// downloadPath: 修改器下载根目录（如 "./fling-data/trainers/"）
// 对标 Python download_trainers_thread.py:284-315。
func OrganizeTrainer(exeFiles []string, tempDir, trainerName, origin, version, downloadPath string) error {
	if len(exeFiles) == 0 {
		return errNoExeFiles
	}

	safeName := SymbolReplacement(trainerName)

	// 检查是否已安装 → 跳过
	if trainerAlreadyInstalled(downloadPath, safeName) {
		return ErrTrainerAlreadyExists
	}

	if len(exeFiles) == 1 {
		return organizeSingleFile(exeFiles[0], tempDir, safeName, trainerName, origin, version, downloadPath)
	}

	return organizeMultiVersion(exeFiles, tempDir, safeName, origin, version, trainerName, downloadPath)
}

// organizeSingleFile 将单个 .exe 文件移动到目标目录并写入 gcm_info.json。
func organizeSingleFile(exeName, tempDir, safeName, trainerName, origin, version, downloadPath string) error {
	targetDir := filepath.Join(downloadPath, safeName)
	err := os.MkdirAll(targetDir, 0o750)
	if err != nil {
		return fmt.Errorf("存储错误: 创建目标目录失败: %w", err)
	}

	src := filepath.Join(tempDir, exeName)
	dst := filepath.Join(targetDir, exeName)
	err = os.Rename(src, dst)
	if err != nil {
		return fmt.Errorf("存储错误: 移动文件失败: %w", err)
	}

	err = writeGcmInfo(targetDir, trainerName, origin, version)
	if err != nil {
		return err
	}

	return cleanupTempDir(tempDir)
}

// organizeMultiVersion 处理多版本修改器的文件组织。
// 对标 Python download_trainers_thread.py:293-310。
func organizeMultiVersion(exeFiles []string, tempDir, safeName, origin, version, trainerName, downloadPath string) error {
	for _, exeName := range exeFiles {
		details := extractVersionDetails(exeName, origin)
		trainerDirName := safeName + details
		targetDir := filepath.Join(downloadPath, trainerDirName)

		err := os.MkdirAll(targetDir, 0o750)
		if err != nil {
			return fmt.Errorf("存储错误: 创建目标目录失败: %w", err)
		}

		src := filepath.Join(tempDir, exeName)
		dst := filepath.Join(targetDir, exeName)
		err = os.Rename(src, dst)
		if err != nil {
			return fmt.Errorf("存储错误: 移动文件失败: %w", err)
		}

		err = writeGcmInfo(targetDir, trainerName, origin, version)
		if err != nil {
			return err
		}
	}

	return cleanupTempDir(tempDir)
}

// extractVersionDetails 从 exe 文件名中提取版本标识。
// fling_main:  正则 `trainer(.*)\.exe` (大小写不敏感)，提取 group(1)
// fling_archive: 正则 `\s+Update.*|\s+v\d+.*`，匹配完整更新后缀
// 对标 Python download_trainers_thread.py:296-304。
func extractVersionDetails(exeName, origin string) string {
	switch origin {
	case originMain:
		if m := reMainVer.FindStringSubmatch(exeName); m != nil {
			return m[1]
		}
	case originArchive:
		if match := reArchiveVer.FindString(exeName); match != "" {
			details := strings.Replace(match, " Trainer", "", 1)
			details = strings.TrimSuffix(details, ".exe")
			return details
		}
	default:
		return ""
	}
	return ""
}

// writeGcmInfo 向目标目录写入 gcm_info.json。
// 对标 Python download_trainers_thread.py:72-85。
func writeGcmInfo(targetDir, trainerName, origin, version string) error {
	gameName := extractGameName(trainerName)

	info := gcmInfoData{
		GameName: gameName,
		Origin:   origin,
		Version:  version,
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("存储错误: 序列化 gcm_info.json 失败: %w", err)
	}

	infoPath := filepath.Join(targetDir, "gcm_info.json")
	//nolint:gosec // 文件权限 0o644 适用于配置文件
	err = os.WriteFile(infoPath, data, 0o644)
	if err != nil {
		return fmt.Errorf("存储错误: 写入 gcm_info.json 失败: %w", err)
	}

	return nil
}

// extractGameName 从展示名中提取纯游戏名（去掉 "[FL] " 前缀和 " Trainer" 后缀）。
// 输入: "[FL] DREDGE Trainer" → 输出: "DREDGE"
func extractGameName(trainerName string) string {
	name := strings.TrimPrefix(trainerName, "[FL] ")
	name = strings.TrimSuffix(name, " Trainer")
	return strings.TrimSpace(name)
}

// cleanupTempDir 清理临时解压目录中的所有文件和子目录。
func cleanupTempDir(tempDir string) error {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("存储错误: 读取临时目录失败: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(tempDir, entry.Name())
		err := os.RemoveAll(path)
		if err != nil {
			return fmt.Errorf("存储错误: 清理临时文件 %s 失败: %w", entry.Name(), err)
		}
	}

	return nil
}
