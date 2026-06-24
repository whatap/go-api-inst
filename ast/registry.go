package ast

import (
	"fmt"
	"os"
	"strings"
)

// Registry holds the target → Rule mapping for inject mode.
// §272 Phase 3 Step 2 (2026-05-19): removed `whatapRules` reverse-map and
// related lookup. remove no longer inverts auto-injection — see issue 272.
type Registry struct {
	rules         map[string]*Rule  // inject: "database/sql.Open" → Rule
	declWildcards []*Rule           // "decl:..." rules whose target contains "*"
	blankImports  map[string]string // import path → whatap import (e.g. logrus)

	// §242 — package-path filter. Values are Rule.Target's extracted package
	// path (e.g. "github.com/gin-gonic/gin", "fmt"). Replaces the former
	// Name-based filter.
	enabled  map[string]bool // user-requested opt-in packages (OptIn rules register only when listed here)
	disabled map[string]bool // user-requested exclusion (rules register only when NOT listed here)
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		rules:        make(map[string]*Rule),
		blankImports: make(map[string]string),
	}
}

// Register adds a built-in rule and automatically builds the reverse mapping
// for Remove mode. Built-in rules honour the package-path filter:
//   - if the rule's package is in `disabled`, skip
//   - if the rule is OptIn and its package is NOT in `enabled`, skip
func (r *Registry) Register(rule *Rule) {
	pkg := ExtractRulePackage(rule.Target)

	if r.disabled != nil && r.disabled[pkg] {
		return
	}
	if rule.OptIn {
		if r.enabled == nil || !r.enabled[pkg] {
			return
		}
	}
	r.registerInternal(rule)
}

// RegisterUser adds a user-defined rule (§227 Step 5). User rules come from
// the explicit `rules:` array in whatap.yaml and **always** apply — the
// package-path filter only governs built-in rules. Otherwise
// `disabled_packages` covering a user rule's package would disable user
// customisation entirely, which is the opposite of what the user asked for.
func (r *Registry) RegisterUser(rule *Rule) {
	r.registerInternal(rule)
}

// registerInternal is the shared registration body for both Register and
// RegisterUser. It owns the wildcard split + reverse-target wiring.
func (r *Registry) registerInternal(rule *Rule) {
	// Wildcard decl: rules go into a separate slice; lookup iterates on miss.
	if strings.HasPrefix(rule.Target, "decl:") && strings.Contains(rule.Target, "*") {
		r.declWildcards = append(r.declWildcards, rule)
		return
	}

	r.rules[rule.Target] = rule
	// §272 Phase 3 Step 2 (2026-05-19): reverse mapping for ModeRemove deleted.
	// remove no longer inverts auto-injection (see §272), so the previous
	// per-Advice whatapRules wiring (§237 옵션 E) is gone. ReverseTarget on
	// Transform is now silently ignored — see custom_rules.go for the user
	// yaml deprecation warning.
}

// Lookup returns the Rule for a target string (inject mode).
// First checks exact matches, then falls back to decl wildcards.
func (r *Registry) Lookup(target string) *Rule {
	if rule, ok := r.rules[target]; ok {
		return rule
	}
	if strings.HasPrefix(target, "decl:") {
		for _, rule := range r.declWildcards {
			if matchDeclWildcard(rule.Target, target) {
				return rule
			}
		}
	}
	return nil
}

// matchDeclWildcard reports whether a "decl:pkgpath.funcName" target matches a
// wildcard pattern like "decl:pkgpath.*" or "decl:*".
//
// Supported patterns:
//   - "decl:*"             — any function declaration
//   - "decl:pkgpath.*"     — any function in pkgpath
//   - "decl:pkgpath.Pre*"  — prefix match within pkgpath
//   - "decl:pkgpath.*Suf"  — suffix match within pkgpath
//   - "decl:pkgpath.A*Z"   — single middle wildcard (one "*" anywhere in the
//                            function-name segment, must straddle pkg.func dot
//                            on the function side only)
//
// Multiple "*" within one segment are not supported.
func matchDeclWildcard(pattern, target string) bool {
	if pattern == target {
		return true
	}
	if !strings.HasPrefix(pattern, "decl:") || !strings.HasPrefix(target, "decl:") {
		return false
	}
	p := pattern[len("decl:"):]
	t := target[len("decl:"):]
	// "*" — match all
	if p == "*" {
		return true
	}
	// Reject patterns with more than one wildcard.
	if strings.Count(p, "*") != 1 {
		return false
	}
	star := strings.Index(p, "*")
	prefix, suffix := p[:star], p[star+1:]
	if !strings.HasPrefix(t, prefix) {
		return false
	}
	if !strings.HasSuffix(t, suffix) {
		return false
	}
	// Avoid prefix and suffix overlapping in a too-short target.
	return len(t) >= len(prefix)+len(suffix)
}

// §272 Phase 3 Step 2 (2026-05-19): LookupWhatap removed — remove no longer
// matches against whatap-side targets.

// SetPackageFilter sets the user-supplied enabled/disabled package lists.
// The values are full Rule.Target package paths (e.g. "fmt",
// "github.com/gin-gonic/gin"). Either list may be nil — nil = no filter on
// that dimension. §242 replaces the former SetNameFilter.
func (r *Registry) SetPackageFilter(enabled, disabled []string) {
	r.enabled = toSet(enabled)
	r.disabled = toSet(disabled)
}

func toSet(paths []string) map[string]bool {
	if len(paths) == 0 {
		return nil
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = true
		}
	}
	return out
}

// ValidatePackageFilter warns on entries in the user's enabled_packages /
// disabled_packages that do not match any known built-in rule. §242 Step 11.
//
// Decision: **warn + continue**, never fail the build. Typos or stale entries
// emit a stderr line so the user notices, while fresh Rule additions do not
// race with user configs that haven't caught up yet. Custom user rules bypass
// the filter entirely, so they are irrelevant here.
//
// Pass the full built-in rule set — typically LoadBuiltinRules() — because the
// Registry itself has already dropped skipped rules by the time filter checks
// would run.
func ValidatePackageFilter(enabled, disabled []string, builtinRules []*Rule) {
	if len(enabled) == 0 && len(disabled) == 0 {
		return
	}

	// Collect every package path present among built-in rules.
	known := make(map[string]bool, 64)
	for _, rule := range builtinRules {
		if rule == nil {
			continue
		}
		pkg := ExtractRulePackage(rule.Target)
		if pkg != "" {
			known[pkg] = true
		}
	}

	emit := func(listName, pkg string) {
		fmt.Fprintf(os.Stderr,
			"[whatap-go-inst] warning: unknown package in %s: %q (no built-in rule matches). Typo or fork?\n",
			listName, pkg)
	}
	for _, p := range enabled {
		p = strings.TrimSpace(p)
		if p != "" && !known[p] {
			emit("enabled_packages", p)
		}
	}
	for _, p := range disabled {
		p = strings.TrimSpace(p)
		if p != "" && !known[p] {
			emit("disabled_packages", p)
		}
	}
}

// RegisterBlankImport registers a blank-import mapping.
// When importPath is detected in a file, whatapImport is added as a blank import.
func (r *Registry) RegisterBlankImport(importPath, whatapImport string) {
	r.blankImports[importPath] = whatapImport
}

// BlankImports returns the blank import mappings.
func (r *Registry) BlankImports() map[string]string {
	return r.blankImports
}

// Size returns the number of registered rules.
func (r *Registry) Size() int {
	return len(r.rules)
}

// AllRules returns all registered rules.
func (r *Registry) AllRules() []*Rule {
	result := make([]*Rule, 0, len(r.rules))
	for _, rule := range r.rules {
		result = append(result, rule)
	}
	return result
}
