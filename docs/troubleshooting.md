# Troubleshooting Guide

This document covers common errors and solutions when using whatap-go-inst.

---

## 1. Empty File (0 bytes) Panic Error

### Symptoms
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal 0xc0000005 code=0x0 addr=0x10 pc=0x8a0f66]

goroutine 1 [running]:
go/token.(*File).Base(...)
github.com/dave/dst/decorator.(*fileDecorator).fragment.func1(...)
```

### Cause
`dst.Parse()` causes nil pointer dereference when **0-byte empty .go files** exist in the project.

### Solution
1. **Use whatap-go-inst v0.x.x or later** - Empty files are automatically skipped
2. Or delete/modify empty files:
   ```bash
   # Find empty files
   find . -name "*.go" -empty

   # Add minimal content to empty file
   echo 'package main' > empty_file.go
   ```

---

## 2. go-api Dependency Error (missing go.sum entry)

### Symptoms
```
missing go.sum entry for module providing package github.com/magiconair/properties
no required module provides package github.com/whatap/golib/util/ansi
```

### Cause
1. Project has an old version of `whatap/go-api` (e.g., v0.3.3)
2. The `golib` package referenced by `go-api` is not public

### Solution
1. **Use wrapper mode** - Latest go-api is automatically installed
   ```bash
   whatap-go-inst go build ./...
   ```

2. Or manually update go-api:
   ```bash
   go get github.com/whatap/go-api@latest
   go mod tidy
   ```

3. Use local go-api (for development):
   ```bash
   go mod edit -replace github.com/whatap/go-api=../go-api
   ```

---

## 3. expected 'package', found 'EOF' Error

### Symptoms
```
cmd/ip2asncmd/ip_index.go:1:1: expected 'package', found 'EOF'
```

### Cause
Empty .go file fails to parse in Go compiler. Go files must have a `package` declaration.

### Solution
Delete empty file or add minimal content:
```bash
# Delete
rm problematic_file.go

# Or add minimal content
echo 'package main' > problematic_file.go
```

**Note**: This error is a problem with the original project, not whatap-go-inst.

---

## 4. gorilla mux whataphttp Unused Import Error

### Symptoms
```
./main.go:7:2: "github.com/whatap/go-api/instrumentation/net/http/whataphttp" imported and not used
```

### Cause
In gorilla mux projects, the nethttp transformer detects `HandleFunc` and adds unnecessary whataphttp import.

### Solution
Use whatap-go-inst v0.x.x or later - whataphttp is excluded when gorilla mux is detected.

---

## 5. go-redis v8/v9 Type Mismatch Error

### Symptoms
```
cannot use &redis.Options{…} (value of type *"github.com/go-redis/redis/v8".Options)
as *"github.com/redis/go-redis/v9".Options value
```

### Cause
v9 whatapgoredis is injected into a go-redis v8 project.

### Solution
Use whatap-go-inst v0.x.x or later - v8/v9 is automatically detected.

---

## 6. Echo v3/v4 Type Mismatch Error

### Symptoms
```
cannot use whatapecho.Middleware() (value of type "github.com/labstack/echo/v4".MiddlewareFunc)
as "github.com/labstack/echo".MiddlewareFunc value
```

### Cause
v4 whatapecho is injected into an Echo v3 project.

### Solution
Use whatap-go-inst v0.x.x or later - v3/v4 is automatically detected.

---

## 7. Undefined Function Error

### Symptoms
```
cmd/file.go:123:12: undefined: someFunctionName
```

### Cause
Build error in the original project itself. Not a whatap-go-inst issue.

### Solution
Verify that the original project builds first:
```bash
# Test build without whatap-go-inst
go build ./...
```

---

## 8. Replace Directive Path Error

### Symptoms
```
go: ../some-local-path: no such file or directory
```

### Cause
go.mod's replace directive uses a relative path, which doesn't work when copied to a temporary directory in wrapper mode.

### Solution
whatap-go-inst automatically adjusts replace relative paths. If the problem persists:
1. Use absolute paths
2. Or use inject mode:
   ```bash
   whatap-go-inst inject -s ./myproject -o ./output
   cd output && go build ./...
   ```

---

## Frequently Asked Questions (FAQ)

### Q: What's the difference between inject and wrapper modes?
- **inject**: Transforms source files and outputs to a separate directory
- **wrapper**: Transforms in a temporary directory and builds, no changes to original source (recommended)

### Q: Are test files instrumented too?
No. `*_test.go` files are automatically skipped.

### Q: How is the vendor directory handled?
The vendor directory is copied but not instrumented.

---

## Reporting Issues

For unresolved issues, please report on GitHub Issues:
- https://github.com/whatap/go-api-inst/issues

Include the following information:
1. whatap-go-inst version
2. Go version
3. OS information
4. Full error message
5. Minimal reproducible code
