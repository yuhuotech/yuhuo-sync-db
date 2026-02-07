package logger

import (
	"fmt"
	"io"
	"os"
	"time"
)

// LogLevel 日志等级
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Logger 日志记录器
type Logger struct {
	level      LogLevel
	fileHandle *os.File
	writers    []io.Writer
}

// NewLogger 创建日志记录器
func NewLogger(logLevel string, logFile string) (*Logger, error) {
	var level LogLevel
	switch logLevel {
	case "DEBUG":
		level = DEBUG
	case "WARN":
		level = WARN
	case "ERROR":
		level = ERROR
	default:
		level = INFO
	}

	// 打开日志文件
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := &Logger{
		level:      level,
		fileHandle: file,
		writers:    []io.Writer{os.Stdout, file},
	}

	return logger, nil
}

// logWithLevel 记录指定级别的日志
func (l *Logger) logWithLevel(levelStr string, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] [%s] %s\n", timestamp, levelStr, message)

	for _, writer := range l.writers {
		fmt.Fprint(writer, logMessage)
	}
}

// Debug 记录 DEBUG 级别日志
func (l *Logger) Debug(message string) {
	if l.level <= DEBUG {
		l.logWithLevel("DEBUG", message)
	}
}

// Info 记录 INFO 级别日志
func (l *Logger) Info(message string) {
	if l.level <= INFO {
		l.logWithLevel("INFO", message)
	}
}

// Warn 记录 WARN 级别日志
func (l *Logger) Warn(message string) {
	if l.level <= WARN {
		l.logWithLevel("WARN", message)
	}
}

// Error 记录 ERROR 级别日志
func (l *Logger) Error(message string) {
	if l.level <= ERROR {
		l.logWithLevel("ERROR", message)
	}
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	if l.fileHandle != nil {
		return l.fileHandle.Close()
	}
	return nil
}
