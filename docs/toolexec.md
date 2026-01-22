# toolexec Mode

Uses Go compiler's `-toolexec` flag to transform code directly in the compile pipeline.

## Overview

```bash
go build -toolexec="whatap-go-inst toolexec" ./...
```

## How It Works

### Go Compilation Process

Go build internally calls several tools:

```
go build
    │
    ├── compile (*.go → *.o)
    ├── asm (*.s → *.o)
    └── link (*.o → executable)
```

### -toolexec Flag

The `-toolexec` flag runs the specified program before these tools are invoked:

```
go build -toolexec="whatap-go-inst toolexec"
    │
    ├── whatap-go-inst toolexec compile *.go → *.o
    │       │
    │       ├── 1. Analyze source files
    │       ├── 2. AST transformation (inject instrumentation)
    │       ├── 3. Create temporary files
    │       └── 4. Call original compile (with transformed source)
    │
    ├── whatap-go-inst toolexec asm ... (pass through)
    └── whatap-go-inst toolexec link ... (pass through)
```

### Detailed Flow

```
┌────────────────────────────────────────────────────────────────┐
│ Go compiler invocation                                          │
│ whatap-go-inst toolexec /path/to/compile -o main.a main.go     │
└────────────────────────────┬───────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│ 1. Tool identification                                          │
│    - Is it compile tool? → Yes: Perform AST transformation     │
│    - Other tool? → Pass through                                │
└────────────────────────────┬───────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│ 2. Source file filtering                                        │
│    - File in GOROOT? → Skip (standard library)                 │
│    - File in GOMODCACHE? → Skip (external package)             │
│    - _test.go? → Skip                                          │
│    - User code? → Transform target                             │
└────────────────────────────┬───────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│ 3. AST transformation                                           │
│    - Parse file                                                 │
│    - Detect frameworks (gin, echo, etc.)                       │
│    - Add imports                                                │
│    - Insert main() initialization code                         │
│    - Output to temporary file                                  │
└────────────────────────────┬───────────────────────────────────┘
                             │
                             ▼
┌────────────────────────────────────────────────────────────────┐
│ 4. Call original compiler                                       │
│    /path/to/compile -o main.a /tmp/transformed/main.go         │
└────────────────────────────────────────────────────────────────┘
```

## Usage

### Prerequisites

Before using toolexec mode, initialize and add dependencies:

```bash
# 1. Initialize (once)
whatap-go-inst init

# 2. Add dependencies (required!)
go get github.com/whatap/go-api@latest
go mod tidy
```

### Basic Usage

```bash
# Build
go build -toolexec="whatap-go-inst toolexec" ./...

# Specify output file
go build -toolexec="whatap-go-inst toolexec" -o myapp .

# Run
go run -toolexec="whatap-go-inst toolexec" .

# Test
go test -toolexec="whatap-go-inst toolexec" ./...
```

### Debug Output

```bash
GO_API_AST_DEBUG=1 go build -toolexec="whatap-go-inst toolexec" ./...
```

Example output:
```
[whatap-go-inst] output: /var/folders/.../b001/exe/main
[whatap-go-inst] transformed: [/var/folders/.../whatap-go-inst-123/main.go]
```

### Shell Aliases

**bash/zsh:**
```bash
# ~/.bashrc or ~/.zshrc
alias gobuild='go build -toolexec="whatap-go-inst toolexec"'
alias gorun='go run -toolexec="whatap-go-inst toolexec"'
alias gotest='go test -toolexec="whatap-go-inst toolexec"'
```

Usage:
```bash
gobuild ./...
gorun .
gotest ./...
```

## Code Analysis

### cmd/toolexec.go Core Logic

```go
// Check if compile tool
func isCompileTool(tool string) bool {
    base := filepath.Base(tool)
    return base == "compile" || base == "compile.exe"
}

// Process compile arguments
func processCompileArgs(args []string) []string {
    var goFiles []string

    // 1. Extract Go files
    for _, arg := range args {
        if strings.HasSuffix(arg, ".go") {
            goFiles = append(goFiles, arg)
        }
    }

    // 2. Transform each file
    for _, goFile := range goFiles {
        if shouldSkipFile(goFile) {
            continue // Skip standard library, external packages
        }

        // AST transformation
        injector.InjectFile(goFile, tmpFile)
    }

    // 3. Replace arguments with transformed files
    return newArgs
}
```

### Skip Conditions

```go
func shouldSkipFile(path string) bool {
    // 1. Standard library (GOROOT)
    if strings.HasPrefix(path, os.Getenv("GOROOT")) {
        return true
    }

    // 2. External packages (GOMODCACHE)
    if strings.HasPrefix(path, gomodcache) {
        return true
    }

    // 3. Test files
    if strings.HasSuffix(path, "_test.go") {
        return true
    }

    return false
}
```

## Advantages

| Advantage | Description |
|-----------|-------------|
| **Preserves original** | Source code is not modified at all |
| **Fine-grained control** | Can combine freely with Go build flags |
| **Standard approach** | Uses Go's official extension mechanism |

## Disadvantages

| Disadvantage | Description |
|--------------|-------------|
| **Long command** | Need to type `-toolexec` flag every time |
| **Build cache caution** | Not applied to cached builds |

## Build Cache Handling

Results built with toolexec are cached, but don't share cache with regular `go build`:

```bash
# First build (slow)
go build -toolexec="whatap-go-inst toolexec" ./...

# Second build (uses cache, fast)
go build -toolexec="whatap-go-inst toolexec" ./...

# Regular build (separate cache)
go build ./...
```

To completely clear the cache:
```bash
go clean -cache
```

## Combining with Other -toolexec Tools

To combine multiple toolexec tools, create a wrapper script:

**toolexec-chain.sh:**
```bash
#!/bin/bash
# Run first tool
whatap-go-inst toolexec "$@"
```

```bash
go build -toolexec="./toolexec-chain.sh" ./...
```

## Important Notes

1. **CGO support limitation**: When CGO is used, C compiler is not transformed.
2. **Cross compilation**: Works even when `GOOS`/`GOARCH` are different.
3. **Build mode**: Can be used with `-buildmode` flag.

## Troubleshooting

### Transformation not applied

```bash
# Clear cache and rebuild
go clean -cache
GO_API_AST_DEBUG=1 go build -toolexec="whatap-go-inst toolexec" ./...
```

### Compile error occurs

```bash
# Check transformed source
GO_API_AST_DEBUG=1 go build -toolexec="whatap-go-inst toolexec" ./... 2>&1 | grep transformed
# Examine the temporary file contents
```

## Next Steps

- [Build Wrapper Mode](./build-wrapper.md) - Simpler usage
- [go:generate Mode](./go-generate.md)
- [Direct Source Modification](./source-inject.md)
