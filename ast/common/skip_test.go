package common

import (
	"testing"
)

// §66: 자동 생성 파일 스킵 (.pb.go, _generated.go 등)
func TestShouldSkipFile_GeneratedFiles(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		basePath   string
		patterns   []string
		shouldSkip bool
	}{
		{
			name:       "protobuf generated file",
			filePath:   "proto/gen/user.pb.go",
			basePath:   "",
			patterns:   []string{"**/*.pb.go"},
			shouldSkip: true,
		},
		{
			name:       "grpc gateway generated file",
			filePath:   "api/handler.pb.gw.go",
			basePath:   "",
			patterns:   []string{"**/*.pb.gw.go"},
			shouldSkip: true,
		},
		{
			name:       "generic generated file",
			filePath:   "internal/model_generated.go",
			basePath:   "",
			patterns:   []string{"**/*_generated.go"},
			shouldSkip: true,
		},
		{
			name:       "normal handler file",
			filePath:   "internal/handler.go",
			basePath:   "",
			patterns:   []string{"**/*.pb.go", "**/*_generated.go"},
			shouldSkip: false,
		},
		{
			name:       "test file not excluded",
			filePath:   "internal/handler_test.go",
			basePath:   "",
			patterns:   []string{"**/*.pb.go"},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSkipFile(tt.filePath, tt.basePath, tt.patterns)
			if got != tt.shouldSkip {
				t.Errorf("§66: ShouldSkipFile(%q) = %v, want %v", tt.filePath, got, tt.shouldSkip)
			}
		})
	}
}

// §136: config exclude 옵션 — 사용자 지정 패턴
func TestShouldSkipFile_ExcludePatterns(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		basePath   string
		patterns   []string
		shouldSkip bool
	}{
		{
			name:       "vendor directory excluded",
			filePath:   "vendor/github.com/somelib/lib.go",
			basePath:   "",
			patterns:   []string{"vendor/**"},
			shouldSkip: true,
		},
		{
			name:       "proto directory excluded",
			filePath:   "proto/gen/service.go",
			basePath:   "",
			patterns:   []string{"proto/**"},
			shouldSkip: true,
		},
		{
			name:       "specific file excluded by filename pattern",
			filePath:   "internal/mock_db.go",
			basePath:   "",
			patterns:   []string{"**/mock_*.go"},
			shouldSkip: true,
		},
		{
			name:       "normal file not excluded",
			filePath:   "internal/handler.go",
			basePath:   "",
			patterns:   []string{"vendor/**", "proto/**"},
			shouldSkip: false,
		},
		{
			name:       "nil patterns uses defaults",
			filePath:   "internal/handler.go",
			basePath:   "",
			patterns:   nil,
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSkipFile(tt.filePath, tt.basePath, tt.patterns)
			if got != tt.shouldSkip {
				t.Errorf("§136: ShouldSkipFile(%q, patterns=%v) = %v, want %v",
					tt.filePath, tt.patterns, got, tt.shouldSkip)
			}
		})
	}
}

// §136: ShouldSkipFileExcludeOnly — GOROOT/GOMODCACHE 체크 없이 패턴만 확인 (§174 fast 모드)
func TestShouldSkipFileExcludeOnly(t *testing.T) {
	got := ShouldSkipFileExcludeOnly("myapp/handler.go", "", []string{"vendor/**"})
	if got {
		t.Error("ShouldSkipFileExcludeOnly should not skip normal files")
	}

	got = ShouldSkipFileExcludeOnly("vendor/lib.go", "", []string{"vendor/**"})
	if !got {
		t.Error("ShouldSkipFileExcludeOnly should skip vendor files")
	}
}

// §66: ShouldSkipDirectory — 기본 스킵 디렉토리
func TestShouldSkipDirectory(t *testing.T) {
	tests := []struct {
		name       string
		dirPath    string
		shouldSkip bool
	}{
		{"vendor dir", "myapp/vendor", true},
		{".git dir", "myapp/.git", true},
		{"node_modules dir", "myapp/node_modules", true},
		{"whatap-instrumented dir", "myapp/whatap-instrumented", true},
		{"normal dir", "myapp/internal", false},
		{"cmd dir", "myapp/cmd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldSkipDirectory(tt.dirPath, "", []string{})
			if got != tt.shouldSkip {
				t.Errorf("ShouldSkipDirectory(%q) = %v, want %v", tt.dirPath, got, tt.shouldSkip)
			}
		})
	}
}
