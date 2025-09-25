package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"pbs-backuper/internal/backup"
	"pbs-backuper/internal/logger"
	"pbs-backuper/internal/models"
	"pbs-backuper/internal/storage"
)

var (
	chunkPath    string
	remotePath   string
	tempPath     string
	rcloneBinary string
	rcloneConfig string
	rcloneArgs   []string
	prefixDigits int
	verbose      bool
	timeout      time.Duration
	logPath      string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:   "backuper",
	Short: "PVE备份服务器chunk数据备份工具",
	Long: `PVE备份服务器chunk数据备份工具，支持：
- 可配置前缀分组的全量备份
- 基于文件树变化的增量备份
- SHA256校验和验证
- 通过rclone进行云存储

该工具扫描.chunk目录（以4位十六进制0000-ffff命名）
并根据前缀分组创建压缩包。`,
	Example: `  # 使用2位前缀分组的全量备份
  backuper full --chunk-path /path/to/.chunk --remote-path remote:backup --prefix-digits 2

  # 增量备份
  backuper incremental --chunk-path /path/to/.chunk --remote-path remote:backup

  # 使用自定义rclone配置
  backuper full --chunk-path /path/to/.chunk --remote-path remote:backup \\
    --rclone-binary /usr/bin/rclone --rclone-config ~/.config/rclone/rclone.conf \\
    --rclone-args "--transfers=4,--checkers=8" --prefix-digits 3`,
}

// fullCmd 全量备份命令
var fullCmd = &cobra.Command{
	Use:   "full",
	Short: "执行全量备份",
	Long: `执行所有chunk目录的全量备份。
根据前缀分组创建压缩包并上传到远程存储。
生成备份元数据用于将来的增量备份。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := buildConfig("full")
		if err != nil {
			return fmt.Errorf("配置无效: %w", err)
		}

		return runBackup(config)
	},
}

// incrementalCmd 增量备份命令
var incrementalCmd = &cobra.Command{
	Use:   "incremental",
	Short: "执行增量备份",
	Long: `基于之前的备份元数据执行增量备份。
仅为变化的目录创建和上传压缩包。
要求远程存储中存在之前的备份元数据。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		config, err := buildConfig("incremental")
		if err != nil {
			return fmt.Errorf("配置无效: %w", err)
		}

		return runBackup(config)
	},
}

func init() {
	// 添加全局标志
	rootCmd.PersistentFlags().StringVar(&chunkPath, "chunk-path", "", ".chunk目录路径（必需）")
	rootCmd.PersistentFlags().StringVar(&remotePath, "remote-path", "", "远程存储路径（必需）")
	rootCmd.PersistentFlags().StringVar(&tempPath, "temp-path", "/tmp/backuper", "临时文件路径")
	rootCmd.PersistentFlags().StringVar(&rcloneBinary, "rclone-binary", "rclone", "rclone二进制文件路径")
	rootCmd.PersistentFlags().StringVar(&rcloneConfig, "rclone-config", "", "rclone配置文件路径")
	rootCmd.PersistentFlags().StringSliceVar(&rcloneArgs, "rclone-args", []string{}, "额外的rclone参数（逗号分隔）")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用详细输出")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Minute, "操作超时时间")
	rootCmd.PersistentFlags().StringVar(&logPath, "log-path", "", "日志文件路径（可选，默认仅输出到控制台）")

	// 全量备份特有标志
	fullCmd.Flags().IntVar(&prefixDigits, "prefix-digits", 2, "分组前缀位数（1-4）")

	// 标记必需参数
	rootCmd.MarkPersistentFlagRequired("chunk-path")
	rootCmd.MarkPersistentFlagRequired("remote-path")

	// 添加子命令
	rootCmd.AddCommand(fullCmd)
	rootCmd.AddCommand(incrementalCmd)
}

// Execute 执行命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// buildConfig 构建配置对象
func buildConfig(mode string) (*models.Config, error) {
	// 验证必需参数
	if chunkPath == "" {
		return nil, fmt.Errorf("chunk-path是必需的")
	}
	if remotePath == "" {
		return nil, fmt.Errorf("remote-path是必需的")
	}

	// 验证chunk路径
	if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("chunk目录不存在: %s", chunkPath)
	}

	// 验证前缀位数（仅全量备份）
	if mode == "full" {
		if prefixDigits < 1 || prefixDigits > 4 {
			return nil, fmt.Errorf("前缀位数必须在1到4之间，得到%d", prefixDigits)
		}
	}

	// 处理rclone参数
	var processedArgs []string
	for _, arg := range rcloneArgs {
		// 支持逗号分隔的参数
		if strings.Contains(arg, ",") {
			parts := strings.Split(arg, ",")
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					processedArgs = append(processedArgs, trimmed)
				}
			}
		} else {
			if trimmed := strings.TrimSpace(arg); trimmed != "" {
				processedArgs = append(processedArgs, trimmed)
			}
		}
	}

	return &models.Config{
		ChunkPath:    chunkPath,
		RemotePath:   remotePath,
		TempPath:     tempPath,
		RcloneBinary: rcloneBinary,
		RcloneConfig: rcloneConfig,
		RcloneArgs:   processedArgs,
		PrefixDigits: prefixDigits,
		Mode:         mode,
		Verbose:      verbose,
	}, nil
}

// runBackup 执行备份
func runBackup(config *models.Config) error {
	// 初始化日志系统
	if err := logger.InitLogger(config.Verbose, logPath); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}

	// 创建存储实例
	store := storage.NewRcloneStorage(config.RcloneBinary, config.RcloneConfig, config.RcloneArgs)

	// 创建备份管理器
	manager := backup.NewBackupManager(config, store)

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 确保临时目录存在
	if err := os.MkdirAll(config.TempPath, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 记录备份开始
	logger.LogBackupStart(config.Mode, config.ChunkPath, config.RemotePath)

	fmt.Printf("开始%s备份...\n", config.Mode)
	fmt.Printf("Chunk路径: %s\n", config.ChunkPath)
	fmt.Printf("远程路径: %s\n", config.RemotePath)
	fmt.Printf("临时路径: %s\n", config.TempPath)

	// 执行备份
	var result *models.BackupResult
	var err error

	if config.Mode == "full" {
		fmt.Printf("前缀位数: %d\n", config.PrefixDigits)
		result, err = manager.RunFullBackup(ctx)
	} else {
		result, err = manager.RunIncrementalBackup(ctx)
	}

	if err != nil {
		logger.Errorf("备份失败: %v", err)
		return fmt.Errorf("备份失败: %w", err)
	}

	// 记录备份完成
	logger.LogBackupComplete(config.Mode, result.Duration, result.TotalArchives,
		result.UpdatedArchives, result.SkippedArchives, len(result.ErrorArchives))

	// 输出结果
	printBackupResult(result, config.Verbose)

	return nil
}

// printBackupResult 输出备份结果
func printBackupResult(result *models.BackupResult, verbose bool) {
	fmt.Printf("\n=== 备份完成 ===\n")
	fmt.Printf("耗时: %v\n", result.Duration)
	fmt.Printf("总压缩包数: %d\n", result.TotalArchives)
	fmt.Printf("更新压缩包数: %d\n", result.UpdatedArchives)
	fmt.Printf("跳过压缩包数: %d\n", result.SkippedArchives)
	fmt.Printf("错误压缩包数: %d\n", len(result.ErrorArchives))
	fmt.Printf("上传文件数: %d\n", len(result.UploadedFiles))

	if len(result.ErrorArchives) > 0 {
		fmt.Printf("\n错误:\n")
		for _, archive := range result.ErrorArchives {
			fmt.Printf("  - %s: %s\n", archive, result.Details[archive])
		}
	}

	if verbose && len(result.Details) > 0 {
		fmt.Printf("\n详细结果:\n")
		for archive, detail := range result.Details {
			fmt.Printf("  %s: %s\n", archive, detail)
		}
	}

	if len(result.UploadedFiles) > 0 {
		fmt.Printf("\n已上传文件:\n")
		for _, file := range result.UploadedFiles {
			fmt.Printf("  - %s\n", file)
		}
	}

	if len(result.ErrorArchives) > 0 {
		logger.Warnf("备份完成，但有%d个错误", len(result.ErrorArchives))
	} else {
		fmt.Printf("\n备份成功完成！\n")
	}
}
