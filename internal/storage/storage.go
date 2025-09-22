package storage

import (
	"context"
	"time"
)

// FileInfo 远程文件信息
type FileInfo struct {
	Name    string    // 文件名
	Size    int64     // 文件大小
	ModTime time.Time // 修改时间
	IsDir   bool      // 是否为目录
}

// Storage 存储接口，抽象化云端存储操作
type Storage interface {
	// ListFiles 列出指定路径下的文件
	ListFiles(ctx context.Context, remotePath string) ([]FileInfo, error)

	// DownloadFile 下载文件到本地
	DownloadFile(ctx context.Context, remotePath, localPath string) error

	// UploadFile 上传本地文件到远程
	UploadFile(ctx context.Context, localPath, remotePath string) error

	// FileExists 检查远程文件是否存在
	FileExists(ctx context.Context, remotePath string) (bool, error)

	// GetFileContent 获取远程文件内容（小文件）
	GetFileContent(ctx context.Context, remotePath string) ([]byte, error)
}
