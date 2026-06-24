package ast

import (
	"testing"

	"github.com/dave/dst"
)

// TestIsReplacedTarget_DisabledByFlag — skipReplacedModules=false 시
// replacedModules 가 채워져 있어도 항상 false.
func TestIsReplacedTarget_DisabledByFlag(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux"},
		skipReplacedModules: false,
	}
	if e.isReplacedTarget("github.com/gorilla/mux.NewRouter") {
		t.Errorf("skipReplacedModules=false 인데 true 반환")
	}
}

// TestIsReplacedTarget_EmptyList — replacedModules 비어있으면 false.
func TestIsReplacedTarget_EmptyList(t *testing.T) {
	e := &Engine{
		replacedModules:     nil,
		skipReplacedModules: true,
	}
	if e.isReplacedTarget("github.com/gorilla/mux.NewRouter") {
		t.Errorf("replacedModules 비어있는데 true 반환")
	}
}

// TestIsReplacedTarget_ExactMatch — 함수 target 의 패키지 = replaced 모듈.
func TestIsReplacedTarget_ExactMatch(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux"},
		skipReplacedModules: true,
	}
	if !e.isReplacedTarget("github.com/gorilla/mux.NewRouter") {
		t.Errorf("exact match Rule 이 skip 되지 않음")
	}
}

// TestIsReplacedTarget_MethodTarget — 메서드 target 도 패키지 추출이 정확해야 함.
// ExtractRulePackage 사용으로 "<pkg>.<Type>.<Method>" 형식에서 <pkg> 만 비교.
func TestIsReplacedTarget_MethodTarget(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux"},
		skipReplacedModules: true,
	}
	if !e.isReplacedTarget("github.com/gorilla/mux.Route.Subrouter") {
		t.Errorf("method-style target 이 skip 되지 않음")
	}
}

// TestIsReplacedTarget_SubpackagePrefix — replaced 모듈의 서브패키지도 매칭.
func TestIsReplacedTarget_SubpackagePrefix(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gomodule/redigo"},
		skipReplacedModules: true,
	}
	if !e.isReplacedTarget("github.com/gomodule/redigo/redis.Dial") {
		t.Errorf("subpackage prefix match 가 skip 되지 않음")
	}
}

// TestIsReplacedTarget_FalsePrefix — 우연한 prefix 일치 방지.
// "gorilla/mux" replaced 시 "gorilla/muxer" 는 매칭되면 안 됨.
func TestIsReplacedTarget_FalsePrefix(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux"},
		skipReplacedModules: true,
	}
	if e.isReplacedTarget("github.com/gorilla/muxer.NewThing") {
		t.Errorf("false-prefix 가 매칭됨 (mux 와 muxer 구분 실패)")
	}
}

// TestIsReplacedTarget_NoMatch — 다른 패키지 Rule 은 영향 없음.
// replace 가 mux 에만 있을 때 sql Rule 은 그대로 변환되어야 함.
func TestIsReplacedTarget_NoMatch(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux"},
		skipReplacedModules: true,
	}
	if e.isReplacedTarget("database/sql.Open") {
		t.Errorf("관련 없는 Rule 이 skip 됨")
	}
}

// TestIsReplacedTarget_MultipleReplaced — replace 가 여러 모듈에 걸쳐 있는 경우.
func TestIsReplacedTarget_MultipleReplaced(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"github.com/gorilla/mux", "github.com/sashabaranov/go-openai"},
		skipReplacedModules: true,
	}
	if !e.isReplacedTarget("github.com/sashabaranov/go-openai.NewClient") {
		t.Errorf("두 번째 replaced 모듈 매칭 실패")
	}
	if !e.isReplacedTarget("github.com/gorilla/mux.NewRouter") {
		t.Errorf("첫 번째 replaced 모듈 매칭 실패")
	}
	if e.isReplacedTarget("net/http.DefaultClient.Get") {
		t.Errorf("관련 없는 Rule 이 skip 됨")
	}
}

// TestIsReplacedTarget_EmptyEntry — replacedModules 에 빈 문자열 섞여 있어도 안전.
func TestIsReplacedTarget_EmptyEntry(t *testing.T) {
	e := &Engine{
		replacedModules:     []string{"", "github.com/gorilla/mux"},
		skipReplacedModules: true,
	}
	if !e.isReplacedTarget("github.com/gorilla/mux.NewRouter") {
		t.Errorf("빈 entry 가 있어도 다른 entry 매칭은 정상 동작해야 함")
	}
	if e.isReplacedTarget("net/http.Get") {
		t.Errorf("빈 entry 가 모든 target 에 매칭됨 (false positive)")
	}
}

// TestSetters — setter 로 필드가 정확히 채워지는지.
func TestEngine_SettersReplacedModules(t *testing.T) {
	noopResolve := func(_ dst.Node) string { return "" }
	e := NewEngine(NewRegistry(), ModeInject, noopResolve)
	e.SetReplacedModules([]string{"foo/bar"})
	e.SetSkipReplacedModules(true)
	if len(e.replacedModules) != 1 || e.replacedModules[0] != "foo/bar" {
		t.Errorf("SetReplacedModules 가 필드를 채우지 않음: %v", e.replacedModules)
	}
	if !e.skipReplacedModules {
		t.Errorf("SetSkipReplacedModules(true) 가 적용되지 않음")
	}
	e.SetSkipReplacedModules(false)
	if e.skipReplacedModules {
		t.Errorf("SetSkipReplacedModules(false) 가 적용되지 않음")
	}
}
