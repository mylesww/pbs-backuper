package models

import (
	"time"
)

// FileTreeNode 文件树节点，记录文件/目录的基本信息
type FileTreeNode struct {
	Name     string                   `json:"name"`
	Size     int64                    `json:"size"`
	ModTime  time.Time                `json:"mod_time"`
	IsDir    bool                     `json:"is_dir"`
	Children map[string]*FileTreeNode `json:"children,omitempty"`
}

// BackupMetadata 备份元数据，记录整体备份信息
type BackupMetadata struct {
	Version      int                      `json:"version"`       // 元数据版本
	PrefixDigits int                      `json:"prefix_digits"` // 前缀位数
	BackupTime   time.Time                `json:"backup_time"`   // 备份时间
	FileTree     map[string]*FileTreeNode `json:"file_tree"`     // 文件树，key为顶层目录名
	Checksums    map[string]string        `json:"checksums"`     // 压缩包SHA256值，key为压缩包名
}

// Config 备份配置
type Config struct {
	ChunkPath    string   `json:"chunk_path"`    // .chunk目录路径
	RemotePath   string   `json:"remote_path"`   // 远程存储路径
	TempPath     string   `json:"temp_path"`     // 临时文件路径
	RcloneBinary string   `json:"rclone_binary"` // rclone二进制路径
	RcloneConfig string   `json:"rclone_config"` // rclone配置文件路径
	RcloneArgs   []string `json:"rclone_args"`   // rclone额外参数
	PrefixDigits int      `json:"prefix_digits"` // 前缀位数（全量备份使用）
	Mode         string   `json:"mode"`          // 备份模式：full/incremental
	Verbose      bool     `json:"verbose"`       // 详细日志
}

// ArchiveGroup 压缩包分组信息
type ArchiveGroup struct {
	Prefix      string   `json:"prefix"`       // 分组前缀，如"00"
	StartRange  string   `json:"start_range"`  // 开始范围，如"0000"
	EndRange    string   `json:"end_range"`    // 结束范围，如"00ff"
	ArchiveName string   `json:"archive_name"` // 压缩包名称，如"0000-00ff.tar.gz"
	Directories []string `json:"directories"`  // 包含的目录列表
	NeedsUpdate bool     `json:"needs_update"` // 是否需要更新
}

// BackupResult 备份结果
type BackupResult struct {
	TotalArchives   int               `json:"total_archives"`
	UpdatedArchives int               `json:"updated_archives"`
	SkippedArchives int               `json:"skipped_archives"`
	ErrorArchives   []string          `json:"error_archives"`
	UploadedFiles   []string          `json:"uploaded_files"`
	Duration        time.Duration     `json:"duration"`
	Details         map[string]string `json:"details"` // 详细结果信息
}
