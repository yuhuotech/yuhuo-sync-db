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
