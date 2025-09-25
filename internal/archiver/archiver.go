package archiver

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pbs-backuper/internal/models"
)

// Archiver 负责创建和管理压缩包
type Archiver struct {
	chunkPath string
	tempPath  string
}

// NewArchiver 创建新的压缩器
func NewArchiver(chunkPath, tempPath string) *Archiver {
	return &Archiver{
		chunkPath: chunkPath,
		tempPath:  tempPath,
	}
}

// GenerateArchiveGroups 根据前缀位数生成压缩包分组
func (a *Archiver) GenerateArchiveGroups(directories []string, prefixDigits int) ([]*models.ArchiveGroup, error) {
	if prefixDigits < 1 || prefixDigits > 4 {
		return nil, fmt.Errorf("prefix digits must be between 1 and 4, got %d", prefixDigits)
	}

	// 将目录按前缀分组
	groupMap := make(map[string][]string)

	for _, dir := range directories {
		if len(dir) != 4 {
			continue // 跳过不符合格式的目录
		}

		prefix := dir[:prefixDigits]
		groupMap[prefix] = append(groupMap[prefix], dir)
	}

	var groups []*models.ArchiveGroup

	// 为每个前缀创建压缩包分组
	for prefix, dirs := range groupMap {
		sort.Strings(dirs) // 确保目录顺序一致

		// 计算范围
		startRange, endRange := a.calculateRange(prefix, prefixDigits)
		archiveName := fmt.Sprintf("%s-%s.tar.gz", startRange, endRange)

		group := &models.ArchiveGroup{
			Prefix:      prefix,
			StartRange:  startRange,
			EndRange:    endRange,
			ArchiveName: archiveName,
			Directories: dirs,
			NeedsUpdate: false,
		}

		groups = append(groups, group)
	}

	// 按前缀排序
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Prefix < groups[j].Prefix
	})

	return groups, nil
}

// calculateRange 根据前缀和位数计算范围
func (a *Archiver) calculateRange(prefix string, prefixDigits int) (string, string) {
	// 计算开始和结束范围
	startRange := prefix + strings.Repeat("0", 4-prefixDigits)
	endRange := prefix + strings.Repeat("f", 4-prefixDigits)

	return startRange, endRange
}

// CreateArchive 创建压缩包
func (a *Archiver) CreateArchive(group *models.ArchiveGroup) (string, error) {
	// 确保临时目录存在
	if err := os.MkdirAll(a.tempPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	archivePath := filepath.Join(a.tempPath, group.ArchiveName)

	// 创建tar.gz文件
	file, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("failed to create archive file: %w", err)
	}
	defer file.Close()

	// 创建gzip写入器
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	// 创建tar写入器
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// 添加每个目录到压缩包
	for _, dir := range group.Directories {
		dirPath := filepath.Join(a.chunkPath, dir)

		// 检查目录是否存在
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			continue // 跳过不存在的目录
		}

		// 将目录添加到tar包
		err := a.addDirectoryToTar(tarWriter, dirPath, dir)
		if err != nil {
			return "", fmt.Errorf("failed to add directory %s to archive: %w", dir, err)
		}
	}

	return archivePath, nil
}

// addDirectoryToTar 递归将目录添加到tar包
func (a *Archiver) addDirectoryToTar(tarWriter *tar.Writer, sourcePath, basePath string) error {
	return filepath.Walk(sourcePath, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算在tar包中的路径
		relPath, err := filepath.Rel(filepath.Dir(sourcePath), file)
		if err != nil {
			return err
		}

		// 创建tar头
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// 设置名称，使用正斜杠作为分隔符（tar标准）
		header.Name = filepath.ToSlash(relPath)

		// 写入头
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// 如果是文件，写入内容
		if !info.IsDir() {
			fileData, err := os.Open(file)
			if err != nil {
				return err
			}
			defer fileData.Close()

			_, err = io.Copy(tarWriter, fileData)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// CalculateChecksum 计算文件的SHA256校验和
func (a *Archiver) CalculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CreateChecksumFile 创建校验和文件
func (a *Archiver) CreateChecksumFile(archivePath, checksum string) (string, error) {
	checksumPath := archivePath + ".sha256"

	file, err := os.Create(checksumPath)
	if err != nil {
		return "", fmt.Errorf("failed to create checksum file: %w", err)
	}
	defer file.Close()

	// 写入校验和（格式：<checksum>  <filename>）
	archiveName := filepath.Base(archivePath)
	content := fmt.Sprintf("%s  %s\n", checksum, archiveName)

	if _, err := file.WriteString(content); err != nil {
		return "", fmt.Errorf("failed to write checksum: %w", err)
	}

	return checksumPath, nil
}

// MarkGroupsForUpdate 根据变化的目录标记需要更新的压缩包组
func (a *Archiver) MarkGroupsForUpdate(groups []*models.ArchiveGroup, changedDirs map[string]bool) {
	for _, group := range groups {
		// 检查该组中是否有任何目录发生变化
		for _, dir := range group.Directories {
			if changedDirs[dir] {
				group.NeedsUpdate = true
				break
			}
		}
	}
}
