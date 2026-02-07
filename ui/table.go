package ui

import (
	"fmt"

	"github.com/yuhuo/sync-db/models"
)

// PrintDifferenceSummary 打印差异汇总表格
func PrintDifferenceSummary(diff *models.SyncDifference) {
	fmt.Println("\n========== Sync Differences Summary ==========")
	fmt.Println()

	// 表头
	fmt.Printf("%-30s | %-15s | %-15s | %-15s | %-15s | %-15s\n",
		"Table", "Structure", "Rows To Insert", "Rows To Delete", "Rows To Update", "Views")
	fmt.Println(string(make([]byte, 130)) + " ")
	for i := 0; i < 130; i++ {
		fmt.Print("-")
	}
	fmt.Println()

	// 表结构差异
	structMap := make(map[string]*models.StructureDifference)
	for i := range diff.StructureDifferences {
		structMap[diff.StructureDifferences[i].TableName] = &diff.StructureDifferences[i]
	}

	// 获取所有涉及的表名
	allTables := make(map[string]bool)
	for _, structDiff := range diff.StructureDifferences {
		allTables[structDiff.TableName] = true
	}
	for tableName := range diff.DataDifferences {
		allTables[tableName] = true
	}

	// 打印每个表的信息
	for tableName := range allTables {
		structDiffs := "-"
		if sd, exists := structMap[tableName]; exists {
			count := len(sd.ColumnsAdded) + len(sd.ColumnsDeleted) + len(sd.ColumnsModified) + len(sd.IndexesAdded) + len(sd.IndexesDeleted)
			if count > 0 {
				structDiffs = fmt.Sprintf("%d", count)
			}
		}

		insertRows := "-"
		deleteRows := "-"
		updateRows := "-"
		if dataDiff, exists := diff.DataDifferences[tableName]; exists {
			if len(dataDiff.RowsToInsert) > 0 {
				insertRows = fmt.Sprintf("%d", len(dataDiff.RowsToInsert))
			}
			if len(dataDiff.RowsToDelete) > 0 {
				deleteRows = fmt.Sprintf("%d", len(dataDiff.RowsToDelete))
			}
			if len(dataDiff.RowsToUpdate) > 0 {
				updateRows = fmt.Sprintf("%d", len(dataDiff.RowsToUpdate))
			}
		}

		views := "-"

		fmt.Printf("%-30s | %-15s | %-15s | %-15s | %-15s | %-15s\n",
			tableName, structDiffs, insertRows, deleteRows, updateRows, views)
	}

	fmt.Println()
	fmt.Printf("Total view changes: %d\n", len(diff.ViewDifferences))
	fmt.Println()
}

// PrintSQLStatements 打印 SQL 语句列表
func PrintSQLStatements(sqls []string) {
	fmt.Println("\n========== Generated SQL Statements ==========")
	fmt.Println()

	for i, sql := range sqls {
		fmt.Printf("%d. %s\n", i+1, sql)
	}

	fmt.Printf("\nTotal: %d SQL statements\n\n", len(sqls))
}

// PrintExecutionSummary 打印执行摘要
func PrintExecutionSummary(total, success, failed int) {
	fmt.Println("\n========== Execution Summary ==========")
	fmt.Printf("Total: %d\n", total)
	fmt.Printf("Success: %d\n", success)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Println()
}

// PrintFailedSQLs 打印失败的 SQL 列表
func PrintFailedSQLs(failedResults interface{}) {
	fmt.Println("\n========== Failed SQL Statements ==========")
	fmt.Println()

	// 这里需要接收正确的类型，通过 sync 包中的 ExecutionResult
	fmt.Println("Failed statements:")
	// 具体实现取决于传入的类型
}

// PrintVerificationResult 打印验证结果
func PrintVerificationResult(success bool, message string) {
	fmt.Println("\n========== Verification Result ==========")
	fmt.Println(message)
	fmt.Println()

	if success {
		fmt.Println("✓ Sync completed successfully!")
	} else {
		fmt.Println("✗ Sync verification failed!")
	}
	fmt.Println()
}
