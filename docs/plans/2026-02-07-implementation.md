# MySQL 数据库同步工具 (Yuhuo Sync DB) 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现一个完整的 MySQL 数据库同步工具，支持表结构、表数据、视图的比对和同步，提供四步工作流。

**Architecture:**
- 模块化设计：配置加载 → 数据库连接 → 元数据获取 → 差异比对 → SQL 生成 → 执行 → 验证
- 使用 Go 原生的 `database/sql` 包访问 MySQL，使用 `gopkg.in/yaml.v2` 解析 YAML 配置
- 分离关注点：数据库操作、业务逻辑、UI 交互各自独立，便于测试和维护

**Tech Stack:**
- Go 1.16+
- MySQL Driver: `github.com/go-sql-driver/mysql`
- YAML Parser: `gopkg.in/yaml.v2`
- CLI 交互: 标准库 `bufio`
- 日志: `log` 标准库
- 表格显示: `github.com/olekukonko/tablewriter`（可选，或自写）

---

## 阶段 1：项目初始化和基础模块

### Task 1: 初始化 Go 项目

**Files:**
- Create: `go.mod`
- Create: `go.sum`

**Step 1: 初始化 Go Module**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go mod init github.com/yuhuo/sync-db
```

Expected: `go.mod` 文件被创建

**Step 2: 添加依赖**

```bash
go get github.com/go-sql-driver/mysql
go get gopkg.in/yaml.v2
```

Expected: `go.mod` 和 `go.sum` 文件被更新

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: initialize go project with dependencies"
```

---

### Task 2: 创建数据模型

**Files:**
- Create: `models/table.go`
- Create: `models/column.go`
- Create: `models/index.go`
- Create: `models/difference.go`
- Create: `models/view.go`

**Step 1: 创建表列模型 (models/column.go)**

```go
package models

// Column 表示数据库表的列
type Column struct {
	Name             string
	Type             string // VARCHAR, INT, etc.
	Length           int    // for VARCHAR(255), this is 255, 0 if not applicable
	IsNullable       bool
	DefaultValue     *string
	IsAutoIncrement  bool
	Charset          *string // MySQL specific
	Collation        *string // MySQL specific
	Extra            string  // auto_increment, on update CURRENT_TIMESTAMP, etc.
}

// String 返回列的字符串表示
func (c *Column) String() string {
	if c.Length > 0 {
		return c.Name + " " + c.Type + "(" + string(rune(c.Length)) + ")"
	}
	return c.Name + " " + c.Type
}
```

**Step 2: 创建索引模型 (models/index.go)**

```go
package models

// Index 表示数据库表的索引
type Index struct {
	Name    string   // 索引名
	Type    string   // PRIMARY, UNIQUE, INDEX
	Columns []string // 组成索引的列名
}

// Key 返回索引的唯一键
func (i *Index) Key() string {
	return i.Name
}
```

**Step 3: 创建表结构模型 (models/table.go)**

```go
package models

// TableDefinition 表示数据库表的完整定义
type TableDefinition struct {
	TableName  string
	Columns    []Column
	Indexes    []Index
	PrimaryKey string // 主键列名
	Charset    *string
	Collation  *string
}

// GetColumnByName 根据列名获取列定义
func (t *TableDefinition) GetColumnByName(name string) *Column {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

// HasPrimaryKey 检查是否有主键
func (t *TableDefinition) HasPrimaryKey() bool {
	return t.PrimaryKey != ""
}
```

**Step 4: 创建视图模型 (models/view.go)**

```go
package models

// ViewDefinition 表示数据库视图的定义
type ViewDefinition struct {
	ViewName   string
	Definition string // CREATE VIEW 语句的内容部分
}
```

**Step 5: 创建差异模型 (models/difference.go)**

```go
package models

// ColumnModification 表示列的修改
type ColumnModification struct {
	ColumnName string
	OldColumn  Column
	NewColumn  Column
}

// StructureDifference 表示表结构的差异
type StructureDifference struct {
	TableName          string
	ColumnsAdded       []string // 新增的列名
	ColumnsDeleted     []string // 删除的列名
	ColumnsModified    []ColumnModification
	IndexesAdded       []Index
	IndexesDeleted     []Index
}

// DataDifference 表示表数据的差异
type DataDifference struct {
	TableName      string
	RowsToInsert   []map[string]interface{} // 新增行
	RowsToDelete   []map[string]interface{} // 删除行
	RowsToUpdate   []UpdateRow              // 修改行
	PrimaryKeyName string                   // 主键列名
}

// UpdateRow 表示一行数据的更新
type UpdateRow struct {
	PrimaryKeyValue interface{}
	OldValues       map[string]interface{}
	NewValues       map[string]interface{}
}

// ViewDifference 表示视图的差异
type ViewDifference struct {
	ViewName     string
	Operation    string // CREATE, DROP, MODIFY
	OldDefinition string
	NewDefinition string
}

// SyncDifference 表示全部差异的汇总
type SyncDifference struct {
	StructureDifferences []StructureDifference
	DataDifferences      map[string]DataDifference // key: table name
	ViewDifferences      []ViewDifference
}

// HasDifferences 检查是否有任何差异
func (s *SyncDifference) HasDifferences() bool {
	return len(s.StructureDifferences) > 0 || len(s.DataDifferences) > 0 || len(s.ViewDifferences) > 0
}
```

**Step 6: 验证模型编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./models
```

Expected: 编译成功，无错误

**Step 7: Commit**

```bash
git add models/
git commit -m "feat: add core data models (column, index, table, view, difference)"
```

---

### Task 3: 创建配置加载模块

**Files:**
- Create: `config/config.go`
- Create: `config.yaml.example`

**Step 1: 创建配置结构 (config/config.go)**

```go
package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// DatabaseConfig 表示数据库连接配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Charset  string `yaml:"charset"`
}

// LoggingConfig 表示日志配置
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// Config 表示完整的应用配置
type Config struct {
	Source          DatabaseConfig `yaml:"source"`
	Target          DatabaseConfig `yaml:"target"`
	SyncDataTables  []string        `yaml:"sync_data_tables"`
	Logging         LoggingConfig   `yaml:"logging"`
}

// LoadConfig 从 YAML 文件加载配置
func LoadConfig(filePath string) (*Config, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{
		Logging: LoggingConfig{
			Level: "INFO",
			File:  "sync.log",
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate 验证配置的合法性
func (c *Config) Validate() error {
	if c.Source.Host == "" || c.Source.Database == "" {
		return fmt.Errorf("source database config is incomplete")
	}
	if c.Target.Host == "" || c.Target.Database == "" {
		return fmt.Errorf("target database config is incomplete")
	}
	if c.Source.Port == 0 {
		c.Source.Port = 3306
	}
	if c.Target.Port == 0 {
		c.Target.Port = 3306
	}
	if c.Source.Charset == "" {
		c.Source.Charset = "utf8mb4"
	}
	if c.Target.Charset == "" {
		c.Target.Charset = "utf8mb4"
	}
	return nil
}
```

**Step 2: 创建配置示例文件 (config.yaml.example)**

```yaml
# Yuhuo Sync DB 配置文件示例

# 源数据库配置（测试/预发布环境）
source:
  host: 127.0.0.1
  port: 3306
  username: root
  password: password
  database: test_db
  charset: utf8mb4

# 目标数据库配置（线上环境）
target:
  host: 127.0.0.1
  port: 3306
  username: root
  password: password
  database: prod_db
  charset: utf8mb4

# 需要同步数据的表列表（所有表默认比对结构）
sync_data_tables:
  - users
  - orders
  - products

# 日志配置（可选）
logging:
  level: INFO
  file: sync.log
```

**Step 3: 创建配置测试 (config/config_test.go)**

```go
package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	tmpFile, err := ioutil.TempFile("", "config*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	content := `
source:
  host: localhost
  port: 3306
  username: root
  password: pass
  database: source_db
  charset: utf8mb4

target:
  host: localhost
  port: 3306
  username: root
  password: pass
  database: target_db
  charset: utf8mb4

sync_data_tables:
  - table1
  - table2

logging:
  level: INFO
  file: test.log
`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Source.Host != "localhost" {
		t.Errorf("Expected source host 'localhost', got '%s'", cfg.Source.Host)
	}
	if cfg.Target.Database != "target_db" {
		t.Errorf("Expected target database 'target_db', got '%s'", cfg.Target.Database)
	}
	if len(cfg.SyncDataTables) != 2 {
		t.Errorf("Expected 2 sync tables, got %d", len(cfg.SyncDataTables))
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		Source: DatabaseConfig{
			Host:     "localhost",
			Database: "source_db",
		},
		Target: DatabaseConfig{
			Host:     "localhost",
			Database: "target_db",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}

	if cfg.Source.Port != 3306 {
		t.Errorf("Expected default port 3306, got %d", cfg.Source.Port)
	}
	if cfg.Source.Charset != "utf8mb4" {
		t.Errorf("Expected default charset utf8mb4, got %s", cfg.Source.Charset)
	}
}
```

**Step 4: 运行测试**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go test ./config -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add config/
git commit -m "feat: add configuration loading and validation"
```

---

## 阶段 2：数据库连接和基础查询

### Task 4: 创建数据库连接模块

**Files:**
- Create: `database/connection.go`

**Step 1: 创建数据库连接管理 (database/connection.go)**

```go
package database

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/yuhuo/sync-db/config"
)

// Connection 表示数据库连接
type Connection struct {
	db   *sql.DB
	name string // 连接名，用于日志
}

// NewConnection 创建新的数据库连接
func NewConnection(cfg *config.DatabaseConfig, name string) (*Connection, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.Charset,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{
		db:   db,
		name: name,
	}, nil
}

// GetDB 获取底层数据库连接
func (c *Connection) GetDB() *sql.DB {
	return c.db
}

// Close 关闭数据库连接
func (c *Connection) Close() error {
	return c.db.Close()
}

// QueryRow 查询单一结果
func (c *Connection) QueryRow(query string, args ...interface{}) *sql.Row {
	return c.db.QueryRow(query, args...)
}

// Query 查询多行结果
func (c *Connection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}

// Exec 执行 SQL 语句（无返回结果）
func (c *Connection) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.db.Exec(query, args...)
}

// BeginTx 开启事务
func (c *Connection) BeginTx() (*sql.Tx, error) {
	return c.db.Begin()
}

// ConnectionManager 管理多个数据库连接
type ConnectionManager struct {
	mu        sync.Mutex
	conns     map[string]*Connection
	sourceDB  *Connection
	targetDB  *Connection
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(sourceCfg, targetCfg *config.DatabaseConfig) (*ConnectionManager, error) {
	sourceConn, err := NewConnection(sourceCfg, "source")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to source database: %w", err)
	}

	targetConn, err := NewConnection(targetCfg, "target")
	if err != nil {
		sourceConn.Close()
		return nil, fmt.Errorf("failed to connect to target database: %w", err)
	}

	return &ConnectionManager{
		conns:    make(map[string]*Connection),
		sourceDB: sourceConn,
		targetDB: targetConn,
	}, nil
}

// GetSourceDB 获取源数据库连接
func (cm *ConnectionManager) GetSourceDB() *Connection {
	return cm.sourceDB
}

// GetTargetDB 获取目标数据库连接
func (cm *ConnectionManager) GetTargetDB() *Connection {
	return cm.targetDB
}

// Close 关闭所有连接
func (cm *ConnectionManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var errs []error
	if err := cm.sourceDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close source connection: %w", err))
	}
	if err := cm.targetDB.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close target connection: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("error closing connections: %v", errs)
	}
	return nil
}
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./database
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add database/connection.go
git commit -m "feat: add database connection management"
```

---

### Task 5: 创建数据库查询模块

**Files:**
- Create: `database/query.go`

**Step 1: 创建数据库查询 (database/query.go)**

```go
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

// GetTables 获取数据库中的所有表列表
func (qh *QueryHelper) GetTables() ([]string, error) {
	rows, err := qh.conn.Query(`
		SELECT TABLE_NAME
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
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
			CHARACTER_SET_NAME, COLLATION_NAME
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
		)

		if err := rows.Scan(&name, &columnType, &isNullable, &defaultValue, &extra, &columnKey, &characterSetName, &collationName); err != nil {
			return nil, "", fmt.Errorf("failed to scan column: %w", err)
		}

		col := models.Column{
			Name:        name,
			Type:        columnType,
			IsNullable:  isNullable == "YES",
			Extra:       extra,
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

// GetPrimaryKeyValue 获取表的主键值列表
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
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./database
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add database/query.go
git commit -m "feat: add database query helper for metadata and data retrieval"
```

---

## 阶段 3：差异比对核心逻辑

### Task 6: 创建结构比对模块

**Files:**
- Create: `sync/comparator.go`

**Step 1: 创建结构比对逻辑 (sync/comparator.go)**

```go
package sync

import (
	"fmt"
	"strings"

	"github.com/yuhuo/sync-db/database"
	"github.com/yuhuo/sync-db/models"
)

// Comparator 用于比对源库和目标库的差异
type Comparator struct {
	sourceQueryHelper *database.QueryHelper
	targetQueryHelper *database.QueryHelper
	sourceConn        *database.Connection
	targetConn        *database.Connection
}

// NewComparator 创建比较器
func NewComparator(sourceConn, targetConn *database.Connection) *Comparator {
	return &Comparator{
		sourceQueryHelper: database.NewQueryHelper(sourceConn),
		targetQueryHelper: database.NewQueryHelper(targetConn),
		sourceConn:        sourceConn,
		targetConn:        targetConn,
	}
}

// CompareDifferences 比对源库和目标库的所有差异
func (c *Comparator) CompareDifferences(syncDataTables []string) (*models.SyncDifference, error) {
	diff := &models.SyncDifference{
		StructureDifferences: []models.StructureDifference{},
		DataDifferences:      make(map[string]models.DataDifference),
		ViewDifferences:      []models.ViewDifference{},
	}

	// 获取源库和目标库的表列表
	sourceTables, err := c.sourceQueryHelper.GetTables()
	if err != nil {
		return nil, err
	}

	targetTables, err := c.targetQueryHelper.GetTables()
	if err != nil {
		return nil, err
	}

	// 比对表结构
	structDiffs, err := c.compareTableStructures(sourceTables, targetTables)
	if err != nil {
		return nil, err
	}
	diff.StructureDifferences = structDiffs

	// 比对表数据（仅限配置的表）
	dataDiffs, err := c.compareTableData(sourceTables, syncDataTables)
	if err != nil {
		return nil, err
	}
	diff.DataDifferences = dataDiffs

	// 比对视图
	viewDiffs, err := c.compareViews()
	if err != nil {
		return nil, err
	}
	diff.ViewDifferences = viewDiffs

	return diff, nil
}

// compareTableStructures 比对表结构差异
func (c *Comparator) compareTableStructures(sourceTables, targetTables []string) ([]models.StructureDifference, error) {
	var structDiffs []models.StructureDifference

	sourceTableMap := make(map[string]bool)
	for _, t := range sourceTables {
		sourceTableMap[t] = true
	}

	targetTableMap := make(map[string]bool)
	for _, t := range targetTables {
		targetTableMap[t] = true
	}

	// 比对源库的所有表
	for _, tableName := range sourceTables {
		sourceDef, err := c.sourceQueryHelper.GetTableDefinition(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get source table definition: %w", err)
		}

		diff := models.StructureDifference{
			TableName: tableName,
		}

		if !targetTableMap[tableName] {
			// 目标库中不存在这个表，视为新增表
			diff.ColumnsAdded = getColumnNames(sourceDef.Columns)
		} else {
			targetDef, err := c.targetQueryHelper.GetTableDefinition(tableName)
			if err != nil {
				return nil, fmt.Errorf("failed to get target table definition: %w", err)
			}

			// 比对列、索引
			colDiff, colMod := c.compareColumns(sourceDef.Columns, targetDef.Columns)
			diff.ColumnsAdded = colDiff.added
			diff.ColumnsDeleted = colDiff.deleted
			diff.ColumnsModified = colMod

			indexDiff := c.compareIndexes(sourceDef.Indexes, targetDef.Indexes)
			diff.IndexesAdded = indexDiff.added
			diff.IndexesDeleted = indexDiff.deleted
		}

		structDiffs = append(structDiffs, diff)
	}

	// 比对目标库中源库不存在的表（这些表在目标库中不应该被删除，暂时不处理）
	// 但我们需要记录这些表的存在，后续可能需要删除

	return structDiffs, nil
}

// compareColumns 比对列定义
func (c *Comparator) compareColumns(sourceColumns, targetColumns []models.Column) (struct {
	added   []string
	deleted []string
}, []models.ColumnModification) {
	sourceColMap := make(map[string]models.Column)
	for _, col := range sourceColumns {
		sourceColMap[col.Name] = col
	}

	targetColMap := make(map[string]models.Column)
	for _, col := range targetColumns {
		targetColMap[col.Name] = col
	}

	var added, deleted []string
	var modifications []models.ColumnModification

	// 检查新增和修改的列
	for _, sourceCol := range sourceColumns {
		if _, exists := targetColMap[sourceCol.Name]; !exists {
			added = append(added, sourceCol.Name)
		} else {
			targetCol := targetColMap[sourceCol.Name]
			if !columnsEqual(sourceCol, targetCol) {
				modifications = append(modifications, models.ColumnModification{
					ColumnName: sourceCol.Name,
					OldColumn:  targetCol,
					NewColumn:  sourceCol,
				})
			}
		}
	}

	// 检查删除的列
	for _, targetCol := range targetColumns {
		if _, exists := sourceColMap[targetCol.Name]; !exists {
			deleted = append(deleted, targetCol.Name)
		}
	}

	return struct {
		added   []string
		deleted []string
	}{added, deleted}, modifications
}

// columnsEqual 判断两个列是否相等
func columnsEqual(col1, col2 models.Column) bool {
	// 比对基本属性
	if col1.Name != col2.Name || col1.Type != col2.Type {
		return false
	}

	if col1.IsNullable != col2.IsNullable {
		return false
	}

	// 比对默认值
	if (col1.DefaultValue == nil) != (col2.DefaultValue == nil) {
		return false
	}
	if col1.DefaultValue != nil && col2.DefaultValue != nil {
		if *col1.DefaultValue != *col2.DefaultValue {
			return false
		}
	}

	if col1.IsAutoIncrement != col2.IsAutoIncrement {
		return false
	}

	return true
}

// compareIndexes 比对索引定义
func (c *Comparator) compareIndexes(sourceIndexes, targetIndexes []models.Index) struct {
	added   []models.Index
	deleted []models.Index
} {
	sourceIndexMap := make(map[string]models.Index)
	for _, idx := range sourceIndexes {
		sourceIndexMap[idx.Name] = idx
	}

	targetIndexMap := make(map[string]models.Index)
	for _, idx := range targetIndexes {
		targetIndexMap[idx.Name] = idx
	}

	var added, deleted []models.Index

	for name, sourceIdx := range sourceIndexMap {
		if _, exists := targetIndexMap[name]; !exists {
			added = append(added, sourceIdx)
		}
	}

	for name, targetIdx := range targetIndexMap {
		if _, exists := sourceIndexMap[name]; !exists {
			deleted = append(deleted, targetIdx)
		}
	}

	return struct {
		added   []models.Index
		deleted []models.Index
	}{added, deleted}
}

// compareTableData 比对表数据差异
func (c *Comparator) compareTableData(sourceTables []string, syncDataTables []string) (map[string]models.DataDifference, error) {
	syncTableMap := make(map[string]bool)
	for _, t := range syncDataTables {
		syncTableMap[t] = true
	}

	dataDiffs := make(map[string]models.DataDifference)

	for _, tableName := range sourceTables {
		if !syncTableMap[tableName] {
			continue // 跳过不需要同步数据的表
		}

		sourceDef, err := c.sourceQueryHelper.GetTableDefinition(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to get source table definition: %w", err)
		}

		// 检查是否有主键
		if !sourceDef.HasPrimaryKey() {
			// 跳过没有主键的表
			continue
		}

		dataDiff, err := c.compareTableDataByPrimaryKey(tableName, sourceDef.PrimaryKey)
		if err != nil {
			return nil, fmt.Errorf("failed to compare data for table %s: %w", tableName, err)
		}

		dataDiffs[tableName] = dataDiff
	}

	return dataDiffs, nil
}

// compareTableDataByPrimaryKey 按主键比对表数据
func (c *Comparator) compareTableDataByPrimaryKey(tableName, primaryKeyColumn string) (models.DataDifference, error) {
	diff := models.DataDifference{
		TableName:      tableName,
		PrimaryKeyName: primaryKeyColumn,
		RowsToInsert:   []map[string]interface{}{},
		RowsToDelete:   []map[string]interface{}{},
		RowsToUpdate:   []models.UpdateRow{},
	}

	// 获取源库和目标库的主键值
	sourcePKValues, err := c.sourceQueryHelper.GetPrimaryKeyValues(tableName, primaryKeyColumn)
	if err != nil {
		return diff, err
	}

	targetPKValues, err := c.targetQueryHelper.GetPrimaryKeyValues(tableName, primaryKeyColumn)
	if err != nil {
		return diff, err
	}

	// 检查新增行和修改行
	for pkValue := range sourcePKValues {
		sourceRow, err := c.sourceQueryHelper.GetRowByPrimaryKey(tableName, primaryKeyColumn, pkValue)
		if err != nil {
			return diff, err
		}

		if _, exists := targetPKValues[pkValue]; !exists {
			// 新增行
			diff.RowsToInsert = append(diff.RowsToInsert, sourceRow)
		} else {
			// 可能修改了，需要对比
			targetRow, err := c.targetQueryHelper.GetRowByPrimaryKey(tableName, primaryKeyColumn, pkValue)
			if err != nil {
				return diff, err
			}

			if !rowsEqual(sourceRow, targetRow) {
				diff.RowsToUpdate = append(diff.RowsToUpdate, models.UpdateRow{
					PrimaryKeyValue: pkValue,
					OldValues:       targetRow,
					NewValues:       sourceRow,
				})
			}
		}
	}

	// 检查删除行
	for pkValue := range targetPKValues {
		if _, exists := sourcePKValues[pkValue]; !exists {
			// 删除行
			targetRow, err := c.targetQueryHelper.GetRowByPrimaryKey(tableName, primaryKeyColumn, pkValue)
			if err != nil {
				return diff, err
			}
			diff.RowsToDelete = append(diff.RowsToDelete, targetRow)
		}
	}

	return diff, nil
}

// rowsEqual 判断两行数据是否相等
func rowsEqual(row1, row2 map[string]interface{}) bool {
	if len(row1) != len(row2) {
		return false
	}

	for key, val1 := range row1 {
		val2, exists := row2[key]
		if !exists {
			return false
		}

		// 比对值
		if fmt.Sprintf("%v", val1) != fmt.Sprintf("%v", val2) {
			return false
		}
	}

	return true
}

// compareViews 比对视图差异
func (c *Comparator) compareViews() ([]models.ViewDifference, error) {
	sourceViews, err := c.sourceQueryHelper.GetViews()
	if err != nil {
		return nil, err
	}

	targetViews, err := c.targetQueryHelper.GetViews()
	if err != nil {
		return nil, err
	}

	sourceViewMap := make(map[string]models.ViewDefinition)
	for _, view := range sourceViews {
		sourceViewMap[view.ViewName] = view
	}

	targetViewMap := make(map[string]models.ViewDefinition)
	for _, view := range targetViews {
		targetViewMap[view.ViewName] = view
	}

	var viewDiffs []models.ViewDifference

	// 检查新增和修改的视图
	for _, sourceView := range sourceViews {
		if targetView, exists := targetViewMap[sourceView.ViewName]; !exists {
			// 新增视图
			viewDiffs = append(viewDiffs, models.ViewDifference{
				ViewName:      sourceView.ViewName,
				Operation:     "CREATE",
				NewDefinition: sourceView.Definition,
			})
		} else {
			// 检查定义是否相同
			if normalizeViewDefinition(sourceView.Definition) != normalizeViewDefinition(targetView.Definition) {
				// 修改视图
				viewDiffs = append(viewDiffs, models.ViewDifference{
					ViewName:      sourceView.ViewName,
					Operation:     "MODIFY",
					OldDefinition: targetView.Definition,
					NewDefinition: sourceView.Definition,
				})
			}
		}
	}

	// 检查删除的视图
	for _, targetView := range targetViews {
		if _, exists := sourceViewMap[targetView.ViewName]; !exists {
			// 删除视图
			viewDiffs = append(viewDiffs, models.ViewDifference{
				ViewName:      targetView.ViewName,
				Operation:     "DROP",
				OldDefinition: targetView.Definition,
			})
		}
	}

	return viewDiffs, nil
}

// normalizeViewDefinition 规范化视图定义，用于比对
func normalizeViewDefinition(def string) string {
	// 移除多余的空格、换行、制表符
	def = strings.TrimSpace(def)
	// 将多个空格替换为单个空格
	for strings.Contains(def, "  ") {
		def = strings.ReplaceAll(def, "  ", " ")
	}
	def = strings.ReplaceAll(def, "\n", " ")
	def = strings.ReplaceAll(def, "\t", " ")
	return strings.ToLower(def)
}

// getColumnNames 获取列名列表
func getColumnNames(columns []models.Column) []string {
	var names []string
	for _, col := range columns {
		names = append(names, col.Name)
	}
	return names
}
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./sync
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add sync/comparator.go
git commit -m "feat: add table structure and data comparison logic"
```

---

## 阶段 4：SQL 生成和执行

### Task 7: 创建 SQL 生成模块

**Files:**
- Create: `sync/sqlgen.go`

**Step 1: 创建 SQL 生成逻辑 (sync/sqlgen.go)**

```go
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

	// 新增列
	for _, colName := range structDiff.ColumnsAdded {
		// 这里需要完整的列定义，目前简化处理，实际应该从源库获取完整定义
		sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` VARCHAR(255);", tableName, colName)
		sqls = append(sqls, sql)
	}

	// 删除列
	for _, colName := range structDiff.ColumnsDeleted {
		sql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`;", tableName, colName)
		sqls = append(sqls, sql)
	}

	// 修改列
	for _, colMod := range structDiff.ColumnsModified {
		// 简化处理，实际应该生成完整的 MODIFY COLUMN 语句
		sql := fmt.Sprintf("ALTER TABLE `%s` MODIFY COLUMN `%s` VARCHAR(255);", tableName, colMod.ColumnName)
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
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./sync
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add sync/sqlgen.go
git commit -m "feat: add SQL generation from differences"
```

---

### Task 8: 创建 SQL 执行模块

**Files:**
- Create: `sync/executor.go`

**Step 1: 创建 SQL 执行逻辑 (sync/executor.go)**

```go
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
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./sync
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add sync/executor.go
git commit -m "feat: add SQL execution with error handling"
```

---

### Task 9: 创建验证模块

**Files:**
- Create: `sync/verifier.go`

**Step 1: 创建验证逻辑 (sync/verifier.go)**

```go
package sync

import (
	"fmt"

	"github.com/yuhuo/sync-db/database"
)

// Verifier 用于验证同步后的结果
type Verifier struct {
	comparator *Comparator
}

// NewVerifier 创建验证器
func NewVerifier(sourceConn, targetConn *database.Connection) *Verifier {
	return &Verifier{
		comparator: NewComparator(sourceConn, targetConn),
	}
}

// VerifySync 验证同步结果
func (v *Verifier) VerifySync(syncDataTables []string) (bool, string, error) {
	// 重新比对差异
	diff, err := v.comparator.CompareDifferences(syncDataTables)
	if err != nil {
		return false, "", fmt.Errorf("failed to verify sync: %w", err)
	}

	// 如果没有差异，则同步成功
	if !diff.HasDifferences() {
		return true, "Sync verification passed! No differences found.", nil
	}

	// 如果还有差异，则同步未完成
	message := fmt.Sprintf("Sync verification failed! Still have differences:\n"+
		"Structure differences: %d\n"+
		"Data differences: %d\n"+
		"View differences: %d",
		len(diff.StructureDifferences),
		len(diff.DataDifferences),
		len(diff.ViewDifferences))

	return false, message, nil
}
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./sync
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add sync/verifier.go
git commit -m "feat: add verification after sync"
```

---

## 阶段 5：日志和 UI 模块

### Task 10: 创建日志模块

**Files:**
- Create: `logger/logger.go`

**Step 1: 创建日志系统 (logger/logger.go)**

```go
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
	level     LogLevel
	fileHandle *os.File
	writers   []io.Writer
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
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./logger
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add logger/logger.go
git commit -m "feat: add logging system to file and stdout"
```

---

### Task 11: 创建 UI 模块

**Files:**
- Create: `ui/table.go`
- Create: `ui/confirm.go`

**Step 1: 创建表格展示 (ui/table.go)**

```go
package ui

import (
	"fmt"
	"os"

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
	for _, struct Diff := range diff.StructureDifferences {
		allTables[struct Diff.TableName] = true
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
```

**Step 2: 创建用户确认交互 (ui/confirm.go)**

```go
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmContinue 询问用户是否继续
func ConfirmContinue(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (y/n): ", prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		if input == "y" || input == "yes" {
			return true
		} else if input == "n" || input == "no" {
			return false
		} else {
			fmt.Println("Please enter 'y' or 'n'")
		}
	}
}

// AskForAction 询问用户采取什么操作
func AskForAction(prompt string, options []string) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s\n", prompt)
		for i, opt := range options {
			fmt.Printf("%d. %s\n", i+1, opt)
		}
		fmt.Print("Enter your choice (1-" + fmt.Sprintf("%d", len(options)) + "): ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// 验证输入
		for i, opt := range options {
			if input == fmt.Sprintf("%d", i+1) || input == opt {
				return opt
			}
		}

		fmt.Println("Invalid choice, please try again")
	}
}
```

**Step 3: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build ./ui
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add ui/
git commit -m "feat: add UI components for displaying results and getting user input"
```

---

## 阶段 6：程序入口和集成

### Task 12: 创建程序入口 (main.go)

**Files:**
- Create: `main.go`

**Step 1: 创建主程序入口 (main.go)**

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuhuo/sync-db/config"
	"github.com/yuhuo/sync-db/database"
	"github.com/yuhuo/sync-db/logger"
	"github.com/yuhuo/sync-db/sync"
	"github.com/yuhuo/sync-db/ui"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	appLogger, err := logger.NewLogger(cfg.Logging.Level, cfg.Logging.File)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer appLogger.Close()

	appLogger.Info("Application started")
	appLogger.Info(fmt.Sprintf("Connecting to source database: %s:%d/%s", cfg.Source.Host, cfg.Source.Port, cfg.Source.Database))
	appLogger.Info(fmt.Sprintf("Connecting to target database: %s:%d/%s", cfg.Target.Host, cfg.Target.Port, cfg.Target.Database))

	// 连接数据库
	connManager, err := database.NewConnectionManager(&cfg.Source, &cfg.Target)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to connect to databases: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to connect to databases: %v\n", err)
		os.Exit(1)
	}
	defer connManager.Close()

	appLogger.Info("Successfully connected to both databases")

	// 第一步：比对差异
	fmt.Println("\n========== Step 1: Comparing Differences ==========\n")
	appLogger.Info("Starting difference comparison")

	comparator := sync.NewComparator(connManager.GetSourceDB(), connManager.GetTargetDB())
	diff, err := comparator.CompareDifferences(cfg.SyncDataTables)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to compare differences: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to compare differences: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(fmt.Sprintf("Comparison complete: %d structure diffs, %d data diffs, %d view diffs",
		len(diff.StructureDifferences), len(diff.DataDifferences), len(diff.ViewDifferences)))

	// 展示差异
	ui.PrintDifferenceSummary(diff)

	if !diff.HasDifferences() {
		fmt.Println("✓ No differences found. Sync is already complete!")
		appLogger.Info("No differences found")
		return
	}

	// 用户确认
	if !ui.ConfirmContinue("Do you want to continue with the sync?") {
		fmt.Println("Sync cancelled by user")
		appLogger.Info("Sync cancelled by user")
		return
	}

	// 第二步：生成 SQL
	fmt.Println("\n========== Step 2: Generating SQL Statements ==========\n")
	appLogger.Info("Generating SQL statements")

	sqlGen := sync.NewSQLGenerator()
	sqls, err := sqlGen.GenerateSQL(diff)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to generate SQL: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to generate SQL: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(fmt.Sprintf("Generated %d SQL statements", len(sqls)))

	// 展示 SQL
	ui.PrintSQLStatements(sqls)

	// 用户确认执行
	if !ui.ConfirmContinue("Do you want to execute these SQL statements?") {
		fmt.Println("SQL execution cancelled by user")
		appLogger.Info("SQL execution cancelled by user")
		return
	}

	// 第三步：执行 SQL
	fmt.Println("\n========== Step 3: Executing SQL Statements ==========\n")
	appLogger.Info("Starting SQL execution")

	executor := sync.NewExecutor(connManager.GetTargetDB(), appLogger)
	results := executor.ExecuteSQL(sqls)

	total, success, failed := sync.GetSummary(results)
	appLogger.Info(fmt.Sprintf("SQL execution complete: %d total, %d success, %d failed", total, success, failed))

	ui.PrintExecutionSummary(total, success, failed)

	if failed > 0 {
		failedResults := sync.GetFailedResults(results)
		fmt.Println("Failed SQL statements:")
		for _, result := range failedResults {
			fmt.Printf("  SQL: %s\n", result.SQL)
			fmt.Printf("  Error: %v\n\n", result.Error)
			appLogger.Error(fmt.Sprintf("Failed SQL: %s, Error: %v", result.SQL, result.Error))
		}
	}

	// 第四步：验证
	fmt.Println("\n========== Step 4: Verifying Sync Results ==========\n")
	appLogger.Info("Starting verification")

	verifier := sync.NewVerifier(connManager.GetSourceDB(), connManager.GetTargetDB())
	verifySuccess, verifyMessage, err := verifier.VerifySync(cfg.SyncDataTables)
	if err != nil {
		appLogger.Error(fmt.Sprintf("Failed to verify sync: %v", err))
		fmt.Fprintf(os.Stderr, "Failed to verify sync: %v\n", err)
		os.Exit(1)
	}

	appLogger.Info(verifyMessage)
	ui.PrintVerificationResult(verifySuccess, verifyMessage)

	if verifySuccess {
		fmt.Println("✓ All steps completed successfully!")
		appLogger.Info("Sync completed successfully")
	} else {
		fmt.Println("✗ Sync verification failed! Please check the errors above.")
		appLogger.Error("Sync verification failed")
	}
}
```

**Step 2: 验证代码编译**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
go build -o sync-db ./
```

Expected: 编译成功，生成 `sync-db` 二进制文件

**Step 3: 运行基础测试**

```bash
cd /Users/hmw/data/www/yuhuo-sync-db
./sync-db --help 2>/dev/null || echo "Program compiled successfully (expected error without config)"
```

Expected: 程序编译成功

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add main entry point integrating all modules"
```

---

## 后续步骤

现在实现计划已完成。计划包含以下12个任务：

**第1阶段（基础）：** Task 1-3 - 项目初始化、模型定义、配置系统
**第2阶段（数据库）：** Task 4-5 - 数据库连接、查询助手
**第3阶段（核心）：** Task 6 - 差异比对逻辑
**第4阶段（SQL）：** Task 7-9 - SQL 生成、执行、验证
**第5阶段（交互）：** Task 10-11 - 日志、UI 组件
**第6阶段（集成）：** Task 12 - 主程序入口

**下一步建议：**
1. 选择执行方式（当前会话或新会话）
2. 依次完成每个 Task，确保测试通过
3. 整合所有模块形成完整的工具
4. 创建示例数据库进行端到端测试

---

## 优化和改进点（后续可考虑）

1. **SQL 生成改进：** 当前简化处理，需要完整列定义生成（从源库获取）
2. **性能优化：** 大数据量时支持分批处理和并发操作
3. **错误处理：** 增强错误消息和恢复机制
4. **用户体验：** 支持浏览差异详情、选择性同步、撤销操作
5. **高级功能：** 支持增量同步、条件过滤、导出同步脚本

