package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldApplyInject(t *testing.T) {
	tests := []struct {
		subCmd string
		want   bool
	}{
		{"build", true},
		{"run", true},
		{"test", true},
		{"install", true},
		{"get", false},
		{"mod", false},
		{"fmt", false},
		{"vet", false},
		{"version", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.subCmd, func(t *testing.T) {
			got := shouldApplyInject(tt.subCmd)
			if got != tt.want {
				t.Errorf("shouldApplyInject(%q) = %v, want %v", tt.subCmd, got, tt.want)
			}
		})
	}
}

func TestParseOutputFlag(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantArgs   []string
	}{
		{
			"no output flag",
			[]string{"./..."},
			"",
			[]string{"./..."},
		},
		{
			"output with -o flag",
			[]string{"-o", "myapp", "."},
			"myapp",
			[]string{"."},
		},
		{
			"output with -o= flag",
			[]string{"-o=myapp", "."},
			"myapp",
			[]string{"."},
		},
		{
			"output flag in middle",
			[]string{"-v", "-o", "app", "./..."},
			"app",
			[]string{"-v", "./..."},
		},
		{
			"multiple flags",
			[]string{"-v", "-race", "-o", "binary", "./cmd/..."},
			"binary",
			[]string{"-v", "-race", "./cmd/..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOutput, gotArgs := parseOutputFlag(tt.args)
			if gotOutput != tt.wantOutput {
				t.Errorf("parseOutputFlag() output = %q, want %q", gotOutput, tt.wantOutput)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseOutputFlag() args len = %d, want %d", len(gotArgs), len(tt.wantArgs))
				return
			}
			for i, arg := range gotArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("parseOutputFlag() args[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestAdjustRelativePath(t *testing.T) {
	// Create temp directories to test with
	tmpBase := t.TempDir()
	srcDir := filepath.Join(tmpBase, "project")
	dstDir := filepath.Join(tmpBase, "tmp-build")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		relativePath string
		wantPrefix   string
	}{
		{
			"parent directory reference",
			"../go-api",
			"../",
		},
		{
			"current directory reference",
			"./local-module",
			"../",
		},
		{
			"absolute path unchanged",
			"/absolute/path",
			"/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adjustRelativePath(tt.relativePath, srcDir, dstDir)
			if !strings.HasPrefix(got, tt.wantPrefix) && got != tt.wantPrefix {
				t.Errorf("adjustRelativePath(%q) = %q, want prefix %q", tt.relativePath, got, tt.wantPrefix)
			}
		})
	}
}

func TestCopySourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	// Create source directory structure
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create main.go
	mainGo := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod
	goMod := `module test-app

go 1.21
`
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.sum
	goSum := `github.com/test/module v1.0.0 h1:hash=
`
	if err := os.WriteFile(filepath.Join(srcDir, "go.sum"), []byte(goSum), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .git directory (should be skipped)
	gitDir := filepath.Join(srcDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create random file (should be skipped)
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run copySourceFiles
	if err := copySourceFiles(srcDir, dstDir); err != nil {
		t.Fatalf("copySourceFiles() error = %v", err)
	}

	// Check main.go was copied
	if _, err := os.Stat(filepath.Join(dstDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should be copied")
	}

	// Check go.mod was copied
	if _, err := os.Stat(filepath.Join(dstDir, "go.mod")); os.IsNotExist(err) {
		t.Error("go.mod should be copied")
	}

	// Check go.sum was copied
	if _, err := os.Stat(filepath.Join(dstDir, "go.sum")); os.IsNotExist(err) {
		t.Error("go.sum should be copied")
	}

	// Check .git was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, ".git")); !os.IsNotExist(err) {
		t.Error(".git directory should not be copied")
	}

	// Check README.md was NOT copied
	if _, err := os.Stat(filepath.Join(dstDir, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md should not be copied")
	}
}

func TestCopyGoMod_WithReplace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project structure:
	// tmpDir/
	//   workspace/
	//     project/
	//       go.mod (with replace ../go-api)
	//     go-api/
	//       (the replaced module)
	//   build/
	//     (destination - different level, not sibling to project)

	workspaceDir := filepath.Join(tmpDir, "workspace")
	projectDir := filepath.Join(workspaceDir, "project")
	goApiDir := filepath.Join(workspaceDir, "go-api")
	dstDir := filepath.Join(tmpDir, "build")

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(goApiDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create go.mod with replace directive
	// From project/, ../go-api points to workspace/go-api
	goMod := `module test-app

go 1.21

require github.com/whatap/go-api v0.0.0

replace github.com/whatap/go-api => ../go-api
`
	srcGoMod := filepath.Join(projectDir, "go.mod")
	dstGoMod := filepath.Join(dstDir, "go.mod")

	if err := os.WriteFile(srcGoMod, []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy go.mod
	if err := copyGoMod(srcGoMod, dstGoMod, projectDir, dstDir); err != nil {
		t.Fatalf("copyGoMod() error = %v", err)
	}

	// Read copied go.mod
	content, err := os.ReadFile(dstGoMod)
	if err != nil {
		t.Fatalf("Failed to read copied go.mod: %v", err)
	}

	contentStr := string(content)

	// Check that the replace path was adjusted
	// From project/ (workspace/project) to build/ (tmpDir/build)
	// ../go-api from project = workspace/go-api
	// From build/, path to workspace/go-api = ../workspace/go-api
	if strings.Contains(contentStr, "=> ../go-api\n") {
		t.Error("Replace path should be adjusted for new directory location")
	}

	// The new path should reference the go-api directory correctly
	// It should contain workspace in the path since we moved to a different level
	if !strings.Contains(contentStr, "workspace/go-api") {
		t.Errorf("Adjusted path should contain 'workspace/go-api', got: %s", contentStr)
	}
}

func TestCopyGoMod_ReplaceBlock(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	dstDir := filepath.Join(tmpDir, "tmp-build")

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create go.mod with replace block
	goMod := `module test-app

go 1.21

replace (
	github.com/whatap/go-api => ../go-api
	github.com/other/module => ../other-module
)
`
	srcGoMod := filepath.Join(projectDir, "go.mod")
	dstGoMod := filepath.Join(dstDir, "go.mod")

	if err := os.WriteFile(srcGoMod, []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy go.mod
	if err := copyGoMod(srcGoMod, dstGoMod, projectDir, dstDir); err != nil {
		t.Fatalf("copyGoMod() error = %v", err)
	}

	// Read copied go.mod
	content, err := os.ReadFile(dstGoMod)
	if err != nil {
		t.Fatalf("Failed to read copied go.mod: %v", err)
	}

	// Check structure is preserved
	if !strings.Contains(string(content), "replace (") {
		t.Error("Replace block should be preserved")
	}
	if !strings.Contains(string(content), ")") {
		t.Error("Replace block closing should be preserved")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "subdir", "dest.txt")

	content := "Hello, World!"
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Destination file should exist")
	}

	// Verify content
	result, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(result) != content {
		t.Errorf("File content = %q, want %q", string(result), content)
	}
}
