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
	Comment          *string // Column comment
}

// String 返回列的字符串表示
func (c *Column) String() string {
	if c.Length > 0 {
		return c.Name + " " + c.Type + "(" + string(rune(c.Length)) + ")"
	}
	return c.Name + " " + c.Type
}
