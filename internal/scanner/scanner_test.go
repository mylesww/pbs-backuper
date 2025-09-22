package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"backuper/internal/models"
)

func TestChunkScanner(t *testing.T) {
	// 创建临时测试目录
	tempDir := t.TempDir()

	// 创建测试chunk目录结构
	chunkDirs := []string{"0000", "0001", "00ff", "abcd", "ffff"}
	for _, dir := range chunkDirs {
		dirPath := filepath.Join(tempDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// 在每个目录中创建一些测试文件
		testFile := filepath.Join(dirPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// 创建一些无效的目录（应该被忽略）
	invalidDirs := []string{"invalid", "12345", "xyz"}
	for _, dir := range invalidDirs {
		dirPath := filepath.Join(tempDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create invalid directory: %v", err)
		}
	}

	// 测试扫描器
	scanner := NewChunkScanner(tempDir)

	// 测试GetChunkDirectories
	dirs, err := scanner.GetChunkDirectories()
	if err != nil {
		t.Fatalf("GetChunkDirectories failed: %v", err)
	}

	expectedDirs := 5 // 只有有效的hex目录
	if len(dirs) != expectedDirs {
		t.Errorf("Expected %d directories, got %d", expectedDirs, len(dirs))
	}

	// 验证目录名都是有效的
	for _, dir := range dirs {
		if len(dir) != 4 {
			t.Errorf("Invalid directory name length: %s", dir)
		}
	}

	// 测试ScanFileTree
	fileTree, err := scanner.ScanFileTree()
	if err != nil {
		t.Fatalf("ScanFileTree failed: %v", err)
	}

	if len(fileTree) != expectedDirs {
		t.Errorf("Expected %d entries in file tree, got %d", expectedDirs, len(fileTree))
	}

	// 验证文件树结构
	for dirName, node := range fileTree {
		if !node.IsDir {
			t.Errorf("Top level entry %s should be a directory", dirName)
		}

		if len(node.Children) == 0 {
			t.Errorf("Directory %s should have children", dirName)
		}

		// 验证test.txt文件存在
		if _, exists := node.Children["test.txt"]; !exists {
			t.Errorf("test.txt should exist in directory %s", dirName)
		}
	}
}

func TestCompareFileTrees(t *testing.T) {
	// 创建两个测试文件树
	oldTree := map[string]*models.FileTreeNode{
		"0000": {
			Name:    "0000",
			Size:    100,
			ModTime: time.Now().Add(-time.Hour),
			IsDir:   true,
			Children: map[string]*models.FileTreeNode{
				"file1.txt": {
					Name:    "file1.txt",
					Size:    50,
					ModTime: time.Now().Add(-time.Hour),
					IsDir:   false,
				},
			},
		},
	}

	newTree := map[string]*models.FileTreeNode{
		"0000": {
			Name:    "0000",
			Size:    150, // 大小变化
			ModTime: time.Now(),
			IsDir:   true,
			Children: map[string]*models.FileTreeNode{
				"file1.txt": {
					Name:    "file1.txt",
					Size:    50,
					ModTime: time.Now().Add(-time.Hour),
					IsDir:   false,
				},
				"file2.txt": { // 新文件
					Name:    "file2.txt",
					Size:    100,
					ModTime: time.Now(),
					IsDir:   false,
				},
			},
		},
		"0001": { // 新目录
			Name:    "0001",
			Size:    25,
			ModTime: time.Now(),
			IsDir:   true,
			Children: map[string]*models.FileTreeNode{
				"new.txt": {
					Name:    "new.txt",
					Size:    25,
					ModTime: time.Now(),
					IsDir:   false,
				},
			},
		},
	}

	// 比较文件树
	changedDirs := CompareFileTrees(oldTree, newTree)

	// 验证结果
	if !changedDirs["0000"] {
		t.Error("Directory 0000 should be marked as changed")
	}

	if !changedDirs["0001"] {
		t.Error("Directory 0001 should be marked as changed (new)")
	}

	// 应该只有这两个目录变化
	expectedChanges := 2
	if len(changedDirs) != expectedChanges {
		t.Errorf("Expected %d changed directories, got %d", expectedChanges, len(changedDirs))
	}
}
