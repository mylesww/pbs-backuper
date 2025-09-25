package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RcloneStorage rclone存储实现
type RcloneStorage struct {
	binary     string   // rclone二进制路径
	configFile string   // rclone配置文件路径
	extraArgs  []string // 额外参数
}

// NewRcloneStorage 创建rclone存储实例
func NewRcloneStorage(binary, configFile string, extraArgs []string) *RcloneStorage {
	return &RcloneStorage{
		binary:     binary,
		configFile: configFile,
		extraArgs:  extraArgs,
	}
}

// rcloneCommand 执行rclone命令的通用方法，分离标准输出和错误输出
func (r *RcloneStorage) rcloneCommand(ctx context.Context, args ...string) ([]byte, error) {
	// 构建基础命令参数
	cmdArgs := []string{}

	// 添加配置文件参数
	if r.configFile != "" {
		cmdArgs = append(cmdArgs, "--config", r.configFile)
	}

	// 添加自定义参数
	cmdArgs = append(cmdArgs, r.extraArgs...)

	// 添加命令特定参数
	cmdArgs = append(cmdArgs, args...)

	// 添加必须参数
	cmdArgs = append(cmdArgs, "--quiet")
	cmdArgs = append(cmdArgs, "--progress=false")

	cmd := exec.CommandContext(ctx, r.binary, cmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		// 使用我们捕获的stderr
		return stdout.Bytes(), fmt.Errorf("rclone command failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// ListFiles 实现Storage接口 - 列出文件
func (r *RcloneStorage) ListFiles(ctx context.Context, remotePath string) ([]FileInfo, error) {
	// 使用rclone lsjson命令获取文件列表
	output, err := r.rcloneCommand(ctx, "lsjson", remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var jsonFiles []struct {
		Path    string    `json:"Path"`
		Name    string    `json:"Name"`
		Size    int64     `json:"Size"`
		ModTime time.Time `json:"ModTime"`
		IsDir   bool      `json:"IsDir"`
	}

	if err := json.Unmarshal(output, &jsonFiles); err != nil {
		return nil, fmt.Errorf("failed to parse rclone output: %w", err)
	}

	files := make([]FileInfo, len(jsonFiles))
	for i, f := range jsonFiles {
		files[i] = FileInfo{
			Name:    f.Name,
			Size:    f.Size,
			ModTime: f.ModTime,
			IsDir:   f.IsDir,
		}
	}

	return files, nil
}

// DownloadFile 实现Storage接口 - 下载文件
func (r *RcloneStorage) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	_, err := r.rcloneCommand(ctx, "copyto", remotePath, filepath.Dir(localPath))
	if err != nil {
		return fmt.Errorf("failed to download file %s to %s: %w", remotePath, localPath, err)
	}
	return nil
}

// UploadFile 实现Storage接口 - 上传文件
func (r *RcloneStorage) UploadFile(ctx context.Context, localPath, remotePath string) error {
	_, err := r.rcloneCommand(ctx, "copyto", localPath, remotePath)
	// fmt.Println("UploadFile", localPath, remotePath, err)
	if err != nil {
		return fmt.Errorf("failed to upload file %s to %s: %w", localPath, remotePath, err)
	}
	return nil
}

// FileExists 实现Storage接口 - 检查文件是否存在
func (r *RcloneStorage) FileExists(ctx context.Context, remotePath string) (bool, error) {
	// 使用rclone lsf命令检查文件是否存在
	output, err := r.rcloneCommand(ctx, "lsf", remotePath)
	if err != nil {
		// 如果是文件不存在的错误，返回false
		if strings.Contains(string(output), "not found") || strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	// 如果有输出，说明文件存在
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// GetFileContent 实现Storage接口 - 获取文件内容
func (r *RcloneStorage) GetFileContent(ctx context.Context, remotePath string) ([]byte, error) {
	// 使用rclone cat命令获取文件内容，现在rcloneCommand已经分离了标准输出和错误输出
	output, err := r.rcloneCommand(ctx, "cat", remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	return output, nil
}
