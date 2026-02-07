package models

// DifferenceType 表示差异的类型
type DifferenceType string

const (
	// ColumnAdded 列被添加
	ColumnAdded DifferenceType = "COLUMN_ADDED"
	// ColumnRemoved 列被删除
	ColumnRemoved DifferenceType = "COLUMN_REMOVED"
	// ColumnModified 列被修改
	ColumnModified DifferenceType = "COLUMN_MODIFIED"
	// IndexAdded 索引被添加
	IndexAdded DifferenceType = "INDEX_ADDED"
	// IndexRemoved 索引被删除
	IndexRemoved DifferenceType = "INDEX_REMOVED"
	// TableAdded 表被添加
	TableAdded DifferenceType = "TABLE_ADDED"
	// TableRemoved 表被删除
	TableRemoved DifferenceType = "TABLE_REMOVED"
	// ViewAdded 视图被添加
	ViewAdded DifferenceType = "VIEW_ADDED"
	// ViewRemoved 视图被删除
	ViewRemoved DifferenceType = "VIEW_REMOVED"
	// ViewModified 视图被修改
	ViewModified DifferenceType = "VIEW_MODIFIED"
	// CharsetModified 字符集被修改
	CharsetModified DifferenceType = "CHARSET_MODIFIED"
	// CollationModified 排序规则被修改
	CollationModified DifferenceType = "COLLATION_MODIFIED"
)

// ColumnDifference 表示列的差异
type ColumnDifference struct {
	ColumnName string
	OldValue   *Column
	NewValue   *Column
	Field      string // 被修改的字段名，例如 "Type", "Length", "IsNullable"
}

// IndexDifference 表示索引的差异
type IndexDifference struct {
	IndexName string
	OldValue  *Index
	NewValue  *Index
}

// TableDifference 表示表的差异
type TableDifference struct {
	TableName         string
	ColumnDifferences []ColumnDifference
	IndexDifferences  []IndexDifference
	CharsetChange     *string // 新的字符集
	CollationChange   *string // 新的排序规则
}

// Difference 表示两个数据库之间的单个差异
type Difference struct {
	Type                DifferenceType
	SourceDatabase      string
	TargetDatabase      string
	TableName           *string
	ViewName            *string
	ColumnName          *string
	IndexName           *string
	OldValue            interface{}
	NewValue            interface{}
	ColumnDifference    *ColumnDifference
	IndexDifference     *IndexDifference
	TableDifference     *TableDifference
	Severity            string // CRITICAL, WARNING, INFO
	Description         string
	SuggestedSQL        string // 建议的SQL语句
}

// DifferenceReport 表示差异报告
type DifferenceReport struct {
	SourceDatabase  string
	TargetDatabase  string
	Differences     []Difference
	TableDifferences map[string]*TableDifference // key是表名
	Summary         DifferenceSummary
}

// DifferenceSummary 表示差异总结
type DifferenceSummary struct {
	TotalDifferences      int
	CriticalCount         int
	WarningCount          int
	InfoCount             int
	TablesAdded           int
	TablesRemoved         int
	TablesModified        int
	ViewsAdded            int
	ViewsRemoved          int
	ViewsModified         int
	ColumnsAdded          int
	ColumnsRemoved        int
	ColumnsModified       int
	IndexesAdded          int
	IndexesRemoved        int
}

// AddDifference 添加一个差异到报告中
func (dr *DifferenceReport) AddDifference(diff Difference) {
	dr.Differences = append(dr.Differences, diff)
	if diff.Severity == "CRITICAL" {
		dr.Summary.CriticalCount++
	} else if diff.Severity == "WARNING" {
		dr.Summary.WarningCount++
	} else {
		dr.Summary.InfoCount++
	}
	dr.Summary.TotalDifferences++
}

// GetTableDifference 获取指定表的差异
func (dr *DifferenceReport) GetTableDifference(tableName string) *TableDifference {
	if dr.TableDifferences == nil {
		dr.TableDifferences = make(map[string]*TableDifference)
	}
	if _, exists := dr.TableDifferences[tableName]; !exists {
		dr.TableDifferences[tableName] = &TableDifference{
			TableName: tableName,
		}
	}
	return dr.TableDifferences[tableName]
}
