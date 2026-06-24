package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestShouldSkipReplacedModules_DefaultNil — yaml 미지정 (nil) 시 기본 true.
func TestShouldSkipReplacedModules_DefaultNil(t *testing.T) {
	var c InstrumentationConfig // zero value, SkipReplacedModules == nil
	if !c.ShouldSkipReplacedModules() {
		t.Errorf("nil pointer → 기본값 true 여야 함")
	}
}

// TestShouldSkipReplacedModules_ExplicitTrue — yaml 에 명시 true.
func TestShouldSkipReplacedModules_ExplicitTrue(t *testing.T) {
	v := true
	c := InstrumentationConfig{SkipReplacedModules: &v}
	if !c.ShouldSkipReplacedModules() {
		t.Errorf("explicit true 가 false 로 보고됨")
	}
}

// TestShouldSkipReplacedModules_ExplicitFalse — yaml 에 명시 false.
func TestShouldSkipReplacedModules_ExplicitFalse(t *testing.T) {
	v := false
	c := InstrumentationConfig{SkipReplacedModules: &v}
	if c.ShouldSkipReplacedModules() {
		t.Errorf("explicit false 가 true 로 보고됨")
	}
}

// TestYAMLUnmarshal_SkipReplacedModules — yaml 의 다양한 값이 *bool 로 정확히 파싱.
func TestYAMLUnmarshal_SkipReplacedModules(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantNil bool
		wantVal bool
	}{
		{"missing", `instrumentation: {}`, true, false},
		{"true", "instrumentation:\n  skip_replaced_modules: true", false, true},
		{"false", "instrumentation:\n  skip_replaced_modules: false", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var cfg Config
			if err := yaml.Unmarshal([]byte(c.yaml), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := cfg.Instrumentation.SkipReplacedModules
			if c.wantNil {
				if got != nil {
					t.Errorf("expected nil, got pointer to %v", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected pointer to %v, got nil", c.wantVal)
			}
			if *got != c.wantVal {
				t.Errorf("expected %v, got %v", c.wantVal, *got)
			}
		})
	}
}

// TestConfigMerge_SkipReplacedModules — pointer-aware merge.
// nil 값은 기존 값 유지, non-nil 값은 override.
func TestConfigMerge_SkipReplacedModules(t *testing.T) {
	tru := true
	fal := false

	cases := []struct {
		name  string
		base  *bool
		other *bool
		want  *bool
	}{
		{"both nil", nil, nil, nil},
		{"base nil, other false", nil, &fal, &fal},
		{"base nil, other true", nil, &tru, &tru},
		{"base true, other nil → 유지", &tru, nil, &tru},
		{"base false, other true → override", &fal, &tru, &tru},
		{"base true, other false → override", &tru, &fal, &fal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := &Config{Instrumentation: InstrumentationConfig{SkipReplacedModules: c.base}}
			other := &Config{Instrumentation: InstrumentationConfig{SkipReplacedModules: c.other}}
			base.Merge(other)
			got := base.Instrumentation.SkipReplacedModules
			if c.want == nil {
				if got != nil {
					t.Errorf("expected nil, got pointer to %v", *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected pointer to %v, got nil", *c.want)
			}
			if *got != *c.want {
				t.Errorf("expected %v, got %v", *c.want, *got)
			}
		})
	}
}
