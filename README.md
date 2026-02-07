# Yuhuo Sync DB

![Go Version](https://img.shields.io/badge/Go-1.25.5-blue)
![License](https://img.shields.io/badge/license-MIT-green)

一个强大的 MySQL/MariaDB 数据库同步工具，用于在项目上线前将测试通过的数据库结构和数据与线上环境进行比对和同步。

## ✨ 功能特性

- **自动差异检测**：自动识别源库和目标库之间的所有差异
  - 表结构变更（新增列、删除列、修改列、修改索引）
  - 表数据变更（新增行、删除行、修改行）
  - 视图定义变更

- **安全的同步机制**
  - 四步工作流程，每步都有用户确认
  - 失败容错：单条 SQL 失败不会中断整体流程
  - 完整的日志记录：所有操作都被记录便于审计

- **智能 SQL 生成**
  - 自动生成 ALTER TABLE、INSERT、UPDATE、DELETE 等 SQL
  - 使用 SHOW CREATE TABLE 获取新表的完整定义
  - 包含列注释和所有元数据
  - 正确的执行顺序避免约束冲突

- **灵活的配置**
  - YAML 格式配置文件
  - 支持多表配置
  - 可选的数据同步表列表

- **完整的日志**
  - 控制台和文件双输出
  - 四个日志级别（DEBUG、INFO、WARN、ERROR）
  - 详细的执行记录和错误信息

## 📋 系统要求

- **Go**: 1.25.5 或更高版本
- **数据库**: MySQL 5.7+ 或 MariaDB 10.2+
- **网络**: 能访问源数据库和目标数据库

## 🚀 快速开始

### 1. 克隆仓库
```bash
git clone <repository-url>
cd yuhuo-sync-db
```

### 2. 下载依赖
```bash
go mod download
```

### 3. 构建项目
```bash
go build -o sync-db .
```

### 4. 配置数据库连接
复制配置模板并编辑：
```bash
cp config.yaml.example config.yaml
# 编辑 config.yaml，填入实际的数据库连接信息
```

### 5. 运行工具
```bash
./sync-db
```

或指定配置文件：
```bash
./sync-db -config /path/to/config.yaml
```

## 📖 使用说明

### 配置文件格式

```yaml
# 源数据库配置（测试/预发布环境）
source:
  host: 10.0.0.1
  port: 3306
  username: db_user
  password: db_password
  database: test_db
  charset: utf8mb4

# 目标数据库配置（线上环境）
target:
  host: 10.0.0.2
  port: 3306
  username: db_user
  password: db_password
  database: prod_db
  charset: utf8mb4

# 需要同步数据的表列表（可选）
# 注意：所有表默认都会比对结构，此列表仅用于指定需要同步数据的表
sync_data_tables:
  - users
  - orders
  - products

# 日志配置（可选）
logging:
  level: INFO      # DEBUG, INFO, WARN, ERROR
  file: sync.log   # 日志文件路径
```

### 工作流程

程序运行分为四个阶段，每个阶段都有用户确认：

#### 第一步：比对差异
```
系统自动扫描源库和目标库的差异，以表格形式展示：
- 表名
- 结构差异数（新增列、删除列、修改列等）
- 数据变更（新增行、删除行、修改行数量）
- 视图变化

用户确认是否继续
```

#### 第二步：生成 SQL 语句
```
根据检测到的差异自动生成 SQL 语句，分类展示：
- 视图 SQL（DROP VIEW 和 CREATE VIEW）
- 表结构 SQL（ALTER TABLE）
- 表数据 SQL（INSERT、UPDATE、DELETE）

用户确认是否执行这些 SQL 语句
```

#### 第三步：执行 SQL 语句
```
在目标数据库中执行生成的 SQL 语句：
- 按顺序执行每条 SQL
- 单条 SQL 失败不会中断流程，继续执行后续 SQL
- 所有错误都被记录便于后续审查
```

#### 第四步：验证同步结果
```
再次比对源库和目标库，验证同步是否完成：
- 如果不再有差异，同步成功
- 如果仍有差异，说明某些 SQL 执行失败，显示失败信息
- 展示执行摘要：总 SQL 数、成功数、失败数
```

## 🔧 高级用法

### 指定配置文件
```bash
./sync-db -config /etc/sync-db/config.yaml
```

### 日志级别控制

在 config.yaml 中设置：
```yaml
logging:
  level: DEBUG  # 详细调试信息
```

### 仅同步表结构（不同步数据）

配置文件中 `sync_data_tables` 留空：
```yaml
sync_data_tables:
  # 空列表，则不同步任何表的数据，仅比对和同步表结构
```

### 同步特定表的数据

配置文件中指定要同步的表：
```yaml
sync_data_tables:
  - users
  - orders
  # products 表只比对结构，不同步数据
```

## 📐 项目结构

```
yuhuo-sync-db/
├── README.md                 # 本文档
├── CLAUDE.md                 # Claude Code 开发指南
├── config.yaml.example       # 配置文件模板
├── go.mod                    # Go 模块定义
├── main.go                   # 程序入口
│
├── config/                   # 配置管理
│   ├── config.go             # 配置加载和验证
│   └── config_test.go        # 配置测试
│
├── database/                 # 数据库相关
│   ├── connection.go         # 连接管理和连接池
│   └── query.go              # 数据库查询（元数据、数据）
│
├── sync/                     # 同步核心逻辑
│   ├── comparator.go         # 第一步：差异比对
│   ├── sqlgen.go             # 第二步：SQL 生成
│   ├── executor.go           # 第三步：SQL 执行
│   └── verifier.go           # 第四步：验证
│
├── models/                   # 数据模型
│   ├── table.go              # 表结构模型
│   ├── column.go             # 列模型
│   ├── index.go              # 索引模型
│   ├── view.go               # 视图模型
│   └── difference.go         # 差异模型
│
├── ui/                       # 用户界面
│   ├── table.go              # 表格展示
│   └── confirm.go            # 用户确认交互
│
├── logger/                   # 日志管理
│   └── logger.go             # 日志输出（控制台+文件）
│
└── docs/                     # 文档
    └── DESIGN.md             # 设计文档
```

## ⚙️ 配置说明

### 必需配置项

| 配置项 | 说明 | 示例 |
|-------|------|------|
| `source.host` | 源数据库主机 | `10.0.0.1` |
| `source.port` | 源数据库端口 | `3306` |
| `source.username` | 源数据库用户名 | `root` |
| `source.password` | 源数据库密码 | `password` |
| `source.database` | 源数据库名 | `test_db` |
| `target.host` | 目标数据库主机 | `10.0.0.2` |
| `target.port` | 目标数据库端口 | `3306` |
| `target.username` | 目标数据库用户名 | `root` |
| `target.password` | 目标数据库密码 | `password` |
| `target.database` | 目标数据库名 | `prod_db` |

### 可选配置项

| 配置项 | 说明 | 默认值 |
|-------|------|--------|
| `source.charset` | 源数据库字符集 | `utf8mb4` |
| `target.charset` | 目标数据库字符集 | `utf8mb4` |
| `sync_data_tables` | 需要同步数据的表列表 | 空（仅同步结构） |
| `logging.level` | 日志级别 | `INFO` |
| `logging.file` | 日志文件路径 | `sync.log` |

## ⚠️ 重要限制和注意事项

### 必需条件

- **表必须有主键**：没有主键的表会跳过数据同步，仅进行结构比对
- **字符集和排序规则**：列级别的字符集/排序规则差异会被忽略，除非通过其他方式修改

### 支持的数据库对象

- ✅ 表结构（列、索引、主键、约束）
- ✅ 表数据（INSERT、UPDATE、DELETE）
- ✅ 视图定义（不同步视图数据）
- ❌ 触发器、存储过程、函数（暂不支持）
- ❌ 用户权限和角色（暂不支持）

### 执行安全性

- **单条 SQL 失败不中断**：如果某条 SQL 执行失败，系统会记录错误但继续执行后续 SQL
- **幂等性**：大多数生成的 SQL 是幂等的（例如 `DROP VIEW IF EXISTS`、`ALTER TABLE ADD COLUMN IF NOT EXISTS`）
- **备份建议**：在执行前强烈建议备份目标数据库

## 🔍 故障排除

### 连接失败

**错误信息**: `Failed to connect to databases`

**解决方案**:
1. 检查数据库主机和端口是否正确
2. 检查数据库用户名和密码是否正确
3. 检查网络连接是否正常
4. 检查数据库是否运行中

### 权限不足

**错误信息**: `Access denied for user`

**解决方案**:
1. 确保数据库用户有以下权限：
   - `SELECT` - 查询表和列信息
   - `CREATE, ALTER, DROP` - 修改表结构
   - `INSERT, UPDATE, DELETE` - 修改表数据

### SQL 执行失败

**错误信息**: `Failed to execute SQL`

**解决方案**:
1. 查看 `sync.log` 日志文件获取详细的错误信息
2. 检查生成的 SQL 语句是否有语法错误
3. 手动执行失败的 SQL 检查具体原因

## 📝 日志文件

程序运行时会生成日志文件（默认为 `sync.log`），包含：
- 程序启动和关闭时间
- 所有执行的 SQL 语句
- 每条 SQL 的执行结果和耗时
- 所有错误信息
- 程序运行总耗时

示例日志：
```
[2026-02-07 22:57:11] [INFO] Application started
[2026-02-07 22:57:11] [INFO] Connecting to source database: 10.0.0.1:3306/test_db
[2026-02-07 22:57:11] [INFO] Connecting to target database: 10.0.0.2:3306/prod_db
[2026-02-07 22:57:11] [INFO] Successfully connected to both databases
[2026-02-07 22:57:12] [INFO] Starting difference comparison
[2026-02-07 22:57:15] [INFO] Comparison complete: 5 structure diffs, 10 data diffs, 0 view diffs
```

## 🛠️ 开发指南

### 构建项目
```bash
go build -o sync-db .
```

### 运行测试
```bash
go test ./...
```

### 运行特定包的测试
```bash
go test ./config -v
```

### 生成代码覆盖率
```bash
go test ./... -cover
```

更多开发指南请参考 [CLAUDE.md](./CLAUDE.md)

## 📄 许可证

本项目采用 MIT 许可证。详见 [LICENSE](./LICENSE) 文件。

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 提交 Issue 的步骤

1. 检查是否已有相同的 Issue
2. 提供详细的错误描述
3. 附上错误日志和配置信息（隐去敏感信息）
4. 说明复现步骤

### 提交 PR 的步骤

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/your-feature`)
3. 提交更改 (`git commit -am 'Add your feature'`)
4. 推送分支 (`git push origin feature/your-feature`)
5. 开启 Pull Request

## 📞 联系方式

- 提交 Issue：[GitHub Issues](https://github.com/yuhuo/sync-db/issues)
- 发送邮件：contact@example.com

## 🙏 致谢

感谢所有贡献者和用户的支持！

---

**最后更新**: 2026-02-07
**版本**: 1.0.0
