package ast

import (
	"github.com/dave/dst"
	"github.com/whatap/go-api-inst/ast/common"
)

// TypeName identifies a Go type by import path and name.
// Mirrors Datadog orchestrion's typed.TypeName.
type TypeName struct {
	ImportPath string // "" for builtins (error, string, int)
	Name       string // "Client", "Request" (pointer: "*Request")
}

// FuncSignature combines parameter count, parameter types, and return types
// into a single filter (Steps 3-4, 3-5, 3-6, 3-8 of matching-layer.md).
//
// Usage patterns:
//   - Count only: Args=nil, set MinArgs/MaxArgs → variadic/overload check
//   - Exact signature: Args + Results → type + count check
//   - Both: Args for types + MinArgs/MaxArgs for additional count constraint
type FuncSignature struct {
	Args    []TypeName // nil = skip param type check; non-nil = check count + types
	Results []TypeName // nil = skip return type check
	MinArgs int        // -1 = derive from len(Args); 0+ = minimum arg count
	MaxArgs int        // -1 = unlimited (variadic); 0+ = maximum arg count
}

// FieldMatch specifies a struct field existence check for CompositeLit matching
// (Step 3-9 of matching-layer.md).
type FieldMatch struct {
	Name     string // field name, e.g. "Transport", "Handler"
	Required bool   // true = must exist (FieldWrap), false = must not exist (FieldInsert)
}

// Rule maps a Target string to an Advice transformation.
// Target is a go/types-resolved import path + name, e.g. "database/sql.Open".
//
// Optional filter fields (nil = PASS, skip the check):
//   - Signature: param count + param types + return types (Steps 3-4/3-5/3-6/3-8)
//   - Receiver:  receiver type for method calls (Step 3-7)
//   - Fields:    struct field existence for CompositeLit (Step 3-9)
//   - Condition: custom predicate (Step 3-10)
//
// OptIn controls default registration. Zero value (false) means the rule is
// registered by default. Rules with OptIn=true are only registered when the
// user lists the rule's package path in `enabled_packages`. §242 — fmt.Print*
// is OptIn=true so high-frequency log apps (Loki, Promtail) don't pay the
// whatapfmt overhead unless the user explicitly opts in.
type Rule struct {
	Target string // e.g. "database/sql.Open", "net/http.Client{}"
	Advice Advice
	OptIn  bool // §242 — true = opt-in required via enabled_packages

	// Optional filters — nil means PASS (skip the check)
	Signature *FuncSignature           // Steps 3-4, 3-5, 3-6, 3-8
	Receiver  *TypeName                // Step 3-7
	Fields    []FieldMatch             // Step 3-9
	Condition func(*MatchContext) bool // Step 3-10
}

// MatchContext carries all context needed for an Advice transformation.
type MatchContext struct {
	File *dst.File
	Mode Mode
	// Target is the resolved target string that matched this rule.
	Target string
	Rule   *Rule

	// AST nodes (set based on what matched)
	Call *dst.CallExpr     // non-nil for function/method call matches
	Lit  *dst.CompositeLit // non-nil for composite literal matches
	Decl *dst.FuncDecl     // non-nil for function declaration matches ("decl:..." targets)

	Ident *dst.Ident        // the package identifier (for renaming)
	Sel   *dst.SelectorExpr // the selector expression

	// Block-level context (for statement insertion/replacement)
	EnclosingFunc *dst.FuncDecl // enclosing function declaration
	EnclosingStmt dst.Stmt      // enclosing statement
	ParentBlock   *[]dst.Stmt   // parent block's statement list
	StmtIndex     int           // index within parent block (-1 if N/A)

	// Resolved names
	PkgName    string // local package name used in code (may be alias)
	ImportPath string // resolved import path
	FuncName   string // function/method/type name

	// Extra imports (populated by Transform Advice for multi-import support)
	ExtraImports map[string]string // import path → alias

	// Applied indicates whether the Advice actually performed a transformation.
	// Advice types that may skip (e.g., MainInsert when not in main()) set this to false.
	// Engine checks this before collecting imports.
	Applied bool
}

// AddImport adds an import to the file.
func (ctx *MatchContext) AddImport(importPath string) {
	common.AddImport(ctx.File, importPath)
}

// AddImportWithAlias adds an import with an alias to the file.
func (ctx *MatchContext) AddImportWithAlias(importPath, alias string) {
	common.AddImportWithAlias(ctx.File, importPath, alias)
}

// §272 Phase 3 Step 4 — removed MatchContext.RemoveImportIfUnused wrapper.

// HasImport checks if the file already has the given import.
func (ctx *MatchContext) HasImport(importPath string) bool {
	return common.HasImport(ctx.File, importPath)
}
