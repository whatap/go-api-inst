package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §272 Phase 2 Step 1 — manual 패턴 단순 삭제가 기본 동작이 되었는지 회귀 검증.
// removeManualPatterns 가 8 케이스를 빠짐없이 처리하는지 확인.
//
// 의도: --all flag 없이도 manual patterns 처리되어야 함 (§272 절대 규칙).

func TestRemove_ManualPatterns_DefaultBehavior(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		wantHas   []string // 결과에 반드시 남아 있어야 할 substring
		wantGone  []string // 결과에 없어야 할 substring
	}{
		{
			name: "1. whatap import + 단독 statement 제거",
			src: `package main

import (
	"fmt"
	"github.com/whatap/go-api/trace"
)

func main() {
	trace.Init("app", "")
	defer trace.Shutdown()
	fmt.Println("hello")
}
`,
			wantHas: []string{`"fmt"`, `fmt.Println("hello")`},
			wantGone: []string{
				`"github.com/whatap/go-api/trace"`,
				`trace.Init(`,
				`trace.Shutdown`,
			},
		},
		{
			name: "2. defer trace.End 제거",
			src: `package main

import "github.com/whatap/go-api/trace"

func handler() {
	ctx, _ := trace.Start(nil, "tx")
	defer trace.End(ctx, nil)
}
`,
			wantHas: []string{
				`trace.Start`, // warning 대상이라 본문에는 남음
			},
			wantGone: []string{
				`defer trace.End`,
				`"github.com/whatap/go-api/trace"`,
			},
		},
		{
			name: "3. logsink.Send 단독 statement 제거",
			src: `package main

import "github.com/whatap/go-api/logsink"

func report() {
	logsink.Send("CATEGORY", "msg", nil)
}
`,
			wantGone: []string{
				`logsink.Send`,
				`"github.com/whatap/go-api/logsink"`,
			},
		},
		{
			name: "4. trace.Step / trace.Println / trace.Error 제거",
			src: `package main

import "github.com/whatap/go-api/trace"

func work(ctx interface{}) {
	trace.Step(ctx, "label", "msg", 0, 0)
	trace.Println(ctx, "CAT", "msg")
	trace.Error(ctx, nil)
}
`,
			wantGone: []string{
				`trace.Step`,
				`trace.Println`,
				`trace.Error`,
				`"github.com/whatap/go-api/trace"`,
			},
		},
		{
			name: "5. 중첩 블록 (if) 안의 manual statement 제거",
			src: `package main

import "github.com/whatap/go-api/trace"

func work(ok bool, ctx interface{}) {
	if ok {
		trace.Step(ctx, "a", "b", 0, 0)
	}
}
`,
			wantGone: []string{`trace.Step`, `"github.com/whatap/go-api/trace"`},
			wantHas:  []string{`if ok {`},
		},
		{
			name: "6. ctx, _ := trace.Start(...) 좌변 — warning + 보존",
			src: `package main

import "github.com/whatap/go-api/trace"

func handler() {
	ctx, _ := trace.Start(nil, "tx")
	_ = ctx
}
`,
			// 좌변 변경 + ctx 전파는 자동 처리 불가 → warning + 보존
			wantHas: []string{`trace.Start`},
		},
		{
			name: "7. closure 패턴 (whatapsql.Wrap) — 보존",
			src: `package main

import "github.com/whatap/go-api/sql/whatapsql"

func dbWork() {
	whatapsql.Wrap(nil, "SELECT 1", nil, func() {})
}
`,
			// closure 는 안쪽 함수 실행 의의가 있으므로 보존
			wantHas: []string{`whatapsql.Wrap`},
		},
		{
			name: "8. factory 패턴 (whataplogrus.New) — 보존",
			src: `package main

import "github.com/whatap/go-api/instrumentation/github.com/sirupsen/logrus/whataplogrus"

func setup() {
	_ = whataplogrus.New
}
`,
			// New* / Open* 은 객체 생성이므로 보존
			wantHas: []string{`whataplogrus.New`},
		},
	}

	tmpDir, err := os.MkdirTemp("", "remover_manual_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(tmpDir, "src_"+itoa(i)+".go")
			dst := filepath.Join(tmpDir, "out_"+itoa(i)+".go")
			if err := os.WriteFile(src, []byte(tc.src), 0644); err != nil {
				t.Fatalf("write src: %v", err)
			}

			// §272: --all flag 없이도 manual 처리가 기본
			rem := NewRemover(false)
			if err := rem.RemoveFile(src, dst); err != nil {
				t.Fatalf("RemoveFile: %v", err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read dst: %v", err)
			}
			gotStr := string(got)

			for _, h := range tc.wantHas {
				if !strings.Contains(gotStr, h) {
					t.Errorf("expected to contain %q\n--- got ---\n%s", h, gotStr)
				}
			}
			for _, g := range tc.wantGone {
				if strings.Contains(gotStr, g) {
					t.Errorf("expected NOT to contain %q\n--- got ---\n%s", g, gotStr)
				}
			}
		})
	}
}

// §246 / §272 — ReplaceFunction reverse 화이트리스트 회귀 검증.
// 사용자가 SDK 가이드 따라 손으로 `whatap*.X(...)` 적용한 경우 →
// 원본 `pkg.X(...)` 으로 되돌리고 + 원본 import 추가.
func TestRemove_ManualPatterns_ReplaceFunctionReverse(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantHas  []string
		wantGone []string
	}{
		{
			name: "1. whatapsql.Open(driver, dsn) → sql.Open(driver, dsn) + database/sql import",
			src: `package main

import (
	whatapsql "github.com/whatap/go-api/instrumentation/database/sql/whatapsql"
)

func dbSetup() {
	db, _ := whatapsql.Open("mysql", "user:pass@/db")
	_ = db
}
`,
			wantHas: []string{
				`sql.Open("mysql", "user:pass@/db")`,
				`"database/sql"`,
			},
			wantGone: []string{`whatapsql`},
		},
		{
			name: "2. whatapfmt.Println(...) → fmt.Println(...) + fmt import",
			src: `package main

import whatapfmt "github.com/whatap/go-api/instrumentation/fmt/whatapfmt"

func main() {
	whatapfmt.Println("hello")
	whatapfmt.Printf("%d\n", 42)
}
`,
			wantHas: []string{
				`fmt.Println("hello")`,
				`fmt.Printf("%d\n", 42)`,
				`"fmt"`,
			},
			wantGone: []string{`whatapfmt`},
		},
		{
			name: "3. whatapgoredis.NewClient(...) → redis.NewClient(...) + go-redis/v9 import",
			src: `package main

import whatapgoredis "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis"

type Options struct{}

func setup() {
	client := whatapgoredis.NewClient(&Options{})
	_ = client
}
`,
			wantHas: []string{
				`redis.NewClient(&Options{})`,
				`"github.com/redis/go-redis/v9"`,
			},
			wantGone: []string{`whatapgoredis`},
		},
		{
			name: "4. whatapmongo.Connect(ctx, opts) → mongo.Connect(ctx, opts) + mongo import",
			src: `package main

import whatapmongo "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo"

func mongoSetup() {
	c, _ := whatapmongo.Connect(nil, nil)
	_ = c
}
`,
			wantHas: []string{
				`mongo.Connect(nil, nil)`,
				`"go.mongodb.org/mongo-driver/mongo"`,
			},
			wantGone: []string{`whatapmongo`},
		},
		{
			name: "5. 화이트리스트 외 — 매핑 없음, 그대로",
			src: `package main

import "github.com/whatap/go-api/trace"

func main() {
	trace.Init("app", "")
}
`,
			// trace.Init 은 ReplaceFunction Rule 아니라 reverse 매핑 없음.
			// removeManualPatterns 가 단독 statement 로 처리 → 라인 삭제.
			wantGone: []string{`trace.Init`, `"github.com/whatap/go-api/trace"`},
		},
	}

	tmpDir, err := os.MkdirTemp("", "remover_replacefn_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(tmpDir, "src_"+itoa(i)+".go")
			dst := filepath.Join(tmpDir, "out_"+itoa(i)+".go")
			if err := os.WriteFile(src, []byte(tc.src), 0644); err != nil {
				t.Fatalf("write src: %v", err)
			}

			rem := NewRemover(false)
			if err := rem.RemoveFile(src, dst); err != nil {
				t.Fatalf("RemoveFile: %v", err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read dst: %v", err)
			}
			gotStr := string(got)

			for _, h := range tc.wantHas {
				if !strings.Contains(gotStr, h) {
					t.Errorf("expected to contain %q\n--- got ---\n%s", h, gotStr)
				}
			}
			for _, g := range tc.wantGone {
				if strings.Contains(gotStr, g) {
					t.Errorf("expected NOT to contain %q\n--- got ---\n%s", g, gotStr)
				}
			}
		})
	}
}

// §272 Phase 2 follow-up — closure FuncLit Body 순회 회귀 검증.
// engine.GET("/", func(c) { trace.Step(...) }) 같이 콜백/고루틴/defer/대입
// 안의 FuncLit 내부 manual statement 가 제거되는지 확인.
func TestRemove_ManualPatterns_ClosureBodies(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantHas  []string
		wantGone []string
	}{
		{
			name: "1. 콜백 인자 안의 trace.Step",
			src: `package main

import "github.com/whatap/go-api/trace"

func register(cb func(int)) {}

func main() {
	register(func(n int) {
		trace.Step(nil, "cb", "msg", 0, 0)
	})
}
`,
			wantHas: []string{`register(func(n int) {`},
			wantGone: []string{
				`trace.Step`,
				`"github.com/whatap/go-api/trace"`,
			},
		},
		{
			name: "2. go func() { whatap ... }() — 고루틴 안",
			src: `package main

import "github.com/whatap/go-api/trace"

func main() {
	go func() {
		trace.Step(nil, "g", "msg", 0, 0)
	}()
}
`,
			wantGone: []string{`trace.Step`, `"github.com/whatap/go-api/trace"`},
			wantHas:  []string{`go func() {`},
		},
		{
			name: "3. defer func() { whatap ...; user-code }() — defer 안 (mixed)",
			src: `package main

import (
	"fmt"
	"github.com/whatap/go-api/trace"
)

func main() {
	defer func() {
		trace.Step(nil, "d", "msg", 0, 0)
		fmt.Println("cleanup")
	}()
}
`,
			// closure body 에 사용자 코드 (fmt.Println) 도 있어야 defer 보존
			// (whatap-only closure 는 의도적으로 통째 제거됨, isRemovableFuncLit)
			wantGone: []string{`trace.Step`, `"github.com/whatap/go-api/trace"`},
			wantHas:  []string{`defer func() {`, `fmt.Println("cleanup")`},
		},
		{
			name: "4. handler := func() { whatap ... } — 변수 대입",
			src: `package main

import "github.com/whatap/go-api/trace"

func main() {
	handler := func() {
		trace.Step(nil, "h", "msg", 0, 0)
	}
	_ = handler
}
`,
			wantGone: []string{`trace.Step`, `"github.com/whatap/go-api/trace"`},
			wantHas:  []string{`handler := func() {`},
		},
	}

	tmpDir, err := os.MkdirTemp("", "remover_closure_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(tmpDir, "src_"+itoa(i)+".go")
			dst := filepath.Join(tmpDir, "out_"+itoa(i)+".go")
			if err := os.WriteFile(src, []byte(tc.src), 0644); err != nil {
				t.Fatalf("write src: %v", err)
			}

			rem := NewRemover(false)
			if err := rem.RemoveFile(src, dst); err != nil {
				t.Fatalf("RemoveFile: %v", err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read dst: %v", err)
			}
			gotStr := string(got)

			for _, h := range tc.wantHas {
				if !strings.Contains(gotStr, h) {
					t.Errorf("expected to contain %q\n--- got ---\n%s", h, gotStr)
				}
			}
			for _, g := range tc.wantGone {
				if strings.Contains(gotStr, g) {
					t.Errorf("expected NOT to contain %q\n--- got ---\n%s", g, gotStr)
				}
			}
		})
	}
}

// §272 Phase 2 Step 2 — wrapper unwrap 회귀 검증.
// 변수 대입 RHS 의 화이트리스트 wrapper 호출이 안쪽 인자로 교체되는지.
func TestRemove_ManualPatterns_WrapperUnwrap(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		wantHas  []string
		wantGone []string
	}{
		{
			name: "1. whatapgin.WrapEngine(gin.New()) → gin.New()",
			src: `package main

import (
	"github.com/gin-gonic/gin"
	"github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"
)

func setup() {
	engine := whatapgin.WrapEngine(gin.New())
	_ = engine
}
`,
			wantHas: []string{
				`engine := gin.New()`,
				`"github.com/gin-gonic/gin"`,
			},
			wantGone: []string{
				`whatapgin.WrapEngine`,
				`whatapgin`,
			},
		},
		// §272 Phase 3 Step 1 (2026-05-19): engine.Process(ModeRemove) 호출 제거
		// 후 활성화. whatapsql.Open 이 ReplaceFunction Rule 의 ModeRemove
		// 분기로 더 이상 변환되지 않으므로 unwrap 단독 검증 가능.
		{
			name: "2. whatapsql.Open(driver, dsn) — §246 B 안 ReplaceFunction reverse → sql.Open(driver, dsn)",
			src: `package main

import (
	"github.com/whatap/go-api/sql/whatapsql"
)

func dbSetup() {
	db, _ := whatapsql.Open("mysql", "user:pass@/db")
	_ = db
}
`,
			// 인자 2개 — wrapper unwrap 안 함. §246 B 안의 reverseReplaceFunctionCalls
			// 가 매칭 → 함수명 + alias 교체 + database/sql import 추가.
			wantHas: []string{
				`sql.Open("mysql", "user:pass@/db")`,
				`"database/sql"`,
			},
			wantGone: []string{`whatapsql`},
		},
		{
			name: "3. db, _ := whatapsql.Open(sql.Open(...)) — 인자 1개 unwrap → sql.Open(...)",
			src: `package main

import (
	"database/sql"
	"github.com/whatap/go-api/sql/whatapsql"
)

func dbSetup() {
	db, _ := whatapsql.Open(sql.Open("mysql", "user:pass@/db"))
	_ = db
}
`,
			wantHas: []string{
				`db, _ := sql.Open("mysql", "user:pass@/db")`,
				`"database/sql"`,
			},
			wantGone: []string{`whatapsql.Open`, `whatapsql`},
		},
		{
			name: "4. handler := whataphttp.Func(myHandler) → myHandler",
			src: `package main

import (
	"net/http"
	"github.com/whatap/go-api/instrumentation/net/http/whataphttp"
)

func myHandler(w http.ResponseWriter, r *http.Request) {}

func setup() {
	handler := whataphttp.Func(myHandler)
	_ = handler
}
`,
			wantHas: []string{
				`handler := myHandler`,
				`"net/http"`,
			},
			wantGone: []string{`whataphttp.Func`, `whataphttp`},
		},
		{
			name: "5. writer := whataplogsink.GetTraceLogWriter(os.Stdout) → os.Stdout",
			src: `package main

import (
	"os"
	"github.com/whatap/go-api/logsink/whataplogsink"
)

func setup() {
	writer := whataplogsink.GetTraceLogWriter(os.Stdout)
	_ = writer
}
`,
			wantHas: []string{
				`writer := os.Stdout`,
				`"os"`,
			},
			wantGone: []string{`whataplogsink.GetTraceLogWriter`, `whataplogsink`},
		},
		{
			name: "6. 비화이트리스트 — 보존 + warning",
			src: `package main

import (
	"github.com/whatap/go-api/httpc"
)

func handler() {
	httpcCtx, _ := httpc.Start(nil, "http://example.com")
	_ = httpcCtx
}
`,
			// httpc.Start 는 wrapper 아님 (LLM/HTTP 추적 시작). unwrap X → warning + 보존
			wantHas: []string{`httpc.Start`},
		},
	}

	tmpDir, err := os.MkdirTemp("", "remover_unwrap_test")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(tmpDir, "src_"+itoa(i)+".go")
			dst := filepath.Join(tmpDir, "out_"+itoa(i)+".go")
			if err := os.WriteFile(src, []byte(tc.src), 0644); err != nil {
				t.Fatalf("write src: %v", err)
			}

			rem := NewRemover(false)
			if err := rem.RemoveFile(src, dst); err != nil {
				t.Fatalf("RemoveFile: %v", err)
			}

			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read dst: %v", err)
			}
			gotStr := string(got)

			for _, h := range tc.wantHas {
				if !strings.Contains(gotStr, h) {
					t.Errorf("expected to contain %q\n--- got ---\n%s", h, gotStr)
				}
			}
			for _, g := range tc.wantGone {
				if strings.Contains(gotStr, g) {
					t.Errorf("expected NOT to contain %q\n--- got ---\n%s", g, gotStr)
				}
			}
		})
	}
}

// itoa avoids strconv import overhead in test files.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
