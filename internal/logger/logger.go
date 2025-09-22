package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

// InitLogger 初始化日志系统
func InitLogger(verbose bool, logPath string) error {
	Logger = logrus.New()

	// 设置日志格式
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   false,
	})

	// 设置日志级别
	if verbose {
		Logger.SetLevel(logrus.DebugLevel)
	} else {
		Logger.SetLevel(logrus.InfoLevel)
	}

	// 如果指定了日志路径，同时输出到文件和控制台
	if logPath != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(logPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		// 创建或打开日志文件
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		// 同时输出到文件和控制台
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		Logger.SetOutput(multiWriter)
	} else {
		// 只输出到控制台
		Logger.SetOutput(os.Stdout)
	}

	return nil
}

// GetLogger 获取日志实例
func GetLogger() *logrus.Logger {
	if Logger == nil {
		// 如果未初始化，使用默认配置
		Logger = logrus.New()
		Logger.SetLevel(logrus.InfoLevel)
		Logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	return Logger
}

// WithField 创建带字段的日志条目
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields 创建带多个字段的日志条目
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// Info 记录信息级别日志
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof 记录格式化信息级别日志
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Debug 记录调试级别日志
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf 记录格式化调试级别日志
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Warn 记录警告级别日志
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf 记录格式化警告级别日志
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error 记录错误级别日志
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf 记录格式化错误级别日志
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal 记录致命错误日志并退出程序
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf 记录格式化致命错误日志并退出程序
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// LogBackupStart 记录备份开始
func LogBackupStart(mode string, chunkPath string, remotePath string) {
	WithFields(logrus.Fields{
		"mode":        mode,
		"chunk_path":  chunkPath,
		"remote_path": remotePath,
		"start_time":  time.Now().Format("2006-01-02 15:04:05"),
	}).Info("Backup started")
}

// LogBackupComplete 记录备份完成
func LogBackupComplete(mode string, duration time.Duration, totalArchives, updatedArchives, skippedArchives, errorCount int) {
	WithFields(logrus.Fields{
		"mode":             mode,
		"duration":         duration.String(),
		"total_archives":   totalArchives,
		"updated_archives": updatedArchives,
		"skipped_archives": skippedArchives,
		"error_count":      errorCount,
	}).Info("Backup completed")
}

// LogArchiveOperation 记录压缩包操作
func LogArchiveOperation(archiveName string, operation string, duration time.Duration, size int64) {
	WithFields(logrus.Fields{
		"archive":   archiveName,
		"operation": operation,
		"duration":  duration.String(),
		"size":      size,
	}).Debug("Archive operation completed")
}

// LogFileOperation 记录文件操作
func LogFileOperation(fileName string, operation string, size int64) {
	WithFields(logrus.Fields{
		"file":      fileName,
		"operation": operation,
		"size":      size,
	}).Debug("File operation completed")
}
