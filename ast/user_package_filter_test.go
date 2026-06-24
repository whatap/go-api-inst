package ast

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// §242 Step 11. User-supplied enabled/disabled paths drive the built-in rule
// registration. Each case below exercises one behaviour of SetPackageFilter
// + Register.
//
// Registry.Register (not Lookup) is what the engine calls during buildRegistry,
// so these tests bypass the lookup path and assert presence in the internal
// rules map via r.rules[target] existence. Lookup only checks exact match for
// non-wildcard targets, which is all we need.
func TestPackageFilter(t *testing.T) {
	optInRule := func(target, pkg string) *Rule {
		_ = pkg
		return &Rule{Target: target, OptIn: true, Advice: &ReplaceFunction{
			WhatapPkg: "example.com/stub/" + target, WhatapFunc: "X",
		}}
	}
	defaultRule := func(target string) *Rule {
		return &Rule{Target: target, Advice: &ReplaceFunction{
			WhatapPkg: "example.com/stub/" + target, WhatapFunc: "X",
		}}
	}

	fmtPrint := optInRule("fmt.Print", "fmt")
	fmtPrintf := optInRule("fmt.Printf", "fmt")
	fmtPrintln := optInRule("fmt.Println", "fmt")
	ginNew := defaultRule("github.com/gin-gonic/gin.New")
	sqlOpen := defaultRule("database/sql.Open")
	echoV4 := optInRule("github.com/labstack/echo/v4.New", "github.com/labstack/echo/v4")
	echoV3 := optInRule("github.com/labstack/echo.New", "github.com/labstack/echo")

	all := []*Rule{fmtPrint, fmtPrintf, fmtPrintln, ginNew, sqlOpen, echoV4, echoV3}

	cases := []struct {
		name        string
		enabled     []string
		disabled    []string
		wantPresent []string // target strings that must be registered
		wantAbsent  []string // target strings that must NOT be registered
	}{
		{
			name:        "1: opt-in via enabled_packages",
			enabled:     []string{"fmt"},
			wantPresent: []string{"fmt.Print", "fmt.Printf", "fmt.Println", "github.com/gin-gonic/gin.New", "database/sql.Open"},
			wantAbsent:  []string{"github.com/labstack/echo/v4.New", "github.com/labstack/echo.New"},
		},
		{
			name:        "2: no config → OptIn rules are skipped, Default rules register",
			wantPresent: []string{"github.com/gin-gonic/gin.New", "database/sql.Open"},
			wantAbsent:  []string{"fmt.Print", "fmt.Printf", "fmt.Println", "github.com/labstack/echo/v4.New"},
		},
		{
			name:        "3: disabled_packages excludes a Default rule",
			disabled:    []string{"github.com/gin-gonic/gin"},
			wantPresent: []string{"database/sql.Open"},
			wantAbsent:  []string{"github.com/gin-gonic/gin.New", "fmt.Print"},
		},
		{
			name:        "4: exact match — echo/v4 opt-in does NOT pull in echo (v3)",
			enabled:     []string{"github.com/labstack/echo/v4"},
			wantPresent: []string{"github.com/labstack/echo/v4.New", "github.com/gin-gonic/gin.New", "database/sql.Open"},
			wantAbsent:  []string{"github.com/labstack/echo.New", "fmt.Print"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRegistry()
			r.SetPackageFilter(tc.enabled, tc.disabled)
			for _, rule := range all {
				r.Register(rule)
			}
			for _, target := range tc.wantPresent {
				if _, ok := r.rules[target]; !ok {
					t.Errorf("expected target %q registered, missing", target)
				}
			}
			for _, target := range tc.wantAbsent {
				if _, ok := r.rules[target]; ok {
					t.Errorf("expected target %q NOT registered, but it was", target)
				}
			}
		})
	}
}

// §242 Step 11 case 5 — unknown enabled/disabled entries must NOT fail the
// build. They emit a stderr warning and the build continues (fresh Rule
// additions vs. user configs that haven't caught up must not race).
func TestValidatePackageFilter_UnknownPathsWarn(t *testing.T) {
	builtins := []*Rule{
		{Target: "database/sql.Open"},
		{Target: "github.com/gin-gonic/gin.New"},
		{Target: "fmt.Print", OptIn: true},
	}

	stderrBuf, restore := captureStderr(t)

	ValidatePackageFilter(
		[]string{"github.com/gin", "fmt"},                      // gin is a typo, fmt is known
		[]string{"github.com/labstack/echo/v4", "unknown/pkg"}, // echo/v4 not in builtins, unknown/pkg likewise
		builtins,
	)

	// Close the write end and wait for the capture goroutine before reading.
	restore()
	out := stderrBuf.String()

	// Must warn for both unknown entries.
	wantWarnings := []string{
		`unknown package in enabled_packages: "github.com/gin"`,
		`unknown package in disabled_packages: "github.com/labstack/echo/v4"`,
		`unknown package in disabled_packages: "unknown/pkg"`,
	}
	for _, w := range wantWarnings {
		if !strings.Contains(out, w) {
			t.Errorf("missing expected warning %q in stderr:\n%s", w, out)
		}
	}

	// Must NOT warn for known entries.
	if strings.Contains(out, `unknown package in enabled_packages: "fmt"`) {
		t.Errorf("unexpected warning for known fmt in stderr:\n%s", out)
	}
}

// captureStderr swaps os.Stderr with a pipe and returns the captured bytes
// via the buffer. The returned func restores the original stderr.
func captureStderr(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	buf := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(buf, r)
		close(done)
	}()

	restore := func() {
		_ = w.Close()
		os.Stderr = orig
		<-done
		_ = r.Close()
	}
	return buf, restore
}
