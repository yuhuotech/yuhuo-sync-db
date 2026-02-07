package database

import (
	"database/sql"
	"fmt"

	"github.com/yuhuo/sync-db/models"
)

// QueryHelper 辅助进行数据库查询
type QueryHelper struct {
	conn *Connection
}

// NewQueryHelper 创建查询助手
func NewQueryHelper(conn *Connection) *QueryHelper {
	return &QueryHelper{conn: conn}
}

// GetTables 获取数据库中的所有表列表（不包括视图）
func (qh *QueryHelper) GetTables() ([]string, error) {
	rows, err := qh.conn.Query(`
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

// GetTableDefinition 获取表的完整定义
func (qh *QueryHelper) GetTableDefinition(tableName string) (*models.TableDefinition, error) {
	tableDef := &models.TableDefinition{
		TableName: tableName,
	}

	// 获取列定义
	columns, primaryKey, err := qh.getColumns(tableName)
	if err != nil {
		return nil, err
	}
	tableDef.Columns = columns
	tableDef.PrimaryKey = primaryKey

	// 获取索引定义
	indexes, err := qh.getIndexes(tableName)
	if err != nil {
		return nil, err
	}
	tableDef.Indexes = indexes

	return tableDef, nil
}

// getColumns 获取表的列定义
func (qh *QueryHelper) getColumns(tableName string) ([]models.Column, string, error) {
	rows, err := qh.conn.Query(`
		SELECT
			COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE,
			COLUMN_DEFAULT, EXTRA, COLUMN_KEY,
			CHARACTER_SET_NAME, COLLATION_NAME, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, tableName)
	if err != nil {
		return nil, "", fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []models.Column
	var primaryKey string

	for rows.Next() {
		var (
			name             string
			columnType       string
			isNullable       string
			defaultValue     sql.NullString
			extra            string
			columnKey        string
			characterSetName sql.NullString
			collationName    sql.NullString
			columnComment    sql.NullString
		)

		if err := rows.Scan(&name, &columnType, &isNullable, &defaultValue, &extra, &columnKey, &characterSetName, &collationName, &columnComment); err != nil {
			return nil, "", fmt.Errorf("failed to scan column: %w", err)
		}

		col := models.Column{
			Name:            name,
			Type:            columnType,
			IsNullable:      isNullable == "YES",
			Extra:           extra,
			IsAutoIncrement: extra == "auto_increment",
		}

		if defaultValue.Valid {
			col.DefaultValue = &defaultValue.String
		}

		if characterSetName.Valid {
			col.Charset = &characterSetName.String
		}

		if collationName.Valid {
			col.Collation = &collationName.String
		}

		if columnComment.Valid {
			col.Comment = &columnComment.String
		}

		columns = append(columns, col)

		if columnKey == "PRI" {
			primaryKey = name
		}
	}

	return columns, primaryKey, rows.Err()
}

// getIndexes 获取表的索引定义
func (qh *QueryHelper) getIndexes(tableName string) ([]models.Index, error) {
	rows, err := qh.conn.Query(`
		SELECT INDEX_NAME, COLUMN_NAME, SEQ_IN_INDEX
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexes: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*models.Index)
	indexOrder := []string{}

	for rows.Next() {
		var indexName, columnName string
		var seqInIndex int

		if err := rows.Scan(&indexName, &columnName, &seqInIndex); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}

		if _, exists := indexMap[indexName]; !exists {
			indexType := "INDEX"
			if indexName == "PRIMARY" {
				indexType = "PRIMARY"
			}
			indexMap[indexName] = &models.Index{
				Name:    indexName,
				Type:    indexType,
				Columns: []string{},
			}
			indexOrder = append(indexOrder, indexName)
		}

		indexMap[indexName].Columns = append(indexMap[indexName].Columns, columnName)
	}

	var indexes []models.Index
	for _, name := range indexOrder {
		indexes = append(indexes, *indexMap[name])
	}

	return indexes, rows.Err()
}

// GetViews 获取数据库中的所有视图
func (qh *QueryHelper) GetViews() ([]models.ViewDefinition, error) {
	rows, err := qh.conn.Query(`
		SELECT TABLE_NAME, VIEW_DEFINITION
		FROM INFORMATION_SCHEMA.VIEWS
		WHERE TABLE_SCHEMA = DATABASE()
		ORDER BY TABLE_NAME
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query views: %w", err)
	}
	defer rows.Close()

	var views []models.ViewDefinition
	for rows.Next() {
		var viewName string
		var viewDef sql.NullString

		if err := rows.Scan(&viewName, &viewDef); err != nil {
			return nil, fmt.Errorf("failed to scan view: %w", err)
		}

		view := models.ViewDefinition{
			ViewName: viewName,
		}
		if viewDef.Valid {
			view.Definition = viewDef.String
		}

		views = append(views, view)
	}

	return views, rows.Err()
}

// GetPrimaryKeyValues 获取表的主键值列表
func (qh *QueryHelper) GetPrimaryKeyValues(tableName, primaryKeyColumn string) (map[interface{}]bool, error) {
	if primaryKeyColumn == "" {
		return nil, fmt.Errorf("table %s has no primary key", tableName)
	}

	query := fmt.Sprintf("SELECT `%s` FROM `%s`", primaryKeyColumn, tableName)
	rows, err := qh.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query primary key values: %w", err)
	}
	defer rows.Close()

	pkValues := make(map[interface{}]bool)
	for rows.Next() {
		var pkValue interface{}
		if err := rows.Scan(&pkValue); err != nil {
			return nil, fmt.Errorf("failed to scan pk value: %w", err)
		}
		pkValues[pkValue] = true
	}

	return pkValues, rows.Err()
}

// GetRowByPrimaryKey 根据主键获取一行数据
func (qh *QueryHelper) GetRowByPrimaryKey(tableName, primaryKeyColumn string, pkValue interface{}) (map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM `%s` WHERE `%s` = ?", tableName, primaryKeyColumn)
	rows, err := qh.conn.Query(query, pkValue)
	if err != nil {
		return nil, fmt.Errorf("failed to query row: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range cols {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	row := make(map[string]interface{})
	for i, col := range cols {
		row[col] = values[i]
	}

	return row, nil
}

// GetAllRows 获取表的所有行数据
func (qh *QueryHelper) GetAllRows(tableName string) ([]map[string]interface{}, error) {
	rows, err := qh.conn.Query(fmt.Sprintf("SELECT * FROM `%s`", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query rows: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range cols {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range cols {
			row[col] = values[i]
		}

		result = append(result, row)
	}

	return result, rows.Err()
}

// GetCreateTableSQL 获取表的原始 CREATE TABLE 语句
func (qh *QueryHelper) GetCreateTableSQL(tableName string) (string, error) {
	rows, err := qh.conn.Query("SHOW CREATE TABLE `" + tableName + "`")
	if err != nil {
		return "", fmt.Errorf("failed to query create table statement: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return "", fmt.Errorf("no result from SHOW CREATE TABLE for table %s", tableName)
	}

	var table, createSQL string
	if err := rows.Scan(&table, &createSQL); err != nil {
		return "", fmt.Errorf("failed to scan create table statement: %w", err)
	}

	return createSQL, nil
}
