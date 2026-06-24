package ast

import (
	"strings"
	"testing"

	"github.com/whatap/go-api-inst/config"
	"gopkg.in/yaml.v3"
)

// loadCustomFromYAML is a test helper: marshals a yaml fragment for the
// top-level user config and runs LoadCustomRules through it.
func loadCustomFromYAML(t *testing.T, src string) ([]*Rule, error) {
	t.Helper()
	var cfg config.Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	return LoadCustomRules(&cfg)
}

// TestLoadCustomRulesAllTypes exercises one yaml example per discriminator
// (10 internal + hook + inject = 12 happy cases). The 13th type `add` is
// covered as an error case below.
func TestLoadCustomRulesAllTypes(t *testing.T) {
	cases := []struct {
		name     string
		yaml     string
		check    func(t *testing.T, r *Rule)
		wantType string
	}{
		{
			name: "replace",
			yaml: `
version: 1
importAliases:
  whatapsql: "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
rules:
  - type: replace
    target: "database/sql.Open"
    with: "whatapsql.Open"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*ReplaceFunction)
				if a.WhatapAlias != "whatapsql" || a.WhatapFunc != "Open" {
					t.Errorf("unexpected ReplaceFunction: %+v", a)
				}
			},
		},
		{
			name: "replace-with-ctx",
			yaml: `
version: 1
importAliases:
  whataphttp: "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
rules:
  - type: replace-with-ctx
    target: "net/http.DefaultClient.Get"
    with: "whataphttp.DefaultClientGet"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*ReplaceWithCtx)
				if a.OrigVar != "DefaultClient" || a.OrigFunc != "Get" {
					t.Errorf("expected OrigVar=DefaultClient OrigFunc=Get, got %+v", a)
				}
			},
		},
		{
			name: "wrap-call",
			yaml: `
version: 1
importAliases:
  whatapgin: "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
rules:
  - type: wrap-call
    target: "github.com/gin-gonic/gin.Default"
    with: "whatapgin.WrapEngine"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*WrapCall)
				if a.WhatapFunc != "WrapEngine" {
					t.Errorf("expected WrapEngine, got %+v", a)
				}
			},
		},
		{
			name: "arg-wrap",
			yaml: `
version: 1
importAliases:
  whataplogsink: "github.com/whatap/go-api/logsink"
rules:
  - type: arg-wrap
    target: "log.New"
    with: "whataplogsink.GetTraceLogWriter"
    argIndex: 0
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*ArgWrap)
				if a.ArgIndex != 0 || a.WhatapFunc != "GetTraceLogWriter" {
					t.Errorf("unexpected ArgWrap: %+v", a)
				}
			},
		},
		{
			name: "arg-insert",
			yaml: `
version: 1
importAliases:
  whatapgrpc: "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc"
rules:
  - type: arg-insert
    target: "google.golang.org/grpc.NewServer"
    whatapAlias: whatapgrpc
    insertArgs:
      - {wrapFunc: UnaryInterceptor, innerFunc: UnaryServerInterceptor}
    ellipsis: true
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*ArgInsert)
				if !a.Ellipsis || len(a.InsertArgs) != 1 {
					t.Errorf("unexpected ArgInsert: %+v", a)
				}
				if a.InsertArgs[0].WrapFunc != "UnaryInterceptor" {
					t.Errorf("unexpected wrapFunc: %+v", a.InsertArgs[0])
				}
			},
		},
		{
			name: "code-insert",
			yaml: `
version: 1
importAliases:
  whatapk8s: "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes"
rules:
  - type: code-insert
    target: "k8s.io/client-go/kubernetes.NewForConfig"
    with: "whatapk8s.WrapRoundTripper"
    position: before
    argSource: 0
    methodName: Wrap
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*CodeInsert)
				if a.Position != "before" || a.MethodName != "Wrap" || a.WhatapFunc != "WrapRoundTripper" {
					t.Errorf("unexpected CodeInsert: %+v", a)
				}
			},
		},
		{
			name: "main-insert",
			yaml: `
version: 1
importAliases:
  whataplogsink: "github.com/whatap/go-api/logsink"
rules:
  - type: main-insert
    target: "log.SetOutput"
    with: "whataplogsink.GetTraceLogWriter"
    extraImport: "os"
    wrapExpr: "os.Stderr"
    origPkg: "log"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*MainInsert)
				if a.OrigPkgAlias != "log" || a.OrigFunc != "SetOutput" || a.WrapExpr != "os.Stderr" {
					t.Errorf("unexpected MainInsert: %+v", a)
				}
			},
		},
		{
			name: "field-wrap",
			yaml: `
version: 1
importAliases:
  whataphttp: "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
rules:
  - type: field-wrap
    target: "lit:net/http.Server{}"
    with: "whataphttp.WrapHandler"
    fieldName: Handler
    fields:
      - {name: Handler, required: true}
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*FieldWrap)
				if a.FieldName != "Handler" {
					t.Errorf("unexpected FieldWrap.FieldName: %q", a.FieldName)
				}
				if r.Target != "net/http.Server{}" {
					t.Errorf("expected lit: prefix to be stripped, got %q", r.Target)
				}
				if len(r.Fields) != 1 || !r.Fields[0].Required {
					t.Errorf("unexpected Fields: %+v", r.Fields)
				}
			},
		},
		{
			name: "field-wrap-or-insert",
			yaml: `
version: 1
importAliases:
  whataphttp: "github.com/whatap/go-api/instrumentation/net/http/whataphttp"
rules:
  - type: field-wrap-or-insert
    target: "lit:net/http.Client{}"
    wrapWith: "whataphttp.NewRoundTrip"
    insertWith: "whataphttp.NewRoundTripWithEmptyTransport"
    fieldName: Transport
    ctxAware: true
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*FieldWrapOrInsert)
				if a.WrapFunc != "NewRoundTrip" || a.InsertFunc != "NewRoundTripWithEmptyTransport" {
					t.Errorf("unexpected FieldWrapOrInsert: %+v", a)
				}
				if !a.CtxAware {
					t.Errorf("expected CtxAware=true")
				}
			},
		},
		{
			name: "transform",
			yaml: `
version: 1
importAliases:
  whatapdb: "github.com/whatap/go-api/sql"
rules:
  - type: transform
    target: "github.com/aerospike/aerospike-client-go/v6.Client.Put"
    template: 'whatapdb.Wrap({{.Ctx}}, ...)'
    imports:
      - "github.com/whatap/go-api/sql"
      - "context"
`,
			// §272 Phase 3 Step 4 + 별개 cycle (2026-05-19) — yaml `reverseTarget:`
			// key removed entirely. RuleSpec field gone, strict decoder rejects
			// the key. Coverage for that rejection is in
			// TestLoadCustomRules_ReverseTargetRejected.
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*Transform)
				if got := a.ImportAliases["github.com/whatap/go-api/sql"]; got != "whatapdb" {
					t.Errorf("expected whatapdb alias, got %q", got)
				}
			},
		},
		{
			name: "hook",
			yaml: `
version: 1
rules:
  - type: hook
    target: "mypkg.Query"
    before: 'log.Println("before")'
    after:  'log.Println("after")'
    imports:
      - "log"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*Hook)
				if !strings.Contains(a.Before, "before") || !strings.Contains(a.After, "after") {
					t.Errorf("unexpected Hook: %+v", a)
				}
			},
		},
		{
			name: "inject",
			yaml: `
version: 1
rules:
  - type: inject
    target: "decl:myapp/service.ProcessOrder"
    start: 'log.Println("start")'
    end:   'log.Println("end")'
    imports:
      - "log"
`,
			check: func(t *testing.T, r *Rule) {
				a := r.Advice.(*Inject)
				if !strings.Contains(a.Start, "start") || !strings.Contains(a.End, "end") {
					t.Errorf("unexpected Inject: %+v", a)
				}
				if r.Target != "decl:myapp/service.ProcessOrder" {
					t.Errorf("decl target should not be normalized away, got %q", r.Target)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := loadCustomFromYAML(t, tc.yaml)
			if err != nil {
				t.Fatalf("LoadCustomRules: %v", err)
			}
			if len(rules) != 1 {
				t.Fatalf("expected 1 rule, got %d", len(rules))
			}
			tc.check(t, rules[0])
		})
	}
}

// TestLoadCustomRulesEmpty: a config with no Rules/Imports/ImportAliases
// returns (nil, nil) so callers can short-circuit safely.
func TestLoadCustomRulesEmpty(t *testing.T) {
	rules, err := LoadCustomRules(&config.Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != nil {
		t.Fatalf("expected nil rules, got %d", len(rules))
	}
}

// TestLoadCustomRulesNil: nil config returns (nil, nil).
func TestLoadCustomRulesNil(t *testing.T) {
	rules, err := LoadCustomRules(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rules != nil {
		t.Fatalf("expected nil rules, got %d", len(rules))
	}
}

// TestLoadCustomRulesGlobalAndRuleLevelAliasesMerge verifies that rule-level
// importAliases override the global value (Q10 A merge rule).
func TestLoadCustomRulesGlobalAndRuleLevelAliasesMerge(t *testing.T) {
	src := `
version: 1
importAliases:
  whatapgorm: "github.com/whatap/go-api/instrumentation/github.com/go-gorm/gorm/whatapgorm"
rules:
  - type: replace
    target: "gorm.io/gorm.Open"
    with: "whatapgorm.Open"
  - type: replace
    target: "github.com/jinzhu/gorm.Open"
    with: "whatapgorm.Open"
    importAliases:
      whatapgorm: "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm"
`
	rules, err := loadCustomFromYAML(t, src)
	if err != nil {
		t.Fatalf("LoadCustomRules: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	gorm := rules[0].Advice.(*ReplaceFunction)
	if !strings.Contains(gorm.WhatapPkg, "go-gorm") {
		t.Errorf("expected gorm rule to resolve to go-gorm path, got %q", gorm.WhatapPkg)
	}
	jinzhu := rules[1].Advice.(*ReplaceFunction)
	if !strings.Contains(jinzhu.WhatapPkg, "jinzhu") {
		t.Errorf("expected jinzhu rule to resolve to jinzhu path, got %q", jinzhu.WhatapPkg)
	}
}

// TestLoadCustomRulesErrors covers the error paths the loader is expected to
// catch (rather than silently producing invalid Advice).
func TestLoadCustomRulesErrors(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantSub string
	}{
		{
			name: "empty type",
			yaml: `
version: 1
rules:
  - target: "foo.Bar"
`,
			wantSub: "rule type is empty",
		},
		{
			name: "unknown type",
			yaml: `
version: 1
rules:
  - type: replace-everything
    target: "foo.Bar"
`,
			wantSub: `unknown rule type "replace-everything"`,
		},
		{
			name: "replace missing with",
			yaml: `
version: 1
rules:
  - type: replace
    target: "foo.Bar"
`,
			wantSub: `"with" is required`,
		},
		{
			name: "replace unknown alias",
			yaml: `
version: 1
rules:
  - type: replace
    target: "foo.Bar"
    with: "missing.Func"
`,
			wantSub: `unknown importAlias "missing"`,
		},
		{
			name: "code-insert bad position",
			yaml: `
version: 1
importAliases:
  whatapfoo: "example.com/whatapfoo"
rules:
  - type: code-insert
    target: "foo.Bar"
    with: "whatapfoo.Wrap"
    position: middle
    methodName: Wrap
`,
			wantSub: `position must be "before" or "after"`,
		},
		{
			name: "field-wrap-or-insert alias mismatch",
			yaml: `
version: 1
importAliases:
  a: "example.com/a"
  b: "example.com/b"
rules:
  - type: field-wrap-or-insert
    target: "lit:foo.Bar{}"
    wrapWith: "a.X"
    insertWith: "b.Y"
    fieldName: F
`,
			wantSub: "must share the same alias",
		},
		{
			name: "inject without decl prefix",
			yaml: `
version: 1
rules:
  - type: inject
    target: "myapp.ProcessOrder"
    start: 'log.Println("hi")'
`,
			wantSub: `must start with "decl:"`,
		},
		{
			name: "add type rejected",
			yaml: `
version: 1
rules:
  - type: add
    package: main
    file: whatap_init.go
    content: "package main"
`,
			wantSub: `handled outside the Engine`,
		},
		// §231: rules[i] 내부 필드 오타가 strict decoding 으로 잡히는지 확인.
		// yaml.Node.Decode 는 KnownFields 를 존중하지 않으므로 Marshal + 재-decode
		// 로 우회한다. 각 케이스는 "rules[0]:" 프리픽스 + "field <name> not found"
		// 메시지의 핵심 단편으로 검증.
		{
			name: "typo: type → tpye",
			yaml: `
version: 1
rules:
  - tpye: replace
    target: "foo.Bar"
    with: "whatapfoo.Bar"
`,
			wantSub: `field tpye not found`,
		},
		{
			name: "typo: target → tagret",
			yaml: `
version: 1
importAliases:
  whatapfoo: "example.com/whatapfoo"
rules:
  - type: replace
    tagret: "foo.Bar"
    with: "whatapfoo.Bar"
`,
			wantSub: `field tagret not found`,
		},
		{
			name: "typo: with → wiht",
			yaml: `
version: 1
importAliases:
  whatapfoo: "example.com/whatapfoo"
rules:
  - type: replace
    target: "foo.Bar"
    wiht: "whatapfoo.Bar"
`,
			wantSub: `field wiht not found`,
		},
		{
			name: "typo: hook before → befor",
			yaml: `
version: 1
rules:
  - type: hook
    target: "main.fetchData"
    befor: 'log.Println(">>>")'
`,
			wantSub: `field befor not found`,
		},
		{
			name: "typo: inject start → strat",
			yaml: `
version: 1
rules:
  - type: inject
    target: "decl:myapp.ProcessOrder"
    strat: 'log.Println("hi")'
`,
			wantSub: `field strat not found`,
		},
		{
			name: "typo: transform template → tempalte",
			yaml: `
version: 1
importAliases:
  whatapfoo: "example.com/whatapfoo"
rules:
  - type: transform
    target: "foo.Bar"
    tempalte: "{{.Original}}"
`,
			wantSub: `field tempalte not found`,
		},
		{
			name: "typo: replace-with-ctx → nested signature field",
			yaml: `
version: 1
importAliases:
  whatapfoo: "example.com/whatapfoo"
rules:
  - type: replace
    target: "foo.Bar"
    with: "whatapfoo.Bar"
    signature:
      minarg: 1
`,
			wantSub: `field minarg not found`,
		},
		{
			name: "rules[0] prefix preserved",
			yaml: `
version: 1
rules:
  - tpye: hook
    target: "main.doWork"
`,
			wantSub: `rules[0]:`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadCustomFromYAML(t, tc.yaml)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSub, err)
			}
		})
	}
}

// TestLoadCustomRules_ReverseTargetRejected — §272 Phase 4 + 별개 cycle (2026-05-19):
// 사용자 yaml 에 deprecated `reverseTarget:` 키가 남아 있으면 strict decoder
// 가 unknown field 에러를 던진다. 이전엔 deprecation warning + 무시였지만
// 사용자 yaml 영향이 거의 없다는 점이 확인되어 즉시 제거됨.
func TestLoadCustomRules_ReverseTargetRejected(t *testing.T) {
	src := `
version: 1
importAliases:
  whatapdb: "github.com/whatap/go-api/sql"
rules:
  - type: transform
    target: "github.com/example/db.Client.Query"
    template: 'whatapdb.Wrap({{.Ctx}}, ...)'
    imports:
      - "github.com/whatap/go-api/sql"
      - "context"
    reverseTarget: "github.com/whatap/go-api/sql.WrapQuery"
`
	_, err := loadCustomFromYAML(t, src)
	if err == nil {
		t.Fatalf("expected strict-decoder error for `reverseTarget`, got nil")
	}
	if !strings.Contains(err.Error(), "reverseTarget") {
		t.Errorf("expected error mentioning `reverseTarget`, got: %v", err)
	}
}
