# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Yuhuo Sync DB** is a MySQL/MariaDB database synchronization tool for comparing and syncing database schemas and data between environments (e.g., staging to production) before project deployment.

**Core workflow:**
1. Compare differences between source (test/staging) and target (production) databases
2. Generate SQL statements for identified differences
3. Execute SQL statements in target database with error resilience
4. Verify the synchronization by comparing again

**Scope:** Table structures (columns, indexes), table data (rows), and view definitions.

## Build and Run

### Build
```bash
go build -o sync-db .
```

### Run
```bash
./sync-db -config config.yaml
```

**Configuration:**
- Create `config.yaml` from `config.yaml.example`
- Specify source database (test/staging), target database (production), and tables to sync data for
- All tables default to structure comparison; only specified tables get data synced

### Dependencies
```bash
go mod download
```

Key dependencies:
- `github.com/go-sql-driver/mysql` - MySQL driver
- `gopkg.in/yaml.v2` - YAML config parsing

## Architecture

### Four-Phase Workflow

```
Phase 1: Comparison      → Get metadata from source & target DBs, identify differences
              ↓
Phase 2: SQL Generation  → Generate ALTER/CREATE/INSERT/UPDATE/DELETE statements
              ↓
Phase 3: Execution       → Execute SQL statements with error tracking (continue on failure)
              ↓
Phase 4: Verification    → Re-compare to confirm synchronization succeeded
```

### Package Organization

| Package | Responsibility |
|---------|---|
| `main.go` | Application entry point, orchestrates the four phases |
| `config/` | YAML configuration loading and validation |
| `database/` | MySQL connections, metadata queries (INFORMATION_SCHEMA) |
| `sync/` | Core logic: comparator, SQL generator, executor, verifier |
| `models/` | Data structures for tables, differences, views |
| `ui/` | Terminal output (tables, confirmations) |
| `logger/` | Dual-output logging (console + file) |

### Key Components

**Comparator** (`sync/comparator.go`):
- Compares table structures using INFORMATION_SCHEMA queries
- Identifies added/modified/deleted columns and indexes
- Compares table data by primary key (insert/update/delete detection)
- Compares view definitions
- **Type normalization:** Handles MySQL type variations (e.g., `INT` vs `INT(11)`, `TINYINT(1) UNSIGNED` vs `TINYINT UNSIGNED`)
- **Column comparison:** Detects differences in nullable, default values, auto-increment, and comments (ignores charset/collation differences)

**SQL Generator** (`sync/sqlgen.go`):
- Generates SQL in execution order: DROP VIEW → ALTER TABLE → CREATE VIEW → INSERT/UPDATE/DELETE
- **New tables:** Uses `SHOW CREATE TABLE` to get original CREATE TABLE statement
- **Column definitions:** Includes all attributes including COMMENT field
- **Value escaping:** Safely escapes SQL values for strings, numbers, booleans

**Executor** (`sync/executor.go`):
- Executes SQL statements in order
- Continues on failure, logs errors with context
- Tracks execution results for verification

**Verifier** (`sync/verifier.go`):
- Re-runs comparison after execution
- Provides summary: total SQL statements, successful, failed
- Lists failed SQL statements with error messages

### Data Models

**StructureDifference** (`models/difference.go`):
- Tracks table structure changes: added/deleted/modified columns, indexes
- For new tables: stores complete `TableDefinition` with all columns and metadata
- `IsNewTable` flag indicates CREATE TABLE vs ALTER TABLE approach

**DataDifference** (`models/difference.go`):
- Tracks row-level changes: `RowsToInsert`, `RowsToDelete`, `RowsToUpdate`
- Only populated for tables in `sync_data_tables` config
- Requires primary key (tables without primary key skip data sync)

**Column** (`models/column.go`):
- Stores complete column metadata: type, nullable, default value, auto-increment, charset, collation, **comment**
- Used by comparator and SQL generator

### Database Querying

**QueryHelper** (`database/query.go`):
- `GetTables()` - Returns base tables (excludes views via TABLE_TYPE filter)
- `GetTableDefinition()` - Returns columns, indexes, primary key
- `getColumns()` - Queries INFORMATION_SCHEMA.COLUMNS including COLUMN_COMMENT
- `getIndexes()` - Queries INFORMATION_SCHEMA.STATISTICS
- `GetViews()` - Returns view definitions
- `GetPrimaryKeyValues()` - Gets all primary key values for a table
- `GetRowByPrimaryKey()` - Fetches complete row data by PK
- `GetCreateTableSQL()` - Executes SHOW CREATE TABLE for new table creation

### Important Implementation Details

1. **Type Normalization** (`sync/comparator.go:normalizeType()`):
   - Removes length parameters from types while preserving modifiers
   - `VARCHAR(255)` → `VARCHAR`
   - `TINYINT(1) UNSIGNED` → `TINYINT UNSIGNED`
   - Handles multi-modifier types with preserved order

2. **Column Equality** (`sync/comparator.go:columnsEqual()`):
   - Compares: name, normalized type, nullable, default value, auto-increment, **comment**
   - **Ignores:** charset and collation (not included in MODIFY COLUMN SQL)

3. **View Comparison** (`sync/comparator.go:normalizeViewDefinition()`):
   - Normalizes whitespace and line breaks for reliable comparison
   - Case-insensitive comparison

4. **SQL Generation Order:**
   - DROP VIEWs first (may have dependencies)
   - ALTER TABLE (structure changes)
   - CREATE VIEWs
   - INSERT/UPDATE/DELETE (data changes)
   - Order prevents constraint violations and circular dependencies

5. **Error Handling:**
   - SQL execution continues on failure
   - Errors logged with SQL, error message, and context
   - Fourth phase verification reveals which changes succeeded

6. **Configuration Validation:**
   - Required: source/target host, port, username, password, database
   - charset defaults to utf8mb4
   - sync_data_tables is optional (empty = no data sync)

## Common Tasks

### Build and Test
```bash
# Build
go build -o sync-db .

# Run basic tests
go test ./config -v
go test ./... -v

# Test type normalization (for debugging column type issues)
go run test_normalize.go
```

### Run with Configuration
```bash
# Ensure config.yaml exists with correct database credentials
./sync-db

# Or specify custom config path
./sync-db -config my-config.yaml
```

### Debug Specific Table
Create a test file to query specific tables:
```go
qh := database.NewQueryHelper(conn)
tableDef, _ := qh.GetTableDefinition("table_name")
// Inspect tableDef.Columns for metadata
```

## Key Assumptions and Constraints

1. **Primary Key Required:** Tables without primary key skip data synchronization
2. **Charset Differences Ignored:** Minor charset/collation differences don't trigger MODIFY
3. **Fail-Safe SQL Execution:** Individual SQL failures don't stop the process
4. **View Definition Only:** Only view definitions sync, not view data
5. **Connection Pooling:** Database connections use pooling (10 max open, 5 max idle)

## Recent Improvements

- **SHOW CREATE TABLE**: New tables now use `SHOW CREATE TABLE` query instead of manual assembly, preserving all original schema details
- **Column Comments**: Column comments (COMMENT field) now tracked and included in MODIFY COLUMN statements
- **Type Modifiers**: Type normalization correctly preserves modifiers like UNSIGNED, BINARY, etc.

## Files Not in Version Control

- `sync-db` - Compiled binary (generated by `go build`)
- `config.yaml` - Contains database credentials (use `config.yaml.example` as template)
- `sync.log` - Runtime log file
- `.env` files - Environment-specific configuration

See `.gitignore` for complete list.
