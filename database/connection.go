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
