package models

// ViewDefinition 表示数据库视图的定义
type ViewDefinition struct {
	ViewName   string
	Definition string // CREATE VIEW 语句的内容部分
}
