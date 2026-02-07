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
			// 目标库中不存在这个表，这是一个新表
			diff.IsNewTable = true
			diff.TableDefinition = sourceDef
			diff.ColumnsAdded = sourceDef.Columns
			diff.IndexesAdded = sourceDef.Indexes
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
	added   []models.Column
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

	var added []models.Column
	var deleted []string
	var modifications []models.ColumnModification

	// 检查新增和修改的列
	for _, sourceCol := range sourceColumns {
		if _, exists := targetColMap[sourceCol.Name]; !exists {
			added = append(added, sourceCol)
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
		added   []models.Column
		deleted []string
	}{added, deleted}, modifications
}

// columnsEqual 判断两个列是否相等
// 注意：忽略字符集和排序规则的差异，因为这通常不需要修改列定义
func columnsEqual(col1, col2 models.Column) bool {
	// 比对基本属性
	if col1.Name != col2.Name {
		return false
	}

	// 规范化类型进行比对（忽略长度括号）
	type1 := normalizeType(col1.Type)
	type2 := normalizeType(col2.Type)
	if type1 != type2 {
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

	// 注意：NOT 比对 Charset 和 Collation，因为这些差异通常不需要修改列
	// 如果用户需要修改字符集，应该单独使用 ALTER TABLE ... CONVERT TO CHARSET

	return true
}

// normalizeType 规范化列类型，忽略长度参数但保留修饰符
func normalizeType(typeStr string) string {
	// 转换为大起进行比对
	typeStr = strings.ToUpper(typeStr)

	// 移除括号内的内容，但保留括号后面的修饰符
	// 例如：VARCHAR(255) → VARCHAR，TINYINT(1) UNSIGNED → TINYINT UNSIGNED
	idx := strings.Index(typeStr, "(")
	endIdx := strings.Index(typeStr, ")")
	if idx > 0 && endIdx > idx {
		// 去掉括号及其内容，但保留后面的部分（如 UNSIGNED）
		typeStr = typeStr[:idx] + typeStr[endIdx+1:]
	}

	// 清除多余的空格
	typeStr = strings.TrimSpace(typeStr)
	// 将多个空格替换为单个空格
	typeStr = strings.Join(strings.Fields(typeStr), " ")

	return typeStr
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
