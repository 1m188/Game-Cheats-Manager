// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 测试用的固定文件名常量
const (
	testFileNameExe      = "DREDGE Trainer.exe"
	testFileNameReadme   = "readme.txt"
	testMsgAntivirusHint = "杀毒"
)

// createTestZip 在 zipPath 创建一个测试用 .zip 文件。
// entries 是 map[文件名]内容，用于控制压缩包内包含的文件。
func createTestZip(zipPath string, entries map[string]string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, body := range entries {
		header := &zip.FileHeader{
			Name:   name,
			Method: zip.Deflate,
		}
		header.SetMode(0o644)
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			return err
		}
	}
	return w.Close()
}

// TestExtractAndFindTrainer 测试解压压缩包并定位修改器 .exe 文件。
func TestExtractAndFindTrainer(t *testing.T) {
	t.Parallel()

	t.Run("含trainer_exe正常解压并返回", func(t *testing.T) {
		// Given: 一个包含 "DREDGE Trainer.exe" 和 "readme.txt" 的 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "test.zip")

		entries := map[string]string{
			testFileNameExe:    "mock exe content",
			testFileNameReadme: "just a readme",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压并查找 trainer .exe
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 应返回仅包含 .exe 文件的列表，不含 readme.txt
		if err != nil {
			t.Fatalf("预期成功，但返回错误: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
		}
		if got[0] != testFileNameExe {
			t.Errorf("预期 '%s'，得到 '%s'", testFileNameExe, got[0])
		}

		// 验证文件确实解压到目标目录
		if _, statErr := os.Stat(filepath.Join(destDir, testFileNameExe)); os.IsNotExist(statErr) {
			t.Errorf("预期 '%s' 存在于 %s，但不存在", testFileNameExe, destDir)
		}
	})

	t.Run("无trainer_exe返回错误", func(t *testing.T) {
		// Given: 一个包含 .txt 和 .dll 但没有 .exe 的 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "noexe.zip")

		entries := map[string]string{
			testFileNameReadme: "just a readme",
			"somefile.dll":     "library",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压并查找 trainer .exe
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 应返回包含"杀毒软件"关键词的错误
		if err == nil {
			t.Fatalf("预期错误，但成功返回: %v", got)
		}
		if err.Error() == "" {
			t.Fatal("错误信息不应为空")
		}
		// 对标 Python download_trainers_thread.py:277 错误信息
		if !strings.Contains(err.Error(), testMsgAntivirusHint) {
			t.Errorf("错误信息应包含 '%s'，实际: '%s'", testMsgAntivirusHint, err.Error())
		}
	})

	t.Run("zip内有exe但无trainer关键字仍返回错误", func(t *testing.T) {
		// Given: 一个包含"setup.exe"但不含"trainer"关键字的 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "setupexe.zip")

		entries := map[string]string{
			"setup.exe":        "setup program",
			testFileNameReadme: "docs",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压并查找 trainer .exe
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: .exe 存在但文件名不包含 "trainer"，应返回错误并提示杀毒软件
		if err == nil {
			t.Fatalf("预期错误，但成功返回: %v", got)
		}
		if !strings.Contains(err.Error(), testMsgAntivirusHint) {
			t.Errorf("错误信息应包含 '%s'，实际: '%s'", testMsgAntivirusHint, err.Error())
		}
	})

	t.Run("空压缩包返回错误", func(t *testing.T) {
		// Given: 一个没有任何条目的空 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "empty.zip")

		entries := map[string]string{} // 空
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压空压缩包
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 应返回包含"杀毒"关键词的错误
		if err == nil {
			t.Fatalf("预期错误，但成功返回: %v", got)
		}
		if !strings.Contains(err.Error(), testMsgAntivirusHint) {
			t.Errorf("错误信息应包含 '%s'，实际: '%s'", testMsgAntivirusHint, err.Error())
		}
	})

	t.Run("trainer关键字大小写不敏感", func(t *testing.T) {
		// Given: 一个包含 "GAMENAME TRAINER.EXE"（大写）的 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "uppercase.zip")

		entries := map[string]string{
			"GAMENAME TRAINER.EXE": "mock exe",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压并查找 trainer .exe
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 大小写不敏感匹配应成功
		if err != nil {
			t.Fatalf("预期成功，但返回错误: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
		}
		if got[0] != "GAMENAME TRAINER.EXE" {
			t.Errorf("预期 'GAMENAME TRAINER.EXE'，得到 '%s'", got[0])
		}
	})

	t.Run("多个trainer_exe文件全部返回", func(t *testing.T) {
		// Given: 一个包含多个 trainer .exe 的 .zip 文件
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "multi.zip")

		entries := map[string]string{
			testFileNameExe:             "mock 1",
			"DREDGE v1.0.2 Trainer.exe": "mock 2",
			testFileNameReadme:          "docs",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "extracted")

		// When: 解压并查找 trainer .exe
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 两个 trainer .exe 都应返回，readme.txt 不应返回
		if err != nil {
			t.Fatalf("预期成功，但返回错误: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("预期 2 个 trainer 文件，得到 %d: %v", len(got), got)
		}
	})

	t.Run("目标目录不存在时自动创建", func(t *testing.T) {
		// Given: 目标目录的子路径不存在
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "test.zip")

		entries := map[string]string{
			"Game Trainer.exe": "mock",
		}
		if err := createTestZip(zipPath, entries); err != nil {
			t.Fatalf("创建测试 zip 失败: %v", err)
		}

		destDir := filepath.Join(tmpDir, "nested", "extracted")

		// When: 解压到尚不存在的目录
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 应自动创建目录并成功解压
		if err != nil {
			t.Fatalf("预期成功，但返回错误: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("预期 1 个 trainer 文件，得到 %d: %v", len(got), got)
		}
	})

	t.Run("不存在的zip文件返回错误", func(t *testing.T) {
		// Given: 一个不存在的 .zip 路径
		tmpDir := t.TempDir()
		zipPath := filepath.Join(tmpDir, "does_not_exist.zip")
		destDir := filepath.Join(tmpDir, "extracted")

		// When: 尝试解压不存在的文件
		got, err := ExtractAndFindTrainer(zipPath, destDir)

		// Then: 应返回错误
		if err == nil {
			t.Fatalf("预期错误，但成功返回: %v", got)
		}
	})
}
