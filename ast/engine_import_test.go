package ast

import (
	"go/token"
	"testing"

	"github.com/dave/dst"
)

// TestWhatapImports_Basic tests that whatap imports are tracked correctly.
func TestWhatapImports_Basic(t *testing.T) {
	e := &Engine{
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}

	// Record a whatap import
	e.whatapImports["github.com/whatap/go-api/instrumentation/net/http/whataphttp"] = "whataphttp"
	if e.whatapImports["github.com/whatap/go-api/instrumentation/net/http/whataphttp"] != "whataphttp" {
		t.Error("whatapImports should store alias")
	}

	// Same import again — idempotent (map overwrite)
	e.whatapImports["github.com/whatap/go-api/instrumentation/net/http/whataphttp"] = "whataphttp"
	if len(e.whatapImports) != 1 {
		t.Errorf("duplicate whatapImport should be deduplicated, got %d", len(e.whatapImports))
	}
}

// TestReplacedPkgs_Basic tests that replaced packages are tracked correctly.
func TestReplacedPkgs_Basic(t *testing.T) {
	e := &Engine{
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}

	e.replacedPkgs["net/http"] = "http"
	if e.replacedPkgs["net/http"] != "http" {
		t.Error("replacedPkgs should store alias")
	}
}

// TestCollectUsedPackages tests the post-transform package usage scan.
func TestCollectUsedPackages(t *testing.T) {
	file := &dst.File{
		Name: dst.NewIdent("main"),
		Decls: []dst.Decl{
			&dst.GenDecl{Tok: token.IMPORT},
			&dst.FuncDecl{
				Name: dst.NewIdent("main"),
				Type: &dst.FuncType{},
				Body: &dst.BlockStmt{List: []dst.Stmt{
					&dst.ExprStmt{X: &dst.SelectorExpr{
						X: dst.NewIdent("http"), Sel: dst.NewIdent("StatusOK"),
					}},
					&dst.ExprStmt{X: &dst.SelectorExpr{
						X: dst.NewIdent("whataphttp"), Sel: dst.NewIdent("HttpGet"),
					}},
					&dst.ExprStmt{X: &dst.SelectorExpr{
						X: dst.NewIdent("gin"), Sel: dst.NewIdent("Default"),
					}},
				}},
			},
		},
	}

	used := collectUsedPackages(file)

	if !used["http"] {
		t.Error("collectUsedPackages should find 'http'")
	}
	if !used["whataphttp"] {
		t.Error("collectUsedPackages should find 'whataphttp'")
	}
	if !used["gin"] {
		t.Error("collectUsedPackages should find 'gin'")
	}
	if used["json"] {
		t.Error("collectUsedPackages should not find 'json'")
	}
}

// TestReplaceScenario_PartialReplace simulates: some http.X replaced, others remain.
// http.Get → whataphttp.HttpGet, but http.StatusOK remains.
// After transform: "http" still in SelectorExpr → keep import.
func TestReplaceScenario_PartialReplace(t *testing.T) {
	e := &Engine{
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}

	// Simulate transformations
	e.whatapImports["github.com/whatap/go-api/instrumentation/net/http/whataphttp"] = "whataphttp"
	e.replacedPkgs["net/http"] = "http"

	// Build a file where "http" is still used (http.StatusOK remains)
	file := makeTestFile(
		map[string]string{"net/http": ""},
		[]pkgRef{{"http", "StatusOK"}, {"whataphttp", "HttpGet"}},
	)

	used := collectUsedPackages(file)
	if !used["http"] {
		t.Error("http should still be used (StatusOK)")
	}
}

// TestReplaceScenario_FullReplace simulates: all http.X replaced.
// After transform: "http" NOT in SelectorExpr → remove import.
func TestReplaceScenario_FullReplace(t *testing.T) {
	e := &Engine{
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}

	e.whatapImports["github.com/whatap/go-api/instrumentation/net/http/whataphttp"] = "whataphttp"
	e.replacedPkgs["net/http"] = "http"

	// Build a file where "http" is NOT used anymore (only whataphttp)
	file := makeTestFile(
		map[string]string{"net/http": ""},
		[]pkgRef{{"whataphttp", "HttpGet"}},
	)

	used := collectUsedPackages(file)
	if used["http"] {
		t.Error("http should NOT be used (all replaced)")
	}
	_ = e // e would be used in resolveImports
}

// TestWrapScenario_OriginalKept simulates: gin.Default() → whatapgin.WrapEngine(gin.Default())
// gin stays, whatapgin added. No entry in replacedPkgs.
func TestWrapScenario_OriginalKept(t *testing.T) {
	e := &Engine{
		whatapImports: make(map[string]string),
		replacedPkgs:  make(map[string]string),
	}

	// WrapCall: only whatap import added, no replaced package
	e.whatapImports["github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"] = "whatapgin"
	// No entry in replacedPkgs — gin is still used

	if len(e.replacedPkgs) != 0 {
		t.Error("WrapCall should not add to replacedPkgs")
	}
	if _, ok := e.whatapImports["github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"]; !ok {
		t.Error("WrapCall should add whatap import")
	}
}

// helper types and functions for tests

type pkgRef struct {
	pkg  string
	sel  string
}

func makeTestFile(imports map[string]string, refs []pkgRef) *dst.File {
	var specs []dst.Spec
	var impSpecs []*dst.ImportSpec
	for path, alias := range imports {
		spec := &dst.ImportSpec{
			Path: &dst.BasicLit{Kind: token.STRING, Value: `"` + path + `"`},
		}
		if alias != "" {
			spec.Name = dst.NewIdent(alias)
		}
		specs = append(specs, spec)
		impSpecs = append(impSpecs, spec)
	}

	var stmts []dst.Stmt
	for _, ref := range refs {
		stmts = append(stmts, &dst.ExprStmt{
			X: &dst.SelectorExpr{
				X:   dst.NewIdent(ref.pkg),
				Sel: dst.NewIdent(ref.sel),
			},
		})
	}

	return &dst.File{
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
}
