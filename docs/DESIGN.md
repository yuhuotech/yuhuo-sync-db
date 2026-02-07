# Yuhuo Sync DB - 设计文档

## 项目概述

Yuhuo Sync DB 是一个 MySQL/MariaDB 数据库同步工具，用于在项目上线前，将测试通过的测试环境/预发布环境（源数据库）的数据库结构和部分数据表数据，与线上（目标数据库）数据库进行比对、同步。

## 核心需求

### 功能需求

1. **四步工作流程**
   - 第一步：系统自动比对源库和目标库的差异，以表格形式在终端展示，用户确认
   - 第二步：将确认的差异生成 SQL 语句，提供用户二次确认
   - 第三步：在目标数据库执行 SQL，失败时记录但继续执行
   - 第四步：再次比对源数据库和目标数据库的差异，展示结果

2. **支持内容**
   - **表结构**：完整支持新增列、删除列、修改列、修改索引等
   - **表数据**：全量同步，按主键对比新增/删除/修改的行
   - **视图**：同步视图定义（不同步视图数据）

3. **配置支持**
   - YAML 配置文件
   - 配置源数据库连接信息
   - 配置目标数据库连接信息
   - 配置需要同步数据的表列表
   - 所有表默认都要比对结构

### 非功能需求

- **数据库**：MySQL/MariaDB
- **日志**：终端和文件同步输出
- **容错**：必须有主键，没有主键的表跳过数据同步；SQL 失败时记录但继续

## 架构设计

### 项目结构

```
yuhuo-sync-db/
├── main.go                      # 程序入口
├── config/
│   └── config.go                # YAML 配置加载
├── database/
│   ├── connection.go            # 数据库连接管理
│   └── query.go                 # 数据库查询逻辑
├── sync/
│   ├── comparator.go            # 第一步：差异比对
│   ├── sqlgen.go                # 第二步：生成 SQL
│   ├── executor.go              # 第三步：执行 SQL
│   └── verifier.go              # 第四步：验证
├── models/
│   ├── table.go                 # 表结构模型
│   ├── difference.go            # 差异模型
│   └── view.go                  # 视图模型
├── ui/
│   ├── table.go                 # 表格展示
│   └── confirm.go               # 用户确认交互
├── logger/
│   └── logger.go                # 日志
├── config.yaml                  # 配置示例
└── docs/
    └── DESIGN.md                # 本设计文档
```

### 核心流程

```
加载配置 → 连接数据库 → 获取元数据 → 比对差异 → 展示差异（用户确认）
→ 生成 SQL → 展示 SQL（用户确认） → 执行 SQL → 验证（再次比对）
```

## 详细设计

### 1. 数据模型

#### 表结构模型 (TableDefinition)
```
- TableName: string
- Columns: []Column
  - Name: string
  - Type: string (数据类型，如 VARCHAR, INT)
  - Length: string (可选，如 VARCHAR(255))
  - IsNullable: bool
  - DefaultValue: string (可选)
  - IsAutoIncrement: bool
  - Charset: string (可选，MySQL 特有)
  - Collation: string (可选，MySQL 特有)
- Indexes: []Index
  - Name: string
  - Type: string (PRIMARY, UNIQUE, INDEX)
  - Columns: []string
- PrimaryKey: string (主键字段名)
```

#### 表数据差异模型 (DataDifference)
```
- TableName: string
- RowsToInsert: []map[string]interface{} (新增行)
- RowsToDelete: []map[string]interface{} (删除行)
- RowsToUpdate: []UpdateRow (修改行)
  - OldValues: map[string]interface{}
  - NewValues: map[string]interface{}
  - PrimaryKeyValue: interface{}
```

#### 视图模型 (ViewDefinition)
```
- ViewName: string
- Definition: string (CREATE VIEW 语句)
```

#### 差异汇总模型 (SyncDifference)
```
- StructureDifferences: []StructureDifference
  - TableName: string
  - ColumnsAdded: []string
  - ColumnsDeleted: []string
  - ColumnsModified: []ColumnModification
  - IndexesAdded: []Index
  - IndexesDeleted: []Index
- DataDifferences: map[string]DataDifference (仅包含需要同步数据的表)
- ViewDifferences: []ViewDifference
  - ViewName: string
  - Operation: string (CREATE, DROP, MODIFY)
  - OldDefinition: string
  - NewDefinition: string
```

### 2. 差异比对逻辑

#### 表结构比对
1. 查询源库和目标库的表列表
2. 对每个表：
   - 获取列定义，对比新增/删除/修改
   - 获取索引定义，对比新增/删除/修改
3. 记录所有差异

#### 表数据比对（仅针对配置的需要同步数据的表）
1. 验证表有主键，如果没有则跳过
2. 查询源库所有行的主键值
3. 查询目标库所有行的主键值
4. 对比：
   - 新增行：源库有、目标库无
   - 删除行：目标库有、源库无
   - 修改行：都有但其他字段值不同（需要全行对比）
5. 记录所有差异

#### 视图比对
1. 查询源库所有视图的定义
2. 查询目标库所有视图的定义
3. 对比：
   - 新增视图：源库有、目标库无
   - 删除视图：目标库有、源库无
   - 修改视图：都有但定义不同

### 3. SQL 生成逻辑

#### 执行顺序
1. 删除旧视图（DROP VIEW IF EXISTS）
2. 修改表结构（ALTER TABLE）
3. 创建新视图（CREATE VIEW）
4. 修改表数据（INSERT/UPDATE/DELETE）

#### SQL 语句生成

**表结构 SQL：**
- ALTER TABLE table_name ADD COLUMN ... (新增列)
- ALTER TABLE table_name DROP COLUMN ... (删除列)
- ALTER TABLE table_name MODIFY COLUMN ... (修改列)
- ALTER TABLE table_name ADD INDEX ... (新增索引)
- ALTER TABLE table_name DROP INDEX ... (删除索引)

**表数据 SQL：**
- INSERT INTO table_name (...) VALUES (...) (新增行，可批量)
- UPDATE table_name SET ... WHERE primary_key = ... (修改行，可批量)
- DELETE FROM table_name WHERE primary_key = ... (删除行，可批量)

**视图 SQL：**
- DROP VIEW IF EXISTS view_name;
- CREATE VIEW view_name AS ...

#### 批量优化
- INSERT 语句：一次插入多条（如 INSERT VALUES (...), (...), (...)）
- UPDATE/DELETE：逐条执行（保证精准性）

### 4. SQL 执行

- 按顺序执行 SQL 语句
- 每条 SQL 独立执行
- 如果某条 SQL 失败：
  - 记录错误信息（包括 SQL、错误、行号）
  - 继续执行下一条 SQL
  - 最后汇总所有失败的 SQL

### 5. 用户交互

#### 第一步确认
- 展示表格，包含以下列：
  - 表名
  - 结构差异数
  - 新增数据行数
  - 删除数据行数
  - 修改数据行数
  - 视图变化
- 用户选择"确认继续"或"取消"
- 可选：用户可进入详情查看某个表的具体差异

#### 第二步确认
- 按类别展示 SQL（结构、数据、视图）
- 支持翻页和搜索
- 用户选择"执行"或"取消"

#### 第四步展示
- 显示最终验证结果
- 如果还有差异，说明某些 SQL 失败
- 显示执行摘要：总 SQL 数、成功数、失败数
- 列出失败的 SQL 和错误信息

### 6. 日志和输出

#### 终端输出
- 进度提示（正在连接、正在比对等）
- 确认点提示（需要用户输入的位置）
- 最终结果和摘要

#### 日志文件
- 详细日志，包括：
  - 程序启动时间
  - 所有执行的 SQL
  - 每条 SQL 的执行结果和耗时
  - 所有错误信息
  - 程序结束时间和总耗时

#### 日志级别
- ERROR：严重错误
- WARN：警告
- INFO：一般信息
- DEBUG：调试信息（可选）

## 配置文件格式

```yaml
# config.yaml

# 源数据库配置（测试/预发布环境）
source:
  host: 127.0.0.1
  port: 3306
  username: root
  password: password
  database: test_db
  charset: utf8mb4

# 目标数据库配置（线上环境）
target:
  host: 127.0.0.1
  port: 3306
  username: root
  password: password
  database: prod_db
  charset: utf8mb4

# 需要同步数据的表列表（所有表默认比对结构）
sync_data_tables:
  - table1
  - table2
  - table3

# 日志配置（可选）
logging:
  level: INFO
  file: sync.log
```

## 实现注意事项

1. **连接管理**：使用连接池，避免频繁创建/销毁连接

2. **元数据查询**：使用 MySQL 系统表（INFORMATION_SCHEMA）查询表、列、索引、视图等信息

3. **数据类型比对**：需要规范化数据类型比对（如 VARCHAR(255) 和 VARCHAR(256) 的区别）

4. **字符集和排序规则**：需要考虑字符集和排序规则的差异

5. **特殊字段处理**：
   - 自增字段：不同步自增值
   - 时间戳字段（TIMESTAMP）：需要特殊处理
   - JSON 字段：需要特殊比对逻辑

6. **大数据量处理**：
   - 数据比对时可能数据量很大，需要考虑内存效率
   - 可以分批查询和处理

7. **视图依赖**：创建新视图时，需要考虑视图的创建顺序（有依赖关系的视图）

## 测试策略

1. **单元测试**：各个模块的单独测试
2. **集成测试**：完整的四步流程测试
3. **场景测试**：
   - 新增表、新增列、新增索引
   - 删除表、删除列、删除索引
   - 修改列属性
   - 新增视图、删除视图、修改视图
   - 新增数据、删除数据、修改数据
   - 混合场景

## 后续扩展（不在当前范围内）

- 支持其他数据库（PostgreSQL、Oracle 等）
- 按条件同步数据（如按时间范围）
- 支持触发器、存储过程、函数同步
- Web UI 界面
- 增量同步（只同步变化部分）
- 并发执行优化

---

**设计时间**：2026-02-07
**版本**：1.0
