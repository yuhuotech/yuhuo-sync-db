package sync

import (
	"fmt"
	"strings"

	"github.com/yuhuo/sync-db/models"
)

// SQLGenerator 用于生成 SQL 语句
type SQLGenerator struct {
}

// NewSQLGenerator 创建 SQL 生成器
func NewSQLGenerator() *SQLGenerator {
	return &SQLGenerator{}
}

// GenerateSQL 根据差异生成 SQL 语句
func (sg *SQLGenerator) GenerateSQL(diff *models.SyncDifference) ([]string, error) {
	var sqls []string

	// 1. 先删除视图（因为可能有依赖关系）
	for _, viewDiff := range diff.ViewDifferences {
		if viewDiff.Operation == "DROP" || viewDiff.Operation == "MODIFY" {
			sql := fmt.Sprintf("DROP VIEW IF EXISTS `%s`;", viewDiff.ViewName)
			sqls = append(sqls, sql)
		}
	}

	// 2. 修改表结构
	for _, structDiff := range diff.StructureDifferences {
		structSQLs, err := sg.generateStructureSQL(structDiff)
		if err != nil {
			return nil, err
		}
		sqls = append(sqls, structSQLs...)
	}

	// 3. 创建新视图
	for _, viewDiff := range diff.ViewDifferences {
		if viewDiff.Operation == "CREATE" || viewDiff.Operation == "MODIFY" {
			sql := fmt.Sprintf("CREATE VIEW `%s` AS %s;", viewDiff.ViewName, viewDiff.NewDefinition)
			sqls = append(sqls, sql)
		}
	}

	// 4. 修改表数据
	for _, dataDiff := range diff.DataDifferences {
		dataSQLs, err := sg.generateDataSQL(dataDiff)
		if err != nil {
			return nil, err
		}
		sqls = append(sqls, dataSQLs...)
	}

	return sqls, nil
}

// generateStructureSQL 生成表结构修改 SQL
func (sg *SQLGenerator) generateStructureSQL(structDiff models.StructureDifference) ([]string, error) {
	var sqls []string

	tableName := structDiff.TableName

	// 如果是新表，生成 CREATE TABLE 语句
	if structDiff.IsNewTable && structDiff.TableDefinition != nil {
		createSQL := sg.generateCreateTableSQL(tableName, structDiff.TableDefinition)
		sqls = append(sqls, createSQL)
		return sqls, nil // 新表已创建，不需要后续的 ALTER TABLE
	}

	// 新增列 - 使用完整的列定义
	for _, col := range structDiff.ColumnsAdded {
		colDef := sg.buildColumnDefinition(col)
		sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s;", tableName, colDef)
		sqls = append(sqls, sql)
	}

	// 删除列
	for _, colName := range structDiff.ColumnsDeleted {
		sql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", tableName, colName)
		sqls = append(sqls, sql)
	}

	// 修改列 - 使用新列的完整定义
	for _, colMod := range structDiff.ColumnsModified {
		colDef := sg.buildColumnDefinition(colMod.NewColumn)
		sql := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN %s;", tableName, colDef)
		sqls = append(sqls, sql)
	}

	// 删除索引
	for _, idx := range structDiff.IndexesDeleted {
		if idx.Type == "PRIMARY" {
			sql := fmt.Sprintf("ALTER TABLE `%s` DROP PRIMARY KEY;", tableName)
			sqls = append(sqls, sql)
		} else {
			sql := fmt.Sprintf("ALTER TABLE `%s` DROP INDEX `%s`;", tableName, idx.Name)
			sqls = append(sqls, sql)
		}
	}

	// 新增索引
	for _, idx := range structDiff.IndexesAdded {
		sql := sg.generateAddIndexSQL(tableName, idx)
		sqls = append(sqls, sql)
	}

	return sqls, nil
}

// generateAddIndexSQL 生成添加索引的 SQL
func (sg *SQLGenerator) generateAddIndexSQL(tableName string, idx models.Index) string {
	cols := strings.Join(idx.Columns, "`, `")
	cols = "`" + cols + "`"

	switch idx.Type {
	case "PRIMARY":
		return fmt.Sprintf("ALTER TABLE `%s` ADD PRIMARY KEY (%s);", tableName, cols)
	case "UNIQUE":
		return fmt.Sprintf("ALTER TABLE `%s` ADD UNIQUE KEY `%s` (%s);", tableName, idx.Name, cols)
	default:
		return fmt.Sprintf("ALTER TABLE `%s` ADD INDEX `%s` (%s);", tableName, idx.Name, cols)
	}
}

// generateDataSQL 生成表数据修改 SQL
func (sg *SQLGenerator) generateDataSQL(dataDiff models.DataDifference) ([]string, error) {
	var sqls []string

	tableName := dataDiff.TableName
	pkColumn := dataDiff.PrimaryKeyName

	// 插入新增行
	if len(dataDiff.RowsToInsert) > 0 {
		insertSQLs := sg.generateInsertSQL(tableName, dataDiff.RowsToInsert)
		sqls = append(sqls, insertSQLs...)
	}

	// 更新修改行
	for _, updateRow := range dataDiff.RowsToUpdate {
		sql := sg.generateUpdateSQL(tableName, pkColumn, updateRow)
		sqls = append(sqls, sql)
	}

	// 删除行
	if len(dataDiff.RowsToDelete) > 0 {
		deleteSQLs := sg.generateDeleteSQL(tableName, pkColumn, dataDiff.RowsToDelete)
		sqls = append(sqls, deleteSQLs...)
	}

	return sqls, nil
}

// generateInsertSQL 生成 INSERT SQL（单行）
func (sg *SQLGenerator) generateInsertSQL(tableName string, rows []map[string]interface{}) []string {
	// 简化处理：每行单独生成一条 INSERT 语句
	// 实际可以批量生成以提高效率
	var sqls []string

	for _, row := range rows {
		var columns []string
		var values []interface{}

		for col, val := range row {
			columns = append(columns, col)
			values = append(values, val)
		}

		colStr := "`" + strings.Join(columns, "`, `") + "`"
		valStr := ""
		for i, val := range values {
			if i > 0 {
				valStr += ", "
			}
			valStr += sg.escapeValue(val)
		}

		sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);", tableName, colStr, valStr)
		sqls = append(sqls, sql)
	}

	return sqls
}

// generateUpdateSQL 生成 UPDATE SQL
func (sg *SQLGenerator) generateUpdateSQL(tableName, pkColumn string, updateRow models.UpdateRow) string {
	var setParts []string

	for col, newVal := range updateRow.NewValues {
		if col == pkColumn {
			continue // 主键不更新
		}
		setParts = append(setParts, fmt.Sprintf("`%s` = %s", col, sg.escapeValue(newVal)))
	}

	setClause := strings.Join(setParts, ", ")
	whereClause := fmt.Sprintf("`%s` = %s", pkColumn, sg.escapeValue(updateRow.PrimaryKeyValue))

	return fmt.Sprintf("UPDATE `%s` SET %s WHERE %s;", tableName, setClause, whereClause)
}

// generateDeleteSQL 生成 DELETE SQL
func (sg *SQLGenerator) generateDeleteSQL(tableName, pkColumn string, rows []map[string]interface{}) []string {
	var sqls []string

	for _, row := range rows {
		pkValue := row[pkColumn]
		sql := fmt.Sprintf("DELETE FROM `%s` WHERE `%s` = %s;", tableName, pkColumn, sg.escapeValue(pkValue))
		sqls = append(sqls, sql)
	}

	return sqls
}

// buildColumnDefinition 构建完整的列定义 SQL
func (sg *SQLGenerator) buildColumnDefinition(col models.Column) string {
	var sb strings.Builder

	// 列名和类型
	sb.WriteString("`" + col.Name + "` " + col.Type)

	// 如果类型包含长度（如 VARCHAR）但未在类型中包含，则添加
	if col.Length > 0 && !strings.Contains(strings.ToUpper(col.Type), "(") {
		// 仅当是 VARCHAR 等需要长度的类型时才添加
		if strings.Contains(strings.ToUpper(col.Type), "VARCHAR") ||
			strings.Contains(strings.ToUpper(col.Type), "CHAR") {
			sb.WriteString(fmt.Sprintf("(%d)", col.Length))
		}
	}

	// 默认值
	if col.DefaultValue != nil {
		sb.WriteString(" DEFAULT ")
		sb.WriteString(sg.escapeValue(*col.DefaultValue))
	}

	// 非空约束
	if !col.IsNullable {
		sb.WriteString(" NOT NULL")
	}

	// 自增
	if col.IsAutoIncrement {
		sb.WriteString(" AUTO_INCREMENT")
	}

	// 字符集和排序规则（如果指定）
	if col.Charset != nil {
		sb.WriteString(" CHARACTER SET " + *col.Charset)
	}
	if col.Collation != nil {
		sb.WriteString(" COLLATE " + *col.Collation)
	}

	return sb.String()
}

// generateCreateTableSQL 生成 CREATE TABLE 语句
func (sg *SQLGenerator) generateCreateTableSQL(tableName string, tableDef *models.TableDefinition) string {
	var sb strings.Builder

	sb.WriteString("CREATE TABLE `" + tableName + "` (\n")

	// 添加所有列
	for i, col := range tableDef.Columns {
		colDef := sg.buildColumnDefinition(col)
		sb.WriteString("  " + colDef)

		if i < len(tableDef.Columns)-1 {
			sb.WriteString(",\n")
		} else {
			// 检查是否有主键或索引需要添加
			if tableDef.PrimaryKey != "" || len(tableDef.Indexes) > 0 {
				sb.WriteString(",\n")
			} else {
				sb.WriteString("\n")
			}
		}
	}

	// 添加主键约束
	if tableDef.PrimaryKey != "" {
		sb.WriteString("  PRIMARY KEY (`" + tableDef.PrimaryKey + "`)")
		if len(tableDef.Indexes) > 0 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}

	// 添加其他索引
	for i, idx := range tableDef.Indexes {
		if idx.Type == "PRIMARY" {
			continue // 已处理
		}

		switch idx.Type {
		case "UNIQUE":
			sb.WriteString("  UNIQUE KEY `" + idx.Name + "` (`" + strings.Join(idx.Columns, "`, `") + "`)")
		default:
			sb.WriteString("  KEY `" + idx.Name + "` (`" + strings.Join(idx.Columns, "`, `") + "`)")
		}

		if i < len(tableDef.Indexes)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(");")

	return sb.String()
}

// escapeValue 转义 SQL 值
func (sg *SQLGenerator) escapeValue(val interface{}) string {
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case string:
		// 转义单引号和反斜杠
		escaped := strings.ReplaceAll(v, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "'", "\\'")
		return fmt.Sprintf("'%s'", escaped)
	case int, int64, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "1"
		}
		return "0"
	default:
		escaped := strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "\\'")
		return fmt.Sprintf("'%s'", escaped)
	}
}
