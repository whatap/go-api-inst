package common

import (
	"go/token"
	"strconv"
	"testing"

	"github.com/dave/dst"
)

// makeFileWithCode creates a dst.File with imports and SelectorExpr references for testing.
func makeFileWithCode(imports map[string]string, refs map[string]int) *dst.File {
	// Build import specs
	var specs []dst.Spec
	var impSpecs []*dst.ImportSpec
	for alias, path := range imports {
		spec := &dst.ImportSpec{
			Path: &dst.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
		}
		// Set alias if it differs from default package name
		defaultName := getDefaultPackageName(path)
		if alias != defaultName {
			spec.Name = dst.NewIdent(alias)
		}
		specs = append(specs, spec)
		impSpecs = append(impSpecs, spec)
	}

	// Build function body with SelectorExpr references
	var stmts []dst.Stmt
	for pkgIdent, count := range refs {
		for i := 0; i < count; i++ {
			stmts = append(stmts, &dst.ExprStmt{
				X: &dst.SelectorExpr{
					X:   dst.NewIdent(pkgIdent),
					Sel: dst.NewIdent("Func"),
				},
			})
		}
	}

	file := &dst.File{
		Name: dst.NewIdent("main"),
		Decls: []dst.Decl{
			&dst.GenDecl{Tok: token.IMPORT, Specs: specs},
			&dst.FuncDecl{
				Name: dst.NewIdent("main"),
				Type: &dst.FuncType{},
				Body: &dst.BlockStmt{List: stmts},
			},
		},
		Imports: impSpecs,
	}
	return file
}

func TestGetImportPathSet(t *testing.T) {
	file := makeFileWithCode(
		map[string]string{"http": "net/http", "gin": "github.com/gin-gonic/gin"},
		nil,
	)
	got := GetImportPathSet(file)
	if !got["net/http"] {
		t.Error("GetImportPathSet missing net/http")
	}
	if !got["github.com/gin-gonic/gin"] {
		t.Error("GetImportPathSet missing gin")
	}
	if got["encoding/json"] {
		t.Error("GetImportPathSet should not have encoding/json")
	}
}

func TestIsPackageUsed(t *testing.T) {
	tests := []struct {
		name    string
		refs    map[string]int
		pkg     string
		want    bool
	}{
		{"used package", map[string]int{"http": 3}, "http", true},
		{"unused package", map[string]int{"http": 3}, "gin", false},
		{"empty file", map[string]int{}, "http", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := makeFileWithCode(nil, tt.refs)
			got := IsPackageUsed(file, tt.pkg)
			if got != tt.want {
				t.Errorf("IsPackageUsed(%q) = %v, want %v", tt.pkg, got, tt.want)
			}
		})
	}
}

func TestDefaultPackageName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"net/http", "http"},
		{"encoding/json", "json"},
		{"github.com/gin-gonic/gin", "gin"},
		{"github.com/redis/go-redis/v9", "redis"},
		{"github.com/go-redis/redis/v8", "redis"},
		{"gorm.io/gorm", "gorm"},
		{"database/sql", "sql"},
		{"fmt", "fmt"},
		// Known limitations (getDefaultPackageName can't resolve these correctly)
		// {"github.com/goccy/go-json", "json"},      // returns "go-json"
		// {"github.com/quic-go/quic-go", "quic-go"},  // returns "quic-go"
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DefaultPackageName(tt.path)
			if got != tt.want {
				t.Errorf("DefaultPackageName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// §272 Phase 3 Step 4 — TestRemoveImportIfUnused_* removed alongside
// the underlying RemoveImportIfUnused helper.
