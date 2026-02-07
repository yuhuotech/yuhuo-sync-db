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
