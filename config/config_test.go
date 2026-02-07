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
