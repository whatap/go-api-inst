package ast

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// rulesYAMLEnvVar lets developers swap the built-in rules at runtime by
// pointing at an external rules.yaml. Production builds **never** read this
// file — Go-coded `AllRules()` is the canonical source.
const rulesYAMLEnvVar = "WHATAP_RULES_YAML"

// RulesConfig is the top-level schema for a rules YAML file
// (internal rules.yaml or a user custom rules file).
//
// Spec: dev-docs/design/v2/rule-yaml-schema.md §3.
type RulesConfig struct {
	Version       int               `yaml:"version"`
	Imports       []string          `yaml:"imports,omitempty"`
	ImportAliases map[string]string `yaml:"importAliases,omitempty"` // alias → full path
	Rules         []RuleSpec        `yaml:"rules"`
}

// RuleSpec is one yaml rule entry. The subset of fields used depends on Type.
type RuleSpec struct {
	ID     string `yaml:"id,omitempty"`
	Type   string `yaml:"type"`
	Target string `yaml:"target,omitempty"`
	OptIn  bool   `yaml:"optin,omitempty"` // §242 — true = opt-in via enabled_packages

	// Common (most types): "pkg.Func" — resolved through importAliases.
	With string `yaml:"with,omitempty"`

	// type=arg-wrap
	ArgIndex *int `yaml:"argIndex,omitempty"`

	// type=arg-insert
	WhatapAlias string             `yaml:"whatapAlias,omitempty"`
	InsertArgs  []InsertedArgSpec  `yaml:"insertArgs,omitempty"`
	Ellipsis    bool               `yaml:"ellipsis,omitempty"`

	// type=code-insert
	Position   string `yaml:"position,omitempty"`
	ArgSource  *int   `yaml:"argSource,omitempty"`
	MethodName string `yaml:"methodName,omitempty"`

	// type=main-insert
	OrigPkg     string `yaml:"origPkg,omitempty"`
	WrapExpr    string `yaml:"wrapExpr,omitempty"`
	ExtraImport string `yaml:"extraImport,omitempty"`

	// type=field-wrap / field-wrap-or-insert
	FieldName  string `yaml:"fieldName,omitempty"`
	WrapWith   string `yaml:"wrapWith,omitempty"`
	InsertWith string `yaml:"insertWith,omitempty"`
	CtxAware   bool   `yaml:"ctxAware,omitempty"`

	// type=transform
	Template string `yaml:"template,omitempty"`
	// §272 Phase 3 Step 4 + B 안 (2026-05-19): `reverseTarget` field removed.
	// Strict yaml decoder now rejects this key — see issue 272.

	// type=hook
	Before string `yaml:"before,omitempty"`
	After  string `yaml:"after,omitempty"`

	// type=inject
	Start string `yaml:"start,omitempty"`
	End   string `yaml:"end,omitempty"`

	// Rule-level overrides of global imports / importAliases.
	Imports       []string          `yaml:"imports,omitempty"`
	ImportAliases map[string]string `yaml:"importAliases,omitempty"`

	// Optional precise-match filters
	Signature *SignatureSpec    `yaml:"signature,omitempty"`
	Receiver  *ReceiverSpec     `yaml:"receiver,omitempty"`
	Fields    []FieldMatchSpec  `yaml:"fields,omitempty"`
}

// InsertedArgSpec mirrors InsertedArg for arg-insert.
type InsertedArgSpec struct {
	WrapFunc  string `yaml:"wrapFunc"`
	InnerFunc string `yaml:"innerFunc"`
}

// SignatureSpec maps to FuncSignature.
type SignatureSpec struct {
	Args    []TypeNameSpec `yaml:"args,omitempty"`
	Results []TypeNameSpec `yaml:"results,omitempty"`
	MinArgs *int           `yaml:"minArgs,omitempty"`
	MaxArgs *int           `yaml:"maxArgs,omitempty"`
}

// TypeNameSpec maps to TypeName.
type TypeNameSpec struct {
	Package string `yaml:"package,omitempty"`
	Name    string `yaml:"name"`
}

// ReceiverSpec maps to TypeName for receivers.
type ReceiverSpec struct {
	Package string `yaml:"package,omitempty"`
	Name    string `yaml:"name"`
}

// FieldMatchSpec maps to FieldMatch.
type FieldMatchSpec struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
}

// LoadBuiltinRules returns the 92 internal rules.
//
// Source-of-truth precedence (§227 Step 5 revision):
//  1. Go-coded AllRules() — **canonical**. Production builds always use this.
//  2. $WHATAP_RULES_YAML environment variable — **developer-only override**.
//     If set to a readable file and the yaml decodes cleanly, the yaml
//     replaces AllRules() in full for that process. Used to iterate on rule
//     changes without recompiling.
//
// rules.yaml in the repo is a developer reference + diff-test fixture; it is
// NOT embedded in the binary and is NOT loaded by default.
func LoadBuiltinRules() []*Rule {
	if path := os.Getenv(rulesYAMLEnvVar); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] %s=%q: %v — falling back to built-in rules\n",
				rulesYAMLEnvVar, path, err)
			return AllRules()
		}
		rules, err := DecodeRulesYAML(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] %s=%q decode error: %v — falling back to built-in rules\n",
				rulesYAMLEnvVar, path, err)
			return AllRules()
		}
		return rules
	}
	return AllRules()
}

// DecodeRulesYAML parses a rules YAML document into []*Rule using the shared schema.
func DecodeRulesYAML(data []byte) ([]*Rule, error) {
	var cfg RulesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("rules yaml: %w", err)
	}
	return BuildRules(&cfg)
}

// BuildRules converts a decoded RulesConfig into []*Rule.
func BuildRules(cfg *RulesConfig) ([]*Rule, error) {
	if cfg == nil {
		return nil, nil
	}
	rules := make([]*Rule, 0, len(cfg.Rules))
	for i, spec := range cfg.Rules {
		r, err := buildRule(cfg, &spec)
		if err != nil {
			return nil, fmt.Errorf("rules[%d] (%s %q): %w", i, spec.Type, spec.Target, err)
		}
		if r != nil {
			rules = append(rules, r)
		}
	}
	return rules, nil
}

// mergeAliases returns a new map = global ∪ rule-level (rule wins on conflict).
func mergeAliases(global, local map[string]string) map[string]string {
	if len(global) == 0 && len(local) == 0 {
		return nil
	}
	merged := make(map[string]string, len(global)+len(local))
	for k, v := range global {
		merged[k] = v
	}
	for k, v := range local {
		merged[k] = v
	}
	return merged
}

// resolveAlias looks up "alias" in the merged alias map. Returns "" if unknown.
func resolveAlias(alias string, aliases map[string]string) string {
	if alias == "" || aliases == nil {
		return ""
	}
	return aliases[alias]
}

// splitWith parses a "pkg.Func" string into ("pkg", "Func").
// If only one component is present, returns ("", s).
func splitWith(s string) (alias, fn string, err error) {
	if s == "" {
		return "", "", fmt.Errorf(`"with" is required`)
	}
	idx := strings.LastIndex(s, ".")
	if idx < 0 {
		return "", "", fmt.Errorf(`"with" must be "alias.Func", got %q`, s)
	}
	return s[:idx], s[idx+1:], nil
}

// normalizeTarget strips yaml-only sugar prefixes (call:, lit:) and returns the
// internal target string expected by Registry and resolve.go.
func normalizeTarget(t string) string {
	switch {
	case strings.HasPrefix(t, "call:"):
		return strings.TrimPrefix(t, "call:")
	case strings.HasPrefix(t, "lit:"):
		return strings.TrimPrefix(t, "lit:")
	default:
		return t
	}
}

// parseReplaceWithCtxTarget splits a target like "net/http.Get" or
// "net/http.DefaultClient.Get" into (pkgFunc, origVar, origFunc).
//   - "pkg.Func"         → ("pkg.Func", "", "Func")
//   - "pkg.Var.Func"     → ("pkg.Var.Func", "Var", "Func")
func parseReplaceWithCtxTarget(target string) (origVar, origFunc string) {
	// Find the last "/" — everything after is "pkgName.(Var.)?Func"
	slash := strings.LastIndex(target, "/")
	tail := target
	if slash >= 0 {
		tail = target[slash+1:]
	}
	parts := strings.Split(tail, ".")
	switch len(parts) {
	case 2: // pkg.Func
		return "", parts[1]
	case 3: // pkg.Var.Func
		return parts[1], parts[2]
	default:
		return "", ""
	}
}

// ptrInt returns a default int value from a pointer, or def if nil.
func ptrInt(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

// buildSignature converts the yaml SignatureSpec into *FuncSignature.
func buildSignature(spec *SignatureSpec) *FuncSignature {
	if spec == nil {
		return nil
	}
	sig := &FuncSignature{
		MinArgs: ptrInt(spec.MinArgs, -1),
		MaxArgs: ptrInt(spec.MaxArgs, -1),
	}
	for _, a := range spec.Args {
		sig.Args = append(sig.Args, TypeName{ImportPath: a.Package, Name: a.Name})
	}
	for _, r := range spec.Results {
		sig.Results = append(sig.Results, TypeName{ImportPath: r.Package, Name: r.Name})
	}
	return sig
}

// buildReceiver converts the yaml ReceiverSpec into *TypeName.
func buildReceiver(spec *ReceiverSpec) *TypeName {
	if spec == nil {
		return nil
	}
	return &TypeName{ImportPath: spec.Package, Name: spec.Name}
}

// buildFields converts the yaml FieldMatchSpec list into []FieldMatch.
func buildFields(specs []FieldMatchSpec) []FieldMatch {
	if len(specs) == 0 {
		return nil
	}
	out := make([]FieldMatch, 0, len(specs))
	for _, f := range specs {
		out = append(out, FieldMatch{Name: f.Name, Required: f.Required})
	}
	return out
}

// reverseAliases converts alias→path into path→alias (used by Transform).
func reverseAliases(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for alias, path := range in {
		out[path] = alias
	}
	return out
}

// buildRule dispatches on spec.Type and produces a single Rule.
func buildRule(cfg *RulesConfig, spec *RuleSpec) (*Rule, error) {
	target := normalizeTarget(spec.Target)
	aliases := mergeAliases(cfg.ImportAliases, spec.ImportAliases)

	rule := &Rule{
		Target:    target,
		OptIn:     spec.OptIn,
		Signature: buildSignature(spec.Signature),
		Receiver:  buildReceiver(spec.Receiver),
		Fields:    buildFields(spec.Fields),
	}

	switch spec.Type {
	case "replace":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		rule.Advice = &ReplaceFunction{WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn}

	case "replace-with-ctx":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		origVar, origFunc := parseReplaceWithCtxTarget(target)
		if origFunc == "" {
			return nil, fmt.Errorf("replace-with-ctx target must have at least pkg.Func")
		}
		rule.Advice = &ReplaceWithCtx{
			WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn,
			OrigVar: origVar, OrigFunc: origFunc,
		}

	case "wrap-call":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		rule.Advice = &WrapCall{WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn}

	case "arg-wrap":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		rule.Advice = &ArgWrap{
			WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn,
			ArgIndex: ptrInt(spec.ArgIndex, -1),
		}

	case "arg-insert":
		if spec.WhatapAlias == "" {
			return nil, fmt.Errorf("arg-insert requires whatapAlias")
		}
		pkg := resolveAlias(spec.WhatapAlias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q", spec.WhatapAlias)
		}
		args := make([]InsertedArg, 0, len(spec.InsertArgs))
		for _, ia := range spec.InsertArgs {
			args = append(args, InsertedArg{WrapFunc: ia.WrapFunc, InnerFunc: ia.InnerFunc})
		}
		rule.Advice = &ArgInsert{
			WhatapPkg: pkg, WhatapAlias: spec.WhatapAlias,
			InsertArgs: args, Ellipsis: spec.Ellipsis,
		}

	case "code-insert":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		if spec.Position != "before" && spec.Position != "after" {
			return nil, fmt.Errorf(`code-insert position must be "before" or "after"`)
		}
		rule.Advice = &CodeInsert{
			WhatapPkg: pkg, WhatapAlias: alias,
			Position: spec.Position, ArgSource: ptrInt(spec.ArgSource, 0),
			MethodName: spec.MethodName, WhatapFunc: fn,
		}

	case "main-insert":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		origPkg := spec.OrigPkg
		_, origFunc := parseReplaceWithCtxTarget(target)
		advice := &MainInsert{
			WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn,
			ExtraImport: spec.ExtraImport, WrapExpr: spec.WrapExpr,
			OrigPkgAlias: origPkg, OrigFunc: origFunc,
		}
		rule.Advice = advice

	case "field-wrap":
		alias, fn, err := splitWith(spec.With)
		if err != nil {
			return nil, err
		}
		pkg := resolveAlias(alias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q for %q", alias, spec.With)
		}
		if spec.FieldName == "" {
			return nil, fmt.Errorf("field-wrap requires fieldName")
		}
		rule.Advice = &FieldWrap{
			WhatapPkg: pkg, WhatapAlias: alias, WhatapFunc: fn,
			FieldName: spec.FieldName, CtxAware: spec.CtxAware,
		}

	case "field-wrap-or-insert":
		wrapAlias, wrapFn, err := splitWith(spec.WrapWith)
		if err != nil {
			return nil, fmt.Errorf("field-wrap-or-insert wrapWith: %w", err)
		}
		insAlias, insFn, err := splitWith(spec.InsertWith)
		if err != nil {
			return nil, fmt.Errorf("field-wrap-or-insert insertWith: %w", err)
		}
		if wrapAlias != insAlias {
			return nil, fmt.Errorf("field-wrap-or-insert wrapWith/insertWith must share the same alias, got %q vs %q", wrapAlias, insAlias)
		}
		pkg := resolveAlias(wrapAlias, aliases)
		if pkg == "" {
			return nil, fmt.Errorf("unknown importAlias %q", wrapAlias)
		}
		if spec.FieldName == "" {
			return nil, fmt.Errorf("field-wrap-or-insert requires fieldName")
		}
		rule.Advice = &FieldWrapOrInsert{
			WhatapPkg: pkg, WhatapAlias: wrapAlias,
			WrapFunc: wrapFn, InsertFunc: insFn,
			FieldName: spec.FieldName, CtxAware: spec.CtxAware,
		}

	case "transform":
		// imports: list of import paths. Resolve aliases via the shared map
		// and convert to internal path→alias form.
		paths := append([]string(nil), spec.Imports...)
		paths = append(paths, resolveTransformImports(cfg, spec)...)
		internalAliases := reverseAliases(transformAliasSubset(aliases, paths))
		rule.Advice = &Transform{
			Template:      spec.Template,
			Imports:       paths,
			ImportAliases: internalAliases,
		}

	case "hook":
		localAliases := mergeAliases(cfg.ImportAliases, spec.ImportAliases)
		paths := append([]string(nil), spec.Imports...)
		rule.Advice = &Hook{
			Before:        spec.Before,
			After:         spec.After,
			Imports:       paths,
			ImportAliases: pathAliasMap(paths, localAliases),
		}

	case "inject":
		if !strings.HasPrefix(spec.Target, "decl:") {
			return nil, fmt.Errorf(`inject rule target must start with "decl:"`)
		}
		localAliases := mergeAliases(cfg.ImportAliases, spec.ImportAliases)
		paths := append([]string(nil), spec.Imports...)
		rule.Advice = &Inject{
			Start:         spec.Start,
			End:           spec.End,
			Imports:       paths,
			ImportAliases: pathAliasMap(paths, localAliases),
		}

	case "add":
		return nil, fmt.Errorf(`type "add" is handled outside the Engine (ast/custom/add.go)`)

	case "":
		return nil, fmt.Errorf("rule type is empty")
	default:
		return nil, fmt.Errorf("unknown rule type %q", spec.Type)
	}
	return rule, nil
}

// resolveTransformImports is a hook for transform rules to include any additional
// import paths referenced by aliases (e.g. "whatapas" in the template implies the
// whatapas package should be imported, but only if the user lists it in imports).
// Currently identity — kept as a seam for future expansion.
func resolveTransformImports(_ *RulesConfig, _ *RuleSpec) []string {
	return nil
}

// transformAliasSubset filters the global/local aliases map down to the paths actually
// referenced by the transform rule, and drops "default" aliases where the alias name
// equals the last segment of the path. The internal Transform.ImportAliases field only
// stores non-default (explicit) mappings — matching that convention keeps the yaml-derived
// struct byte-identical to the Go hardcoded one.
func transformAliasSubset(aliases map[string]string, paths []string) map[string]string {
	if len(aliases) == 0 || len(paths) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		want[p] = struct{}{}
	}
	out := make(map[string]string)
	for alias, path := range aliases {
		if _, ok := want[path]; !ok {
			continue
		}
		if isDefaultAlias(alias, path) {
			continue
		}
		out[alias] = path
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isDefaultAlias reports whether the alias would be implicitly produced by
// `import "<path>"` (i.e. the last segment of the path).
func isDefaultAlias(alias, path string) bool {
	if alias == "" {
		return true
	}
	slash := strings.LastIndex(path, "/")
	last := path
	if slash >= 0 {
		last = path[slash+1:]
	}
	return alias == last
}

// pathAliasMap builds a (path → alias) map, reading aliases from the given
// alias→path dictionary. Used by Hook / Inject Advice which store this form.
func pathAliasMap(paths []string, aliases map[string]string) map[string]string {
	if len(paths) == 0 || len(aliases) == 0 {
		return nil
	}
	// Build a reverse path→alias index for O(1) lookup.
	rev := reverseAliases(aliases)
	out := make(map[string]string)
	for _, p := range paths {
		if a, ok := rev[p]; ok {
			out[p] = a
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
