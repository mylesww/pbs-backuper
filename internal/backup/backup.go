package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pbs-backuper/internal/archiver"
	"pbs-backuper/internal/models"
	"pbs-backuper/internal/scanner"
	"pbs-backuper/internal/storage"
)

const (
	MetadataFileName = "backup-metadata.json"
	MetadataVersion  = 1
)

// BackupManager 备份管理器
type BackupManager struct {
	config   *models.Config
	storage  storage.Storage
	scanner  *scanner.ChunkScanner
	archiver *archiver.Archiver
}

// NewBackupManager 创建备份管理器
func NewBackupManager(config *models.Config, storage storage.Storage) *BackupManager {
	return &BackupManager{
		config:   config,
		storage:  storage,
		scanner:  scanner.NewChunkScanner(config.ChunkPath),
		archiver: archiver.NewArchiver(config.ChunkPath, config.TempPath),
	}
}

// RunFullBackup 执行全量备份
func (bm *BackupManager) RunFullBackup(ctx context.Context) (*models.BackupResult, error) {
	startTime := time.Now()
	result := &models.BackupResult{
		Details: make(map[string]string),
	}

	// 1. 扫描文件树
	fileTree, err := bm.scanner.ScanFileTree()
	if err != nil {
		return nil, fmt.Errorf("failed to scan file tree: %w", err)
	}

	// 2. 获取chunk目录列表
	directories, err := bm.scanner.GetChunkDirectories()
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk directories: %w", err)
	}

	// 3. 生成压缩包分组
	groups, err := bm.archiver.GenerateArchiveGroups(directories, bm.config.PrefixDigits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate archive groups: %w", err)
	}

	// 4. 创建所有压缩包
	checksums := make(map[string]string)
	for _, group := range groups {
		err := bm.processArchiveGroup(ctx, group, checksums, result)
		if err != nil {
			result.ErrorArchives = append(result.ErrorArchives, group.ArchiveName)
			result.Details[group.ArchiveName] = err.Error()
		}
	}

	// 5. 创建并上传备份元数据
	metadata := &models.BackupMetadata{
		Version:      MetadataVersion,
		PrefixDigits: bm.config.PrefixDigits,
		BackupTime:   startTime,
		FileTree:     fileTree,
		Checksums:    checksums,
	}

	err = bm.saveAndUploadMetadata(ctx, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	result.TotalArchives = len(groups)
	result.Duration = time.Since(startTime)

	return result, nil
}

// RunIncrementalBackup 执行增量备份
func (bm *BackupManager) RunIncrementalBackup(ctx context.Context) (*models.BackupResult, error) {
	startTime := time.Now()
	result := &models.BackupResult{
		Details: make(map[string]string),
	}

	// 1. 下载并解析上次的备份元数据
	oldMetadata, err := bm.loadRemoteMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load previous backup metadata: %w", err)
	}

	// 2. 扫描当前文件树
	currentFileTree, err := bm.scanner.ScanFileTree()
	if err != nil {
		return nil, fmt.Errorf("failed to scan current file tree: %w", err)
	}

	// 3. 比较文件树，找出变化的目录
	changedDirs := scanner.CompareFileTrees(oldMetadata.FileTree, currentFileTree)

	// 4. 获取当前chunk目录列表
	directories, err := bm.scanner.GetChunkDirectories()
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk directories: %w", err)
	}

	// 5. 使用原前缀位数生成压缩包分组
	groups, err := bm.archiver.GenerateArchiveGroups(directories, oldMetadata.PrefixDigits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate archive groups: %w", err)
	}

	// 6. 标记需要更新的压缩包
	bm.archiver.MarkGroupsForUpdate(groups, changedDirs)

	// 7. 处理需要更新的压缩包
	checksums := make(map[string]string)
	// 首先复制旧的校验和
	for k, v := range oldMetadata.Checksums {
		checksums[k] = v
	}

	for _, group := range groups {
		if group.NeedsUpdate {
			err := bm.processArchiveGroup(ctx, group, checksums, result)
			if err != nil {
				result.ErrorArchives = append(result.ErrorArchives, group.ArchiveName)
				result.Details[group.ArchiveName] = err.Error()
			}
		} else {
			result.SkippedArchives++
			result.Details[group.ArchiveName] = "unchanged, skipped"
		}
	}

	// 8. 创建并上传新的备份元数据
	metadata := &models.BackupMetadata{
		Version:      MetadataVersion,
		PrefixDigits: oldMetadata.PrefixDigits,
		BackupTime:   startTime,
		FileTree:     currentFileTree,
		Checksums:    checksums,
	}

	err = bm.saveAndUploadMetadata(ctx, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	result.TotalArchives = len(groups)
	result.Duration = time.Since(startTime)

	return result, nil
}

// processArchiveGroup 处理单个压缩包组
func (bm *BackupManager) processArchiveGroup(ctx context.Context, group *models.ArchiveGroup, checksums map[string]string, result *models.BackupResult) error {
	// 1. 创建压缩包
	archivePath, err := bm.archiver.CreateArchive(group)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer os.Remove(archivePath) // 清理临时文件

	// 2. 计算校验和
	checksum, err := bm.archiver.CalculateChecksum(archivePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// 3. 检查远程校验和是否已存在且相同
	remoteSha256Path := filepath.Join(bm.config.RemotePath, group.ArchiveName+".sha256")
	needsUpload := true

	if remoteChecksum, err := bm.getRemoteChecksum(ctx, remoteSha256Path); err == nil {
		if remoteChecksum == checksum {
			needsUpload = false
			result.Details[group.ArchiveName] = "checksum unchanged, skipped upload"
		}
	}

	if needsUpload {
		// 4. 上传压缩包
		remoteArchivePath := filepath.Join(bm.config.RemotePath, group.ArchiveName)
		err = bm.storage.UploadFile(ctx, archivePath, remoteArchivePath)
		if err != nil {
			return fmt.Errorf("failed to upload archive: %w", err)
		}
		result.UploadedFiles = append(result.UploadedFiles, group.ArchiveName)

		// 5. 创建并上传校验和文件
		checksumPath, err := bm.archiver.CreateChecksumFile(archivePath, checksum)
		if err != nil {
			return fmt.Errorf("failed to create checksum file: %w", err)
		}
		defer os.Remove(checksumPath) // 清理临时文件

		err = bm.storage.UploadFile(ctx, checksumPath, remoteSha256Path)
		if err != nil {
			return fmt.Errorf("failed to upload checksum file: %w", err)
		}
		result.UploadedFiles = append(result.UploadedFiles, group.ArchiveName+".sha256")

		result.UpdatedArchives++
		result.Details[group.ArchiveName] = "created and uploaded"
	} else {
		result.SkippedArchives++
		result.Details[group.ArchiveName] = "checksum unchanged, skipped"
	}

	// 更新校验和映射
	checksums[group.ArchiveName] = checksum

	return nil
}

// loadRemoteMetadata 从远程加载备份元数据
func (bm *BackupManager) loadRemoteMetadata(ctx context.Context) (*models.BackupMetadata, error) {
	remotePath := filepath.Join(bm.config.RemotePath, MetadataFileName)

	// 检查文件是否存在
	exists, err := bm.storage.FileExists(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check metadata file existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("no previous backup metadata found, use full backup mode")
	}

	// 下载元数据内容
	content, err := bm.storage.GetFileContent(ctx, remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to download metadata: %w", err)
	}

	var metadata models.BackupMetadata
	err = json.Unmarshal(content, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// saveAndUploadMetadata 保存并上传备份元数据
func (bm *BackupManager) saveAndUploadMetadata(ctx context.Context, metadata *models.BackupMetadata) error {
	// 1. 序列化元数据
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// 2. 保存到本地临时文件
	localPath := filepath.Join(bm.config.TempPath, MetadataFileName)
	err = os.WriteFile(localPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to save local metadata: %w", err)
	}

	// 3. 上传到远程
	remotePath := filepath.Join(bm.config.RemotePath, MetadataFileName)
	err = bm.storage.UploadFile(ctx, localPath, remotePath)
	if err != nil {
		return fmt.Errorf("failed to upload metadata: %w", err)
	}

	// 4. 保留本地副本（不删除临时文件）
	return nil
}

// getRemoteChecksum 获取远程校验和文件内容
func (bm *BackupManager) getRemoteChecksum(ctx context.Context, remotePath string) (string, error) {
	content, err := bm.storage.GetFileContent(ctx, remotePath)
	if err != nil {
		return "", err
	}

	// 解析校验和文件格式：<checksum>  <filename>
	parts := strings.Fields(string(content))
	if len(parts) >= 1 {
		return parts[0], nil
	}

	return "", fmt.Errorf("invalid checksum file format")
}
