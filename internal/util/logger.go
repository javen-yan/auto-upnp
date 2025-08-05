package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"auto-upnp/config"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// SetupLogger 设置日志配置
func SetupLogger(cfg *config.Config, cmdLogLevel string) (*logrus.Logger, error) {
	logger := logrus.New()

	// 优先使用配置文件中的日志级别，如果没有则使用命令行参数
	logLevel := cmdLogLevel
	if cfg.Log.Level != "" {
		logLevel = cfg.Log.Level
	}

	// 设置日志级别
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return nil, fmt.Errorf("无效的日志级别: %s", logLevel)
	}
	logger.SetLevel(level)

	// 设置日志格式
	setupLogFormatter(logger, cfg.Log.Format, logLevel)

	// 设置日志输出
	if err := setupLogOutput(logger, cfg.Log); err != nil {
		return nil, err
	}

	return logger, nil
}

// setupLogFormatter 设置日志格式
func setupLogFormatter(logger *logrus.Logger, format, level string) {
	switch format {
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			ForceColors:     true,
			TimestampFormat: time.RFC3339,
		})
	case "json":
		fallthrough
	default:
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	}
}

// setupLogOutput 设置日志输出
func setupLogOutput(logger *logrus.Logger, logConfig config.LogConfig) error {
	var writers []io.Writer

	// 始终输出到控制台
	writers = append(writers, os.Stdout)

	// 如果配置了日志文件，添加文件输出
	if logConfig.File != "" {
		fileWriter, err := createLogFileWriter(logConfig)
		if err != nil {
			return fmt.Errorf("创建日志文件失败: %w", err)
		}
		writers = append(writers, fileWriter)
	}

	// 设置多输出
	if len(writers) > 1 {
		logger.SetOutput(io.MultiWriter(writers...))
	} else {
		logger.SetOutput(writers[0])
	}

	return nil
}

// createLogFileWriter 创建日志文件写入器（支持日志切割）
func createLogFileWriter(logConfig config.LogConfig) (io.Writer, error) {
	// 确保日志目录存在
	logDir := filepath.Dir(logConfig.File)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建日志切割器
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logConfig.File,
		MaxSize:    int(logConfig.MaxSize / 1024 / 1024), // 转换为MB
		MaxBackups: logConfig.BackupCount,
		MaxAge:     30, // 保留30天
		Compress:   true,
	}

	return lumberjackLogger, nil
}

// GetLogConfigInfo 获取日志配置信息
func GetLogConfigInfo(cfg *config.Config, cmdLogLevel string) map[string]interface{} {
	// 确定实际使用的日志级别
	actualLevel := cmdLogLevel
	if cfg.Log.Level != "" {
		actualLevel = cfg.Log.Level
	}

	return map[string]interface{}{
		"level":        actualLevel,
		"format":       cfg.Log.Format,
		"file":         cfg.Log.File,
		"max_size_mb":  cfg.Log.MaxSize / 1024 / 1024,
		"backup_count": cfg.Log.BackupCount,
		"cmd_level":    cmdLogLevel,
		"config_level": cfg.Log.Level,
	}
}
