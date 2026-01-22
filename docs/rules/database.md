# Database Transformation Rules

## database/sql

**Detection Pattern**: `sql.Open()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
```

**Transformation Rule**:
```go
// Before
db, err := sql.Open("mysql", "user:pass@/dbname")

// After
db, err := whatapsql.Open("mysql", "user:pass@/dbname")
```

> **Note**: whatapsql wraps the driver to automatically track all DB operations including Query, Exec, Prepare, Begin, etc.

---

## github.com/jmoiron/sqlx

**Detection Pattern**: `sqlx.Open()`, `sqlx.Connect()`, `sqlx.ConnectContext()`, `sqlx.MustConnect()`, `sqlx.MustOpen()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx"
```

**Transformation Rule**:
```go
// Before
db, err := sqlx.Open("mysql", "user:pass@/dbname")
db, err := sqlx.Connect("mysql", "user:pass@/dbname")
db, err := sqlx.ConnectContext(ctx, "mysql", "user:pass@/dbname")
db := sqlx.MustConnect("mysql", "user:pass@/dbname")
db := sqlx.MustOpen("mysql", "user:pass@/dbname")

// After
db, err := whatapsqlx.Open("mysql", "user:pass@/dbname")
db, err := whatapsqlx.Connect("mysql", "user:pass@/dbname")
db, err := whatapsqlx.ConnectContext(ctx, "mysql", "user:pass@/dbname")
db := whatapsqlx.MustConnect("mysql", "user:pass@/dbname")
db := whatapsqlx.MustOpen("mysql", "user:pass@/dbname")
```

> **Note**: whatapsqlx internally uses whatapsql to track all SQL queries. Errors from sqlx-specific features (Select, Get, StructScan, etc.) are collected through generic error tracing.

---

## gorm.io/gorm

**Detection Pattern**: `gorm.Open()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/go-gorm/gorm/whatapgorm"
```

**Transformation Rule**:
```go
// Before
db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})

// After
db, err := whatapgorm.Open(sqlite.Open("test.db"), &gorm.Config{})
```

---

## github.com/jinzhu/gorm (Legacy)

**Detection Pattern**: `gorm.Open()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm"
```

**Transformation Rule**:
```go
// Before
db, err := gorm.Open("mysql", "user:pass@/dbname")

// After
db, err := whatapgorm.Open("mysql", "user:pass@/dbname")
```

> **Note**: jinzhu/gorm is a legacy version. For new projects, gorm.io/gorm is recommended.

---

## Whatap Import Paths

| Original Package | Whatap Instrumentation Import |
|-----------------|------------------------------|
| `database/sql` | `.../database/sql/whatapsql` |
| `github.com/jmoiron/sqlx` | `.../jmoiron/sqlx/whatapsqlx` |
| `gorm.io/gorm` | `.../go-gorm/gorm/whatapgorm` |
| `github.com/jinzhu/gorm` | `.../jinzhu/gorm/whatapgorm` |

> **Note**: All paths are prefixed with `github.com/whatap/go-api/instrumentation/`
