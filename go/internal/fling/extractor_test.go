// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 测试用的固定文件名常量
const (
	testFileNameExe      = "DREDGE Trainer.exe"
	testFileNameReadme   = "readme.txt"
	testBodyReadme       = "just a readme"
	testMsgAntivirusHint = "杀毒"
)

// createTestZip 在 zipPath 创建一个测试用 .zip 文件。
// entries 是 map[文件名]内容，用于控制压缩包内包含的文件。
func createTestZip(zipPath string, entries map[string]string) error {
	f, err := os.Create(zipPath) //nolint:gosec // 测试辅助函数，路径由测试框架控制
	if err != nil {
		return fmt.Errorf("创建测试 zip 文件失败: %w", err)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer Close in test helper

	w := zip.NewWriter(f)
	for name, body := range entries {
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		header.SetMode(0o644)
		writer, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("创建 zip 条目失败: %w", err)
		}
		_, writeErr := writer.Write([]byte(body))
		if writeErr != nil {
			return fmt.Errorf("写入 zip 条目内容失败: %w", writeErr)
		}
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("关闭 zip writer 失败: %w", err)
	}
	return nil
}

// extractTestHelper 创建测试 zip 并调用 ExtractAndFindTrainer，返回结果和错误。
// entries 为空时创建空 zip 文件。
func extractTestHelper(t *testing.T, entries map[string]string) ([]string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	err := createTestZip(zipPath, entries)
	fatalIfErr(t, "创建测试 zip 失败", err)

	destDir := filepath.Join(tmpDir, "extracted")
	return ExtractAndFindTrainer(zipPath, destDir)
}

// extractTestToDir 创建测试 zip 并解压到指定 destDir。
// entries 为空时创建空 zip 文件。
func extractTestToDir(t *testing.T, entries map[string]string, destDir string) ([]string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	err := createTestZip(zipPath, entries)
	fatalIfErr(t, "创建测试 zip 失败", err)

	return ExtractAndFindTrainer(zipPath, destDir)
}

// fatalIfErr 是 t.Fatalf 的简洁包装。
func fatalIfErr(t *testing.T, context string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", context, err)
	}
}

// requireError 要求 err 非 nil 且包含特定关键词。
func requireError(t *testing.T, got []string, err error, keyword string) {
	t.Helper()
	if err == nil {
		t.Fatalf("预期错误，但成功返回: %v", got)
	}
	if !strings.Contains(err.Error(), keyword) {
		t.Errorf("错误信息应包含 '%s'，实际: '%s'", keyword, err.Error())
	}
}

// TestExtractAndFindTrainer_withTrainerExe 测试含 trainer .exe 的正常解压。
func TestExtractAndFindTrainer_withTrainerExe(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		testFileNameExe:    "mock exe content",
		testFileNameReadme: testBodyReadme,
	}
	got, err := extractTestHelper(t, entries)

	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
	}
	if got[0] != testFileNameExe {
		t.Errorf("预期 '%s'，得到 '%s'", testFileNameExe, got[0])
	}
}

// TestExtractAndFindTrainer_noTrainerExe 测试无 trainer .exe 返回错误。
func TestExtractAndFindTrainer_noTrainerExe(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		testFileNameReadme: testBodyReadme,
		"somefile.dll":     "library",
	}
	got, err := extractTestHelper(t, entries)
	requireError(t, got, err, testMsgAntivirusHint)
}

// TestExtractAndFindTrainer_exeButNoTrainer 测试 .exe 存在但无 trainer 关键字。
func TestExtractAndFindTrainer_exeButNoTrainer(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		"setup.exe":        "setup program",
		testFileNameReadme: "docs",
	}
	got, err := extractTestHelper(t, entries)
	requireError(t, got, err, testMsgAntivirusHint)
}

// TestExtractAndFindTrainer_emptyArchive 测试空压缩包返回错误。
func TestExtractAndFindTrainer_emptyArchive(t *testing.T) {
	t.Parallel()

	got, err := extractTestHelper(t, map[string]string{})
	requireError(t, got, err, testMsgAntivirusHint)
}

// TestExtractAndFindTrainer_caseInsensitive 测试 trainer 关键字大小写不敏感。
func TestExtractAndFindTrainer_caseInsensitive(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		"GAMENAME TRAINER.EXE": "mock exe",
	}
	got, err := extractTestHelper(t, entries)

	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
	}
	if got[0] != "GAMENAME TRAINER.EXE" {
		t.Errorf("预期 'GAMENAME TRAINER.EXE'，得到 '%s'", got[0])
	}
}

// TestExtractAndFindTrainer_multipleExes 测试多个 trainer .exe 全部返回。
func TestExtractAndFindTrainer_multipleExes(t *testing.T) {
	t.Parallel()

	entries := map[string]string{
		testFileNameExe:             "mock 1",
		"DREDGE v1.0.2 Trainer.exe": "mock 2",
		testFileNameReadme:          "docs",
	}
	got, err := extractTestHelper(t, entries)

	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("预期 2 个 trainer 文件，得到 %d: %v", len(got), got)
	}
}

// TestExtractAndFindTrainer_autoCreateDir 测试目标目录不存在时自动创建。
func TestExtractAndFindTrainer_autoCreateDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	entries := map[string]string{"Game Trainer.exe": "mock"}
	destDir := filepath.Join(tmpDir, "nested", "extracted")

	got, err := extractTestToDir(t, entries, destDir)

	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
	}
}

// TestExtractAndFindTrainer_nonexistentZip 测试不存在的 zip 文件返回错误。
func TestExtractAndFindTrainer_nonexistentZip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "does_not_exist.zip")
	destDir := filepath.Join(tmpDir, "extracted")

	got, err := ExtractAndFindTrainer(zipPath, destDir)

	if err == nil {
		t.Fatalf("预期错误，但成功返回: %v", got)
	}
}

// TestExtractAndFindTrainer_fileStatAfterExtract 验证文件解压到目标目录后确实存在。
func TestExtractAndFindTrainer_fileStatAfterExtract(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "test.zip")
	entries := map[string]string{
		testFileNameExe:    "mock exe content",
		testFileNameReadme: testBodyReadme,
	}
	err := createTestZip(zipPath, entries)
	fatalIfErr(t, "创建测试 zip 失败", err)

	destDir := filepath.Join(tmpDir, "extracted")
	_, err = ExtractAndFindTrainer(zipPath, destDir)
	fatalIfErr(t, "解压失败", err)

	_, statErr := os.Stat(filepath.Join(destDir, testFileNameExe))
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在于 %s，但不存在", testFileNameExe, destDir)
	}
}

// createDirectExe 创建一个以 MZ 头开头的模拟 .exe 文件。
func createDirectExe(path string, content string) error {
	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	// MZ magic + content
	_, err = f.Write([]byte{0x4D, 0x5A, 0x90, 0x00})
	if err != nil {
		return err
	}
	_, err = f.Write([]byte(content))
	return err
}

// TestDetectFormat_zipMagic 测试 ZIP 魔术字节检测。
func TestDetectFormat_zipMagic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "test.zip")
	entries := map[string]string{"a.txt": "hello"}
	fatalIfErr(t, "创建测试 zip", createTestZip(p, entries))

	got := DetectFormat(p)
	if got != "zip" {
		t.Errorf("DetectFormat(zip) = %q, want \"zip\"", got)
	}
}

// TestDetectFormat_exeDirect 测试直接 .exe 文件（MZ 头）检测。
func TestDetectFormat_exeDirect(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "trainer.exe")
	err := os.WriteFile(p, []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00}, 0o644)
	fatalIfErr(t, "创建 exe 文件", err)

	got := DetectFormat(p)
	if got != "exe" {
		t.Errorf("DetectFormat(exe) = %q, want \"exe\"", got)
	}
}

// TestDetectFormat_unknown 测试未知格式。
func TestDetectFormat_unknown(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "unknown.bin")
	err := os.WriteFile(p, []byte("Hello World!"), 0o644)
	fatalIfErr(t, "创建未知文件", err)

	got := DetectFormat(p)
	if got != "" {
		t.Errorf("DetectFormat(unknown) = %q, want \"\"", got)
	}
}

// TestExtractAndFindTrainer_directExe 测试直接 .exe 文件（非压缩包）被正确识别为 trainer。
func TestExtractAndFindTrainer_directExe(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	exePath := filepath.Join(tmpDir, "DREDGE Trainer.exe")
	err := createDirectExe(exePath, "pretend this is a PE executable")
	fatalIfErr(t, "创建直接 exe 文件", err)

	destDir := filepath.Join(tmpDir, "extracted")
	got, err := ExtractAndFindTrainer(exePath, destDir)

	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
	}
	// 验证文件确实被拷贝到目标目录
	copiedPath := filepath.Join(destDir, "DREDGE Trainer.exe")
	_, statErr := os.Stat(copiedPath)
	if os.IsNotExist(statErr) {
		t.Errorf("直接 exe 应被拷贝到 %s，但不存在", copiedPath)
	}
}
