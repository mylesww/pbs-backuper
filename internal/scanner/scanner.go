package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"pbs-backuper/internal/models"
)

// ChunkScanner 负责扫描.chunk目录
type ChunkScanner struct {
	chunkPath string
}

// NewChunkScanner 创建新的扫描器
func NewChunkScanner(chunkPath string) *ChunkScanner {
	return &ChunkScanner{
		chunkPath: chunkPath,
	}
}

// ScanFileTree 扫描chunk目录，构建文件树
func (s *ChunkScanner) ScanFileTree() (map[string]*models.FileTreeNode, error) {
	fileTree := make(map[string]*models.FileTreeNode)

	// 检查chunk目录是否存在
	if _, err := os.Stat(s.chunkPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chunk directory does not exist: %s", s.chunkPath)
	}

	// 遍历chunk目录下的所有条目
	entries, err := os.ReadDir(s.chunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk directory: %w", err)
	}

	// 只处理符合16进制命名规则的目录
	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]{4}$`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue // 跳过非目录文件
		}

		// 检查目录名是否符合4位16进制格式
		if !hexPattern.MatchString(entry.Name()) {
			continue // 跳过不符合命名规则的目录
		}

		// 扫描子目录
		dirPath := filepath.Join(s.chunkPath, entry.Name())
		node, err := s.scanDirectory(dirPath)
		if err != nil {
			return nil, fmt.Errorf("failed to scan directory %s: %w", dirPath, err)
		}

		fileTree[entry.Name()] = node
	}

	return fileTree, nil
}

// scanDirectory 递归扫描目录，构建文件树节点
func (s *ChunkScanner) scanDirectory(dirPath string) (*models.FileTreeNode, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat directory %s: %w", dirPath, err)
	}

	node := &models.FileTreeNode{
		Name:     filepath.Base(dirPath),
		Size:     0, // 目录大小计算为所有子文件的总和
		ModTime:  info.ModTime(),
		IsDir:    true,
		Children: make(map[string]*models.FileTreeNode),
	}

	// 读取目录内容
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// 遍历目录中的所有条目
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())

		if entry.IsDir() {
			// 递归处理子目录
			childNode, err := s.scanDirectory(entryPath)
			if err != nil {
				return nil, err
			}
			node.Children[entry.Name()] = childNode
			node.Size += childNode.Size // 累加子目录大小
		} else {
			// 处理文件
			fileInfo, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to get file info for %s: %w", entryPath, err)
			}

			fileNode := &models.FileTreeNode{
				Name:    entry.Name(),
				Size:    fileInfo.Size(),
				ModTime: fileInfo.ModTime(),
				IsDir:   false,
			}

			node.Children[entry.Name()] = fileNode
			node.Size += fileInfo.Size() // 累加文件大小
		}
	}

	return node, nil
}

// GetChunkDirectories 获取所有有效的chunk目录名列表（按字典序排序）
func (s *ChunkScanner) GetChunkDirectories() ([]string, error) {
	entries, err := os.ReadDir(s.chunkPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk directory: %w", err)
	}

	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]{4}$`)
	var directories []string

	for _, entry := range entries {
		if entry.IsDir() && hexPattern.MatchString(entry.Name()) {
			directories = append(directories, entry.Name())
		}
	}

	// 按字典序排序
	sort.Strings(directories)

	return directories, nil
}

// CompareFileTrees 比较两个文件树，找出差异
func CompareFileTrees(oldTree, newTree map[string]*models.FileTreeNode) map[string]bool {
	changedDirs := make(map[string]bool)

	// 检查新树中的目录
	for dirName, newNode := range newTree {
		oldNode, exists := oldTree[dirName]
		if !exists {
			// 新增目录
			changedDirs[dirName] = true
			continue
		}

		// 比较目录树
		if hasTreeChanged(oldNode, newNode) {
			changedDirs[dirName] = true
		}
	}

	// 检查删除的目录
	for dirName := range oldTree {
		if _, exists := newTree[dirName]; !exists {
			changedDirs[dirName] = true
		}
	}

	return changedDirs
}

// hasTreeChanged 递归比较两个文件树节点是否有变化
func hasTreeChanged(oldNode, newNode *models.FileTreeNode) bool {
	// 比较基本属性
	if oldNode.Size != newNode.Size ||
		!oldNode.ModTime.Equal(newNode.ModTime) ||
		oldNode.IsDir != newNode.IsDir {
		return true
	}

	// 如果是文件，直接返回结果
	if !oldNode.IsDir {
		return false
	}

	// 比较子节点数量
	if len(oldNode.Children) != len(newNode.Children) {
		return true
	}

	// 递归比较所有子节点
	for name, oldChild := range oldNode.Children {
		newChild, exists := newNode.Children[name]
		if !exists {
			return true // 子节点被删除
		}

		if hasTreeChanged(oldChild, newChild) {
			return true
		}
	}

	// 检查新增的子节点
	for name := range newNode.Children {
		if _, exists := oldNode.Children[name]; !exists {
			return true // 新增子节点
		}
	}

	return false
}
