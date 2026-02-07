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
	Source         DatabaseConfig `yaml:"source"`
	Target         DatabaseConfig `yaml:"target"`
	SyncDataTables []string       `yaml:"sync_data_tables"`
	Logging        LoggingConfig  `yaml:"logging"`
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
