// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gcmInfo 表示 gcm_info.json 的结构，仅在此测试文件中使用。
type gcmInfo struct {
	GameName string `json:"game_name"`
	Origin   string `json:"origin"`
	Version  string `json:"version,omitempty"`
}

const testGameExeName = "Game Trainer.exe"

// createTestExe 在指定目录创建一个空的 .exe 文件用于测试。
//
//nolint:wrapcheck // 测试辅助函数，错误直接返回供测试检查
func createTestExe(dir, name string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte("mock exe"), 0o600)
}

// TestOrganizeTrainer_SingleExe 测试单文件直接移动到目标目录。
func TestOrganizeTrainer_SingleExe(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	exeName := "DREDGE Trainer.exe"
	err = createTestExe(tempExtractDir, exeName)
	if err != nil {
		t.Fatalf("创建测试 exe 失败: %v", err)
	}

	err = OrganizeTrainer([]string{exeName}, tempExtractDir,
		testFLDREDGETrainer, originMain, testVersion, downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	safeName := SymbolReplacement(testFLDREDGETrainer)
	targetDir := filepath.Join(downloadPath, safeName)
	expectedExePath := filepath.Join(targetDir, exeName)
	_, statErr := os.Stat(expectedExePath)
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在，但不存在", expectedExePath)
	}

	infoPath := filepath.Join(targetDir, "gcm_info.json")
	//nolint:gosec // 测试中读取已知路径的文件
	data, readErr := os.ReadFile(infoPath)
	if readErr != nil {
		t.Fatalf("读取 gcm_info.json 失败: %v", readErr)
	}

	var info gcmInfo
	err = json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("解析 gcm_info.json 失败: %v", err)
	}
	if info.GameName != "DREDGE" {
		t.Errorf("game_name 预期 'DREDGE'，得到 '%s'", info.GameName)
	}
	if info.Origin != originMain {
		t.Errorf("origin 预期 '%s'，得到 '%s'", originMain, info.Origin)
	}
	if info.Version != testVersion {
		t.Errorf("version 预期 '%s'，得到 '%s'", testVersion, info.Version)
	}
}

// TestOrganizeTrainer_MultiVersionMain 测试 fling_main 来源多版本拆分。
func TestOrganizeTrainer_MultiVersionMain(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	exeA := "DREDGEtrainerv1.0.2.exe"
	exeB := "DREDGEtrainerv1.0.3.exe"
	for _, n := range []string{exeA, exeB} {
		err = createTestExe(tempExtractDir, n)
		if err != nil {
			t.Fatalf("创建测试 exe 失败: %v", err)
		}
	}

	err = OrganizeTrainer([]string{exeA, exeB}, tempExtractDir,
		testFLDREDGETrainer, originMain, testVersion, downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	safeName := SymbolReplacement(testFLDREDGETrainer)

	expectedDirA := filepath.Join(downloadPath, safeName+"v1.0.2")
	expectedExeA := filepath.Join(expectedDirA, exeA)
	_, statErr := os.Stat(expectedExeA)
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在，但不存在", expectedExeA)
	}

	expectedDirB := filepath.Join(downloadPath, safeName+"v1.0.3")
	expectedExeB := filepath.Join(expectedDirB, exeB)
	_, statErr = os.Stat(expectedExeB)
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在，但不存在", expectedExeB)
	}

	for _, d := range []string{expectedDirA, expectedDirB} {
		infoPath := filepath.Join(d, "gcm_info.json")
		_, statErr = os.Stat(infoPath)
		if os.IsNotExist(statErr) {
			t.Errorf("预期 '%s' 存在 gcm_info.json，但不存在", d)
		}
	}
}

// TestOrganizeTrainer_MultiVersionArchive 测试 fling_archive 来源多版本拆分。
func TestOrganizeTrainer_MultiVersionArchive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	exeA := "DREDGE Trainer v1.0.2.exe"
	exeB := "DREDGE Trainer Update 3.exe"
	for _, n := range []string{exeA, exeB} {
		err = createTestExe(tempExtractDir, n)
		if err != nil {
			t.Fatalf("创建测试 exe 失败: %v", err)
		}
	}

	err = OrganizeTrainer([]string{exeA, exeB}, tempExtractDir,
		testFLDREDGETrainer, originArchive, "", downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	safeName := SymbolReplacement(testFLDREDGETrainer)

	expectedDirA := filepath.Join(downloadPath, safeName+" v1.0.2")
	expectedExeA := filepath.Join(expectedDirA, exeA)
	_, statErr := os.Stat(expectedExeA)
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在，但不存在", expectedExeA)
	}

	expectedDirB := filepath.Join(downloadPath, safeName+" Update 3")
	expectedExeB := filepath.Join(expectedDirB, exeB)
	_, statErr = os.Stat(expectedExeB)
	if os.IsNotExist(statErr) {
		t.Errorf("预期 '%s' 存在，但不存在", expectedExeB)
	}
}

// TestOrganizeTrainer_GcmInfoFields 测试 gcm_info.json 字段内容。
func TestOrganizeTrainer_GcmInfoFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	err = createTestExe(tempExtractDir, testGameExeName)
	if err != nil {
		t.Fatalf("创建测试 exe 失败: %v", err)
	}

	err = OrganizeTrainer([]string{testGameExeName}, tempExtractDir,
		"[FL] My Game Trainer", originArchive, "2025.01.15", downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	safeName := SymbolReplacement("[FL] My Game Trainer")
	infoPath := filepath.Join(downloadPath, safeName, "gcm_info.json")
	//nolint:gosec // 测试中读取已知路径的文件
	data, readErr := os.ReadFile(infoPath)
	if readErr != nil {
		t.Fatalf("读取 gcm_info.json 失败: %v", readErr)
	}

	var info gcmInfo
	err = json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("解析 gcm_info.json 失败: %v", err)
	}

	if info.GameName != "My Game" {
		t.Errorf("game_name 预期 'My Game'，得到 '%s'", info.GameName)
	}
	if info.Origin != originArchive {
		t.Errorf("origin 预期 '%s'，得到 '%s'", originArchive, info.Origin)
	}
	if info.Version != "2025.01.15" {
		t.Errorf("version 预期 '2025.01.15'，得到 '%s'", info.Version)
	}
}

// TestOrganizeTrainer_EmptyVersion 测试版本号为空时不写入 version 字段。
func TestOrganizeTrainer_EmptyVersion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	err = createTestExe(tempExtractDir, testGameExeName)
	if err != nil {
		t.Fatalf("创建测试 exe 失败: %v", err)
	}

	err = OrganizeTrainer([]string{testGameExeName}, tempExtractDir,
		"[FL] My Game Trainer", originArchive, "", downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	safeName := SymbolReplacement("[FL] My Game Trainer")
	infoPath := filepath.Join(downloadPath, safeName, "gcm_info.json")
	//nolint:gosec // 测试中读取已知路径的文件
	data, readErr := os.ReadFile(infoPath)
	if readErr != nil {
		t.Fatalf("读取 gcm_info.json 失败: %v", readErr)
	}

	var info gcmInfo
	err = json.Unmarshal(data, &info)
	if err != nil {
		t.Fatalf("解析 gcm_info.json 失败: %v", err)
	}
	if info.Version != "" {
		t.Errorf("version 预期为空，得到 '%s'", info.Version)
	}

	if strings.Contains(string(data), `"version"`) {
		t.Errorf("version 为空时不应包含在 JSON 中，但 JSON 包含 'version': %s", string(data))
	}
}

// TestOrganizeTrainer_CleanupTemp 测试临时解压目录残留文件被清理。
func TestOrganizeTrainer_CleanupTemp(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	err = createTestExe(tempExtractDir, "DREDGE Trainer.exe")
	if err != nil {
		t.Fatalf("创建测试 exe 失败: %v", err)
	}
	err = createTestExe(tempExtractDir, "readme.txt")
	if err != nil {
		t.Fatalf("创建测试 txt 失败: %v", err)
	}

	err = OrganizeTrainer([]string{"DREDGE Trainer.exe"}, tempExtractDir,
		testFLDREDGETrainer, originMain, testVersion, downloadPath)
	if err != nil {
		t.Fatalf("预期成功，但返回错误: %v", err)
	}

	entries, readErr := os.ReadDir(tempExtractDir)
	if readErr != nil {
		t.Fatalf("读取解压目录失败: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("预期解压目录已清空，但仍有 %d 个条目: %v", len(entries), entries)
	}
}

// TestOrganizeTrainer_EmptyExeList 测试空 exe 列表返回错误。
func TestOrganizeTrainer_EmptyExeList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")

	err := OrganizeTrainer(nil, tempExtractDir,
		testFLDREDGETrainer, originMain, "", downloadPath)
	if err == nil {
		t.Fatal("预期错误，但成功返回")
	}
}

// TestOrganizeTrainer_alreadyExists 测试重复下载已存在的修改器返回跳过错误。
func TestOrganizeTrainer_alreadyExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	downloadPath := filepath.Join(tmpDir, "trainers")
	tempExtractDir := filepath.Join(tmpDir, "extract")
	err := os.MkdirAll(tempExtractDir, 0o750)
	if err != nil {
		t.Fatalf("创建临时解压目录失败: %v", err)
	}

	exeName := "DREDGE Trainer.exe"
	err = createTestExe(tempExtractDir, exeName)
	if err != nil {
		t.Fatalf("创建测试 exe 失败: %v", err)
	}

	// 第一次下载：成功
	err = OrganizeTrainer([]string{exeName}, tempExtractDir,
		testFLDREDGETrainer, originMain, testVersion, downloadPath)
	if err != nil {
		t.Fatalf("首次下载预期成功，但返回错误: %v", err)
	}

	// 第二次下载同一修改器：应返回 ErrTrainerAlreadyExists
	err = OrganizeTrainer([]string{exeName}, tempExtractDir,
		testFLDREDGETrainer, originMain, testVersion, downloadPath)
	if !errors.Is(err, ErrTrainerAlreadyExists) {
		t.Errorf("重复下载预期 ErrTrainerAlreadyExists，得到: %v", err)
	}
}
