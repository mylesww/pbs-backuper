package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// MockStorage 模拟存储实现，用于测试
type MockStorage struct {
	remoteDir string // 模拟远程存储的本地目录
}

// NewMockStorage 创建模拟存储实例
func NewMockStorage(remoteDir string) *MockStorage {
	// 确保远程目录存在
	os.MkdirAll(remoteDir, 0755)
	return &MockStorage{
		remoteDir: remoteDir,
	}
}

// ListFiles 实现Storage接口 - 列出文件
func (m *MockStorage) ListFiles(ctx context.Context, remotePath string) ([]FileInfo, error) {
	fullPath := filepath.Join(m.remoteDir, remotePath)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []FileInfo{}, nil
		}
		return nil, err
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return files, nil
}

// DownloadFile 实现Storage接口 - 下载文件
func (m *MockStorage) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	srcPath := filepath.Join(m.remoteDir, remotePath)

	// 确保本地目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	// 复制文件
	return m.copyFile(srcPath, localPath)
}

// UploadFile 实现Storage接口 - 上传文件
func (m *MockStorage) UploadFile(ctx context.Context, localPath, remotePath string) error {
	dstPath := filepath.Join(m.remoteDir, remotePath)

	// 确保远程目录存在
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	// 复制文件
	return m.copyFile(localPath, dstPath)
}

// FileExists 实现Storage接口 - 检查文件是否存在
func (m *MockStorage) FileExists(ctx context.Context, remotePath string) (bool, error) {
	fullPath := filepath.Join(m.remoteDir, remotePath)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFileContent 实现Storage接口 - 获取文件内容
func (m *MockStorage) GetFileContent(ctx context.Context, remotePath string) ([]byte, error) {
	fullPath := filepath.Join(m.remoteDir, remotePath)
	return os.ReadFile(fullPath)
}

// copyFile 复制文件的辅助函数
func (m *MockStorage) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// GetRemotePath 获取远程存储的实际路径（用于测试验证）
func (m *MockStorage) GetRemotePath() string {
	return m.remoteDir
}
