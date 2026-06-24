package ast

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestRulesYAMLMatchesAllRules verifies that the on-disk rules.yaml stays
// in sync with the Go-coded AllRules(). rules.yaml is a developer reference
// + override file, not embedded in the binary; this test is the contract
// that any edit to rules.yaml must keep field-level parity with rules.go.
//
// §227 Step 5 revision: code is canonical, yaml is dev-only override. This
// test reads the yaml from the source tree at test time.
func TestRulesYAMLMatchesAllRules(t *testing.T) {
	data, err := os.ReadFile("rules.yaml")
	if err != nil {
		t.Fatalf("read rules.yaml: %v", err)
	}
	yamlRules, err := DecodeRulesYAML(data)
	if err != nil {
		t.Fatalf("decode rules.yaml: %v", err)
	}
	goRules := AllRules()

	if len(yamlRules) != len(goRules) {
		t.Fatalf("rule count mismatch: yaml=%d go=%d", len(yamlRules), len(goRules))
	}

	yamlByTarget := indexByTarget(t, yamlRules, "yaml")
	goByTarget := indexByTarget(t, goRules, "go")

	var mismatches []string
	var keys []string
	for k := range goByTarget {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		goRule := goByTarget[key]
		yamlRule, ok := yamlByTarget[key]
		if !ok {
			mismatches = append(mismatches, fmt.Sprintf("target %q missing in yaml", key))
			continue
		}
		if diff := diffRules(goRule, yamlRule); diff != "" {
			mismatches = append(mismatches, fmt.Sprintf("target %q: %s", key, diff))
		}
	}
	for key := range yamlByTarget {
		if _, ok := goByTarget[key]; !ok {
			mismatches = append(mismatches, fmt.Sprintf("target %q present in yaml but not in go", key))
		}
	}

	if len(mismatches) > 0 {
		t.Fatalf("yaml vs go rules diverge (%d issues):\n  %s",
			len(mismatches), strings.Join(mismatches, "\n  "))
	}
}

// TestLoadBuiltinRulesNonEmpty guards against dispatch regressions — whichever
// source LoadBuiltinRules picks must yield 92 rules.
func TestLoadBuiltinRulesNonEmpty(t *testing.T) {
	rules := LoadBuiltinRules()
	if len(rules) != len(AllRules()) {
		t.Fatalf("LoadBuiltinRules returned %d, want %d", len(rules), len(AllRules()))
	}
}

func indexByTarget(t *testing.T, rules []*Rule, source string) map[string]*Rule {
	t.Helper()
	out := make(map[string]*Rule, len(rules))
	for i, r := range rules {
		key := ruleKey(r, i)
		if _, exists := out[key]; exists {
			t.Fatalf("%s rules: duplicate key %q", source, key)
		}
		out[key] = r
	}
	return out
}

// ruleKey uses target + whatap func to distinguish rules that share a target
// (none currently do, but keeps the index future-proof).
func ruleKey(r *Rule, idx int) string {
	key := r.Target
	if r.Advice != nil {
		key += "|" + adviceSignature(r.Advice)
	}
	_ = idx
	return key
}

func adviceSignature(a Advice) string {
	switch v := a.(type) {
	case *ReplaceFunction:
		return "replace:" + v.WhatapPkg + "." + v.WhatapFunc
	case *WrapCall:
		return "wrap-call:" + v.WhatapPkg + "." + v.WhatapFunc
	case *ReplaceWithCtx:
		return "replace-with-ctx:" + v.WhatapPkg + "." + v.WhatapFunc
	case *ArgWrap:
		return fmt.Sprintf("arg-wrap:%s.%s@%d", v.WhatapPkg, v.WhatapFunc, v.ArgIndex)
	case *ArgInsert:
		return "arg-insert:" + v.WhatapAlias
	case *CodeInsert:
		return "code-insert:" + v.WhatapPkg + "." + v.WhatapFunc
	case *MainInsert:
		return "main-insert:" + v.WhatapPkg + "." + v.WhatapFunc
	case *FieldWrap:
		return "field-wrap:" + v.WhatapPkg + "." + v.WhatapFunc
	case *FieldWrapOrInsert:
		return "field-wrap-or-insert:" + v.WhatapPkg + "." + v.WrapFunc
	case *Transform:
		// §272 Phase 3 Step 4 — ReverseTarget field removed; identify
		// Transform by the first import path (transformer fingerprint).
		key := "transform"
		if len(v.Imports) > 0 {
			key += ":" + v.Imports[0]
		}
		return key
	case *Hook:
		return "hook"
	case *Inject:
		return "inject"
	case *OnMatchFunc:
		return "on-match"
	}
	return "unknown"
}

// diffRules reports the first field that differs between two rules.
// Returns empty string if identical.
func diffRules(a, b *Rule) string {
	if a.Target != b.Target {
		return fmt.Sprintf("Target: go=%q yaml=%q", a.Target, b.Target)
	}
	if a.OptIn != b.OptIn {
		return fmt.Sprintf("OptIn: go=%v yaml=%v", a.OptIn, b.OptIn)
	}
	if !reflect.DeepEqual(a.Signature, b.Signature) {
		return fmt.Sprintf("Signature: go=%+v yaml=%+v", a.Signature, b.Signature)
	}
	if !reflect.DeepEqual(a.Receiver, b.Receiver) {
		return fmt.Sprintf("Receiver: go=%+v yaml=%+v", a.Receiver, b.Receiver)
	}
	if !reflect.DeepEqual(a.Fields, b.Fields) {
		return fmt.Sprintf("Fields: go=%+v yaml=%+v", a.Fields, b.Fields)
	}
	if diff := diffAdvice(a.Advice, b.Advice); diff != "" {
		return "Advice: " + diff
	}
	return ""
}

func diffAdvice(a, b Advice) string {
	ta, tb := fmt.Sprintf("%T", a), fmt.Sprintf("%T", b)
	if ta != tb {
		return fmt.Sprintf("type go=%s yaml=%s", ta, tb)
	}
	switch ga := a.(type) {
	case *ReplaceFunction:
		gb := b.(*ReplaceFunction)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *WrapCall:
		gb := b.(*WrapCall)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *ReplaceWithCtx:
		gb := b.(*ReplaceWithCtx)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *ArgWrap:
		gb := b.(*ArgWrap)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *ArgInsert:
		gb := b.(*ArgInsert)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *CodeInsert:
		gb := b.(*CodeInsert)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *MainInsert:
		gb := b.(*MainInsert)
		// Copy to zero out the internal `inserted` flag (not part of schema).
		ga2 := *ga
		gb2 := *gb
		ga2.inserted = false
		gb2.inserted = false
		if !reflect.DeepEqual(ga2, gb2) {
			return fmt.Sprintf("%+v vs %+v", ga2, gb2)
		}
	case *FieldWrap:
		gb := b.(*FieldWrap)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *FieldWrapOrInsert:
		gb := b.(*FieldWrapOrInsert)
		if !reflect.DeepEqual(ga, gb) {
			return fmt.Sprintf("%+v vs %+v", ga, gb)
		}
	case *Transform:
		// §272 Phase 3 Step 4 — ReverseTarget field removed; no longer
		// part of the yaml↔Go field-level diff.
		gb := b.(*Transform)
		if ga.Template != gb.Template {
			return fmt.Sprintf("Template: go=%q yaml=%q", ga.Template, gb.Template)
		}
		if !reflect.DeepEqual(ga.Imports, gb.Imports) {
			return fmt.Sprintf("Imports: go=%v yaml=%v", ga.Imports, gb.Imports)
		}
		if !reflect.DeepEqual(ga.ImportAliases, gb.ImportAliases) {
			return fmt.Sprintf("ImportAliases: go=%v yaml=%v", ga.ImportAliases, gb.ImportAliases)
		}
	}
	return ""
}

// BenchmarkLoadBuiltinRules measures the canonical Go-coded dispatch
// (§227 Step 5: code is canonical, yaml is dev-only). This is the hot
// path hit by every injector/remover initialization.
func BenchmarkLoadBuiltinRules(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rules := LoadBuiltinRules()
		if len(rules) == 0 {
			b.Fatal("empty rules")
		}
	}
}

// BenchmarkDecodeRulesYAML measures yaml parsing + BuildRules overhead
// for the full 92-rule rules.yaml. This is the dev override path and
// also a proxy for user custom yaml parsing cost per rule.
func BenchmarkDecodeRulesYAML(b *testing.B) {
	data, err := os.ReadFile("rules.yaml")
	if err != nil {
		b.Fatalf("read rules.yaml: %v", err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rules, err := DecodeRulesYAML(data)
		if err != nil {
			b.Fatalf("decode: %v", err)
		}
		if len(rules) == 0 {
			b.Fatal("empty rules")
		}
	}
}
