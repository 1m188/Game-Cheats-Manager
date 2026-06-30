// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
)

// sentinel error：压缩包内未找到 trainer .exe。
var errNoTrainerExe = errors.New("未找到修改器 .exe 文件（可能被杀毒软件拦截）")

// ExtractAndFindTrainer 解压压缩包（或直接 .exe）并定位修改器 .exe 文件。
// archivePath: 压缩包文件路径（.zip / .rar / 直接 .exe）
// destDir: 解压目标目录（不存在则自动创建）
// 返回找到的 .exe 文件名列表（仅文件名，不含路径）。
// 对标 Python download_trainers_thread.py:245-267
func ExtractAndFindTrainer(archivePath, destDir string) ([]string, error) {
	err := os.MkdirAll(destDir, 0o750)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 创建解压目录失败: %w", err)
	}

	format := detectFormat(archivePath)
	switch format {
	case "exe":
		return copyDirectExe(archivePath, destDir)
	case "zip":
		return extractAndScanZip(archivePath, destDir)
	case "rar":
		return extractAndScanRar(archivePath, destDir)
	default:
		// 扩展名作为后备检测
		ext := strings.ToLower(filepath.Ext(archivePath))
		switch ext {
		case ".zip":
			return extractAndScanZip(archivePath, destDir)
		case ".rar":
			return extractAndScanRar(archivePath, destDir)
		default:
			return nil, fmt.Errorf("解析错误: 不支持的压缩包格式: %s", ext)
		}
	}
}

// copyDirectExe 将直接 .exe 文件拷贝到目标目录并返回文件名。
func copyDirectExe(exePath, destDir string) ([]string, error) {
	name := filepath.Base(exePath)
	destPath := filepath.Join(destDir, name)

	src, err := os.Open(exePath)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 打开 exe 文件失败: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(destPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("存储错误: 创建目标文件失败: %w", err)
	}
	defer func() { _ = dst.Close() }()

	_, err = io.Copy(dst, src)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 拷贝 exe 文件失败: %w", err)
	}

	// 验证文件名包含 "trainer" 关键字
	lower := strings.ToLower(name)
	if !strings.Contains(lower, "trainer") {
		return nil, errNoTrainerExe
	}

	return []string{name}, nil
}

// DetectFormat 通过魔术字节检测文件格式。
// 返回 "zip"、"rar"、"exe" 或 ""（未知）。
func DetectFormat(path string) string {
	return detectFormat(path)
}

// detectFormat 通过读取文件头部魔术字节检测文件格式。
// ZIP 以 "PK\x03\x04" 开头，RAR 以 "Rar!\x1a\x07" 开头，PE exe 以 "MZ" 开头。
func detectFormat(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return ""
	}

	// MZ → Windows PE executable (.exe)
	if buf[0] == 0x4D && buf[1] == 0x5A {
		return "exe"
	}
	// PK → ZIP
	if buf[0] == 0x50 && buf[1] == 0x4B {
		return "zip"
	}
	// Rar! → RAR
	if n >= 7 && buf[0] == 0x52 && buf[1] == 0x61 && buf[2] == 0x72 && buf[3] == 0x21 {
		return "rar"
	}

	return ""
}

// extractAndScanZip 解压 .zip 文件并返回 trainer .exe 文件名列表。
func extractAndScanZip(zipPath, destDir string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("解析错误: 打开 zip 文件失败: %w", err)
	}
	defer func() { _ = r.Close() }() //nolint:errcheck // 读取操作 Close 错误可忽略

	for _, f := range r.File {
		err := extractZipEntry(f, destDir)
		if err != nil {
			return nil, fmt.Errorf("解析错误: 解压 zip 条目 '%s' 失败: %w", f.Name, err)
		}
	}

	return findTrainerExes(destDir)
}

// extractZipEntry 解压单个 zip 条目到 destDir。
// 使用 filepath.Base 防御路径遍历攻击。
func extractZipEntry(f *zip.File, destDir string) error {
	// 跳过目录条目（名称以 / 结尾）
	if f.FileInfo().IsDir() {
		return nil
	}

	// 防御路径遍历：仅使用文件名，忽略目录前缀
	name := filepath.Base(f.Name)
	if name == "." || name == string(filepath.Separator) {
		return nil
	}

	destPath := filepath.Join(destDir, name)

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("解析错误: 打开 zip 条目失败: %w", err)
	}
	defer func() { _ = rc.Close() }() //nolint:errcheck // 读取操作 Close 错误可忽略

	//nolint:gosec // destPath 已通过 filepath.Base 安全化
	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("存储错误: 创建解压文件失败: %w", err)
	}
	defer func() { _ = outFile.Close() }() //nolint:errcheck // 写入后 Close 错误可忽略

	//nolint:gosec // 压缩包来自 FLiNG 站点，体积可控，无解压炸弹风险
	_, err = io.Copy(outFile, rc)
	if err != nil {
		return fmt.Errorf("存储错误: 写入解压数据失败: %w", err)
	}

	return nil
}

// extractAndScanRar 解压 .rar 文件并返回 trainer .exe 文件名列表。
// 对标 Python download_trainers_thread.py:248-249（原版使用外部 unrar 命令）。
func extractAndScanRar(rarPath, destDir string) ([]string, error) {
	rr, err := rardecode.OpenReader(rarPath, "")
	if err != nil {
		return nil, fmt.Errorf("解析错误: 打开 rar 文件失败: %w", err)
	}
	defer func() { _ = rr.Close() }() //nolint:errcheck // 读取操作 Close 错误可忽略

	for {
		header, err := rr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// 跳过损坏条目，继续处理其余文件
			continue
		}

		err = extractRarEntry(&rr.Reader, header, destDir)
		if err != nil {
			// 跳过单个条目解压失败，继续处理其余文件
			continue
		}
	}

	return findTrainerExes(destDir)
}

// extractRarEntry 解压单个 rar 条目到 destDir。
func extractRarEntry(rr *rardecode.Reader, header *rardecode.FileHeader, destDir string) error {
	if header.IsDir {
		return nil
	}

	name := filepath.Base(header.Name)
	if name == "." || name == string(filepath.Separator) {
		return nil
	}

	destPath := filepath.Join(destDir, name)

	//nolint:gosec // destPath 已通过 filepath.Base 安全化
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("存储错误: 创建解压文件失败: %w", err)
	}
	defer func() { _ = outFile.Close() }() //nolint:errcheck // 写入后 Close 错误可忽略

	_, err = io.Copy(outFile, rr)
	if err != nil {
		return fmt.Errorf("存储错误: 写入解压数据失败: %w", err)
	}

	return nil
}

// findTrainerExes 扫描 destDir 目录，返回文件名包含 "trainer"（大小写不敏感）
// 且以 ".exe" 结尾的文件名列表。对标 Python download_trainers_thread.py:258-266。
func findTrainerExes(destDir string) ([]string, error) {
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return nil, fmt.Errorf("存储错误: 读取解压目录失败: %w", err)
	}

	var exes []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if strings.Contains(lower, "trainer") && strings.HasSuffix(lower, ".exe") {
			exes = append(exes, name)
		}
	}

	if len(exes) == 0 {
		return nil, errNoTrainerExe
	}

	return exes, nil
}
