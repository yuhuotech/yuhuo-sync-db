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
