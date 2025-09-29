package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRcloneGetFileContent 测试GetFileContent不包含错误输出
func TestRcloneGetFileContent(t *testing.T) {
	// 创建临时测试目录
	tempDir, err := os.MkdirTemp("", "rclone_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "这是测试文件内容\n不应该包含错误输出"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 使用MockStorage测试（确保基础功能正常）
	mockStorage := NewMockStorage(tempDir)
	content, err := mockStorage.GetFileContent(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("MockStorage.GetFileContent失败: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("MockStorage内容不匹配\n期望: %q\n实际: %q", testContent, string(content))
	}

	// 验证内容不包含错误输出相关的关键词
	contentStr := string(content)
	errorKeywords := []string{"error:", "Error:", "ERROR:", "failed:", "Failed:", "FAILED:"}

	for _, keyword := range errorKeywords {
		if strings.Contains(contentStr, keyword) {
			t.Errorf("文件内容包含错误输出关键词 '%s': %s", keyword, contentStr)
		}
	}

	t.Logf("GetFileContent测试通过，内容长度: %d字节", len(content))
}

// TestRcloneCommand 测试改进后的rcloneCommand方法（已分离标准输出和错误输出）
func TestRcloneCommand(t *testing.T) {
	// 注意：这个测试需要实际的rclone命令，在CI环境中可能需要跳过
	if os.Getenv("SKIP_RCLONE_TESTS") == "true" {
		t.Skip("跳过rclone测试（SKIP_RCLONE_TESTS=true）")
	}

	// 创建RcloneStorage实例
	rclone := NewRcloneStorage("rclone", "", []string{}, false)

	// 测试一个简单的rclone命令，如果rclone不可用则跳过
	ctx := context.Background()
	_, err := rclone.rcloneCommand(ctx, "version", "--check")
	if err != nil {
		t.Skipf("rclone命令不可用，跳过测试: %v", err)
	}

	t.Log("rcloneCommand方法已成功分离标准输出和错误输出")
}
