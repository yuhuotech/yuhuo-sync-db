package sync

import (
	"fmt"
	"time"

	"github.com/yuhuo/sync-db/database"
	"github.com/yuhuo/sync-db/logger"
)

// ExecutionResult 表示 SQL 执行的结果
type ExecutionResult struct {
	SQL       string
	Success   bool
	Error     error
	Duration  time.Duration
	Message   string
}

// Executor 用于执行 SQL 语句
type Executor struct {
	targetConn *database.Connection
	logger     *logger.Logger
}

// NewExecutor 创建执行器
func NewExecutor(targetConn *database.Connection, logger *logger.Logger) *Executor {
	return &Executor{
		targetConn: targetConn,
		logger:     logger,
	}
}

// ExecuteSQL 执行 SQL 语句列表
func (e *Executor) ExecuteSQL(sqls []string) []ExecutionResult {
	var results []ExecutionResult

	for _, sql := range sqls {
		result := e.executeSingleSQL(sql)
		results = append(results, result)

		// 记录日志
		if result.Success {
			e.logger.Info(fmt.Sprintf("SQL executed successfully: %s (%.2fms)", sql, result.Duration.Seconds()*1000))
		} else {
			e.logger.Error(fmt.Sprintf("SQL execution failed: %s, Error: %v", sql, result.Error))
		}
	}

	return results
}

// executeSingleSQL 执行单条 SQL 语句
func (e *Executor) executeSingleSQL(sql string) ExecutionResult {
	start := time.Now()

	result := ExecutionResult{
		SQL: sql,
	}

	_, err := e.targetConn.Exec(sql)
	duration := time.Since(start)
	result.Duration = duration

	if err != nil {
		result.Success = false
		result.Error = err
		result.Message = fmt.Sprintf("Error: %v", err)
	} else {
		result.Success = true
		result.Message = "Success"
	}

	return result
}

// GetSummary 获取执行摘要
func GetSummary(results []ExecutionResult) (total, success, failed int) {
	total = len(results)
	for _, result := range results {
		if result.Success {
			success++
		} else {
			failed++
		}
	}
	return
}

// GetFailedResults 获取所有失败的结果
func GetFailedResults(results []ExecutionResult) []ExecutionResult {
	var failed []ExecutionResult
	for _, result := range results {
		if !result.Success {
			failed = append(failed, result)
		}
	}
	return failed
}
