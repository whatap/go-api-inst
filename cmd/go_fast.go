package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/whatap/go-api-inst/config"
	"github.com/whatap/go-api-inst/report"
	"golang.org/x/mod/modfile"
)

// goModRequires reads go.mod and returns the set of required module paths
// (exact match — uses modfile.Parse). Returns nil on read/parse error.
//
// §270: replaces the previous string-scan `hasGoAPIRequire` which used
// strings.Contains and accidentally matched nested modules
// (`github.com/whatap/go-api/instrumentation/llm`) when only the body was
// expected.
func goModRequires(projectDir string) map[string]bool {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil
	}
	out := make(map[string]bool, len(f.Require))
	for _, req := range f.Require {
		out[req.Mod.Path] = true
	}
	return out
}

// hasGoAPIRequire checks if go.mod has an exact `require` for
// github.com/whatap/go-api. A `replace` directive alone is not enough —
// the module must be required.
func hasGoAPIRequire(projectDir string) bool {
	required := goModRequires(projectDir)
	return required["github.com/whatap/go-api"]
}

// hasGoAPILLMRequire checks if go.mod has an exact `require` for the LLM
// nested module (§270).
func hasGoAPILLMRequire(projectDir string) bool {
	required := goModRequires(projectDir)
	return required["github.com/whatap/go-api/instrumentation/llm"]
}

// needsGoAPILLM returns true if go.mod requires at least one LLM SDK that
// maps (via frameworkToWhatap) to a whatap adapter inside the LLM nested
// module (instrumentation/llm/ prefix). Used to decide whether to add the
// LLM module require + tool file import (§270).
//
// `replace` directives ARE excluded (§205 교훈) — replaced modules may use
// a different implementation whose public API differs from the original
// SDK, so the whatap adapter cannot safely wrap it. This matches the
// behavior of `buildNestedModuleToolImports` and `buildVendorToolFile`,
// keeping require + tool file decisions in sync.
func needsGoAPILLM(projectDir string) bool {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return false
	}
	required := make(map[string]bool, len(f.Require))
	for _, req := range f.Require {
		required[req.Mod.Path] = true
	}
	replaced := make(map[string]bool, len(f.Replace))
	for _, rep := range f.Replace {
		replaced[rep.Old.Path] = true
	}
	const llmModPrefix = "github.com/whatap/go-api/instrumentation/llm/"
	for _, fw := range frameworkToWhatap {
		if !strings.HasPrefix(fw.whatapPkg, llmModPrefix) {
			continue
		}
		if required[fw.goModPath] && !replaced[fw.goModPath] {
			return true
		}
	}
	return false
}

// frameworkToWhatap maps go.mod require module paths to whatap instrumentation import paths.
// goModPath: exact module path as it appears in go.mod require (including /vN suffix).
// whatapPkg: whatap instrumentation package import path.
var frameworkToWhatap = []struct {
	goModPath string // exact module path in go.mod require
	whatapPkg string // whatap instrumentation package import path
}{
	{"github.com/gin-gonic/gin", "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin"},
	{"github.com/labstack/echo/v4", "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho"},
	{"github.com/go-chi/chi", "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"},
	{"github.com/go-chi/chi/v5", "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi"},
	{"github.com/gorilla/mux", "github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux"},
	{"github.com/valyala/fasthttp", "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp"},
	{"google.golang.org/grpc", "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc"},
	{"gorm.io/gorm", "github.com/whatap/go-api/instrumentation/github.com/go-gorm/gorm/whatapgorm"},
	{"github.com/jinzhu/gorm", "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm"},
	{"github.com/jmoiron/sqlx", "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx"},
	{"github.com/sirupsen/logrus", "github.com/whatap/go-api/instrumentation/github.com/sirupsen/logrus/whataplogrus"},
	{"github.com/gomodule/redigo", "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo"},
	{"github.com/redis/go-redis/v9", "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis"},
	{"github.com/go-redis/redis/v8", "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis"},
	{"go.mongodb.org/mongo-driver", "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo"},
	{"github.com/IBM/sarama", "github.com/whatap/go-api/instrumentation/github.com/IBM/sarama/whatapsarama"},
	{"github.com/Shopify/sarama", "github.com/whatap/go-api/instrumentation/github.com/Shopify/sarama/whatapsarama"},
	{"github.com/aerospike/aerospike-client-go/v6", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas"},
	{"github.com/gofiber/fiber/v2", "github.com/whatap/go-api/instrumentation/github.com/gofiber/fiber/v2/whatapfiber"},
	{"k8s.io/client-go", "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes"},
	{"github.com/sashabaranov/go-openai", "github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
	{"github.com/cloudwego/eino-ext/components/model/openai", "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
	{"github.com/cloudwego/eino-ext/components/model/claude", "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
	{"github.com/anthropics/anthropic-sdk-go", "github.com/whatap/go-api/instrumentation/llm/github.com/anthropics/anthropic-sdk-go/whatapanthropic"},
	{"github.com/openai/openai-go", "github.com/whatap/go-api/instrumentation/llm/github.com/openai/openai-go/whatapopenaigo"},
}

// buildVendorToolFile scans go.mod and builds a tool file with imports
// matching the frameworks the project actually uses.
// §205: modfile.Parse로 정확한 require 모듈 경로 매칭 (strings.Contains 제거).
// replace된 모듈은 제외.
func buildVendorToolFile(projectDir string, debug bool) string {
	// Core packages — always included (no external deps)
	imports := []string{
		"github.com/whatap/go-api/trace",
		"github.com/whatap/go-api/logsink",
		"github.com/whatap/go-api/method",
		"github.com/whatap/go-api/sql",
		"github.com/whatap/go-api/httpc",
		"github.com/whatap/go-api/instrumentation/net/http/whataphttp",
		"github.com/whatap/go-api/instrumentation/fmt/whatapfmt",
		"github.com/whatap/go-api/instrumentation/database/sql/whatapsql",
	}

	// Parse go.mod to detect frameworks
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: cannot read go.mod: %v\n", err)
		}
		return buildToolFileContent(imports)
	}

	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: cannot parse go.mod: %v\n", err)
		}
		return buildToolFileContent(imports)
	}

	// Collect replaced module paths (these use a different implementation, skip them)
	replacedMods := make(map[string]bool)
	for _, rep := range f.Replace {
		replacedMods[rep.Old.Path] = true
	}

	// Collect require module paths (exact match)
	requiredMods := make(map[string]bool)
	for _, req := range f.Require {
		requiredMods[req.Mod.Path] = true
	}

	for _, fw := range frameworkToWhatap {
		// Exact match against require module path, skip if replaced
		if requiredMods[fw.goModPath] && !replacedMods[fw.goModPath] {
			imports = append(imports, fw.whatapPkg)
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Vendor: detected %s → %s\n", fw.goModPath, fw.whatapPkg)
			}
		}
	}

	return buildToolFileContent(imports)
}

// parseReplacedModules parses go.mod and returns module paths that have replace directives.
// §205: Transformers skip injection for replaced modules to prevent build failures
// (e.g. traefik replaces gorilla/mux with containous/mux fork).
func parseReplacedModules(projectDir string, debug bool) []string {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil
	}
	var replaced []string
	for _, rep := range f.Replace {
		replaced = append(replaced, rep.Old.Path)
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] go.mod replace: %s => %s\n", rep.Old.Path, rep.New.Path)
		}
	}
	return replaced
}

// buildNestedModuleToolImports — §270. 비-vendor 모드에서 사용자 go.mod 의
// require 에 매칭되는 frameworkToWhatap 항목 중 whatapPkg 가 nested module
// (현재는 instrumentation/llm/ prefix) 인 것만 import 한 줄 씩 반환.
// 매칭 안 되면 빈 문자열. tool file 의 tidy 가 nested module require 를
// prune 하지 않도록 함.
func buildNestedModuleToolImports(projectDir string, debug bool) string {
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return ""
	}
	required := make(map[string]bool)
	for _, req := range f.Require {
		required[req.Mod.Path] = true
	}
	replaced := make(map[string]bool)
	for _, rep := range f.Replace {
		replaced[rep.Old.Path] = true
	}

	const llmModPrefix = "github.com/whatap/go-api/instrumentation/llm/"
	var out strings.Builder
	seenLLM := false
	for _, fw := range frameworkToWhatap {
		if !strings.HasPrefix(fw.whatapPkg, llmModPrefix) {
			continue
		}
		if required[fw.goModPath] && !replaced[fw.goModPath] {
			out.WriteString("import _ \"")
			out.WriteString(fw.whatapPkg)
			out.WriteString("\"\n")
			seenLLM = true
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] §270 nested-module tool import: %s\n", fw.whatapPkg)
			}
		}
	}
	_ = seenLLM
	return out.String()
}

// buildToolFileContent generates the tool file source from import list.
func buildToolFileContent(imports []string) string {
	content := "//go:build tools\n\npackage tools\n\nimport (\n"
	for _, imp := range imports {
		content += "\t_ \"" + imp + "\"\n"
	}
	content += ")\n"
	return content
}

// runFastBuild fast build mode (using toolexec).
// §234: --output (CLI) drives instrumented source persistence; no separate --no-output.
// Empty outputDir means "do not save instrumented source" (new default).
func runFastBuild(subCmd string, args []string, cfg *config.Config, errorTracking bool) {
	debug := cfg.Instrumentation.Debug
	verbose := debug || os.Getenv("WHATAP_TIMING") != ""
	totalStart := time.Now()

	// Project directory (cfg.BaseDir or cwd)
	projectDir := cfg.BaseDir
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	// Convert to absolute path for toolexec (§72 fix)
	if !filepath.IsAbs(projectDir) {
		absProjectDir, err := filepath.Abs(projectDir)
		if err == nil {
			projectDir = absProjectDir
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Build mode: fast (toolexec)\n")
	}

	// §240: vendor detection moved up so the report's Environment field can
	// include it alongside other runtime context. Subsequent phases reuse
	// this same boolean.
	isVendor := isVendorProject(projectDir, args)

	// §238: Initialize report for toolexec build path (previously only wired to
	// `inject`/`remove` commands). Dependency-level information is populated
	// here in the parent process.
	InitReport("build")
	loadDependencies(report.Get(), projectDir)

	// §240: Snapshot the effective config so the report is reproducible —
	// readers can see preset/enabled/disabled packages, external modules,
	// and error tracking without chasing down the original whatap.yaml.
	report.Get().SetConfigSnapshot(buildConfigSnapshot(cfg, outputDir, errorTracking))

	// §240: Environment / whatap / go-api / module / invocation info so the
	// report is self-contained for remote diagnosis. Collected once per build
	// in the parent; children don't need access.
	report.Get().SetEnvironment(&report.Environment{
		GoVersion:  runtime.Version(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		CGOEnabled: os.Getenv("CGO_ENABLED"),
		VendorMode: isVendor,
	})
	report.Get().SetWhatap(&report.WhatapInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
	})
	report.Get().SetGoAPI(extractGoAPIInfo(projectDir))
	report.Get().SetModule(extractModuleInfo(projectDir))
	report.Get().SetInvocation(&report.BuildInvocation{
		SubCmd:    subCmd,
		BuildArgs: append([]string(nil), args...),
	})

	// §239: When --report is requested, create a temp directory so each
	// toolexec child can drop a per-package fragment JSON. Merged back into
	// the parent's report after the build. `--report` unset → fragDir empty,
	// children do nothing, zero overhead.
	var reportFragDir string
	if reportPath != "" {
		var fragErr error
		reportFragDir, fragErr = os.MkdirTemp("", "whatap-report-frags-")
		if fragErr != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: create report fragment dir failed: %v\n", fragErr)
			reportFragDir = ""
		} else {
			defer os.RemoveAll(reportFragDir)
		}
	}

	// 1. Add go-api dependency if not required in go.mod
	phaseStart := time.Now()

	if debug && isVendor {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Vendor project detected: %s\n", projectDir)
	}

	// §200: go mod edit + tool file (1 import) + go mod tidy.
	// - go get: upgrades transitive deps → can require newer Go → fail
	// - go mod edit alone: no download, go.sum incomplete
	// - tidy without tool file: removes go-api (nothing imports it)
	// - tool file (1 core import) + tidy: keeps go-api, downloads, go.sum complete
	if !hasGoAPIRequire(projectDir) {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Adding go-api dependency...\n")
		}
		goAPIVersion := "v" + Version
		editCmd := exec.Command("go", "mod", "edit", "-require=github.com/whatap/go-api@"+goAPIVersion)
		editCmd.Dir = projectDir
		if err := editCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: go mod edit failed: %v\n", err)
		}
	}

	// §270: LLM nested module 은 조건부 require — 사용자 go.mod 에 LLM SDK
	// (sashabaranov / eino-ext openai/claude / anthropic-sdk-go) 매핑이 있는
	// 경우에만 추가. LLM 안 쓰는 사용자에게 무거운 anthropic + eino transitive
	// 부담 안 줌. (본체 go-api 와 같은 lockstep 버전 사용.)
	if needsGoAPILLM(projectDir) && !hasGoAPILLMRequire(projectDir) {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §270: Adding LLM sub-module dependency...\n")
		}
		llmVersion := "v" + Version
		editCmd := exec.Command("go", "mod", "edit",
			"-require=github.com/whatap/go-api/instrumentation/llm@"+llmVersion)
		editCmd.Dir = projectDir
		if err := editCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: go mod edit (LLM) failed: %v\n", err)
		}
	}

	// §200: Tool file + tidy to add go-api and populate go.sum.
	// Non-vendor: 1 core import (no external deps, no conflicts) +
	//             §270: nested module(s) detected via frameworkToWhatap → import 추가
	//             (tidy 가 prune 하지 않도록 하기 위해)
	// Vendor: go.mod scan → import only matching whatap packages.
	toolFile := filepath.Join(projectDir, "whatap_instrumentation_deps_.go")
	var toolContent string
	if isVendor {
		toolContent = buildVendorToolFile(projectDir, debug)
	} else {
		toolContent = "//go:build tools\n\npackage tools\n\nimport _ \"github.com/whatap/go-api/trace\"\n"
		// §270: 비-vendor 도 frameworkToWhatap 매핑을 검사해 nested module 어댑터
		// (instrumentation/llm/...) 가 필요하면 tool file 에 추가. 핵심 어댑터
		// (gin/echo/...) 는 본체 module 의 sub-package 라 trace 한 줄만으로 go-api
		// require 가 유지되지만, LLM nested module 은 별도 require 필요.
		toolContent += buildNestedModuleToolImports(projectDir, debug)
	}
	os.WriteFile(toolFile, []byte(toolContent), 0644)

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = projectDir
	if debug {
		tidyCmd.Stdout = os.Stderr
		tidyCmd.Stderr = os.Stderr
	}
	if err := tidyCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: go mod tidy failed: %v\n", err)
	}

	// Vendor: sync vendor/ with tool file still present (so vendor copies whatap packages).
	if isVendor {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Running go mod vendor...\n")
		}
		vendorCmd := exec.Command("go", "mod", "vendor")
		vendorCmd.Dir = projectDir
		if debug {
			vendorCmd.Stdout = os.Stderr
			vendorCmd.Stderr = os.Stderr
		}
		if err := vendorCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: go mod vendor failed: %v\n", err)
		}
	}

	os.Remove(toolFile)

	depSetupDuration := time.Since(phaseStart).Seconds()
	if verbose {
		fmt.Fprintf(os.Stderr, "[TIMING] fast_dep_setup: %.2fs\n", depSetupDuration)
	}

	// 2. Pre-resolve whatap package archives
	phaseStart = time.Now()
	resolveCache, err := preResolveWhatapPackages(projectDir, debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: pre-resolve failed: %v\n", err)
		// Continue without importcfg patching; toolexec will still transform source.
	}

	// §205: Parse go.mod replace directives and pass to toolexec via resolve cache.
	// Transformers will skip injection for replaced modules.
	if resolveCache != nil {
		resolveCache.ReplacedModules = parseReplacedModules(projectDir, debug)
	}
	var cacheFilePath string
	if resolveCache != nil {
		cacheFilePath, err = writeCacheFile(resolveCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: write resolve cache failed: %v\n", err)
		} else {
			defer os.Remove(cacheFilePath)
		}
	}
	preResolveDuration := time.Since(phaseStart).Seconds()
	if verbose {
		fmt.Fprintf(os.Stderr, "[TIMING] fast_pre_resolve: %.2fs\n", preResolveDuration)
	}

	// §240: PreResolve report info — resolved count + replaced modules +
	// expanded external module list.
	preResolveInfo := &report.PreResolveInfo{}
	if resolveCache != nil {
		preResolveInfo.ResolvedCount = len(resolveCache.Packages)
		preResolveInfo.ReplacedModules = append([]string(nil), resolveCache.ReplacedModules...)
	}
	if cfg.HasExternalModules() {
		debugEnv := debug
		expanded := resolveExternalModuleList(cfg.ExternalModules, projectDir, debugEnv)
		preResolveInfo.ExternalModules = expanded
	}
	report.Get().SetPreResolve(preResolveInfo)

	// §211: Apply custom add rules (non-append) before go build.
	// Fast mode operates on the user's project directory directly, so we must
	// (a) never overwrite existing files and (b) defer-remove what we created
	// to guarantee cleanup even on build failure. Append rules are NOT handled
	// here — they are the Step 2 scope (toolexec-time).
	var addedFiles []string
	if len(cfg.Add) > 0 {
		created, err := applyAddRulesFast(projectDir, cfg, debug)
		if err != nil {
			for _, f := range created {
				os.Remove(f)
			}
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: apply add rules failed: %v\n", err)
			os.Exit(1)
		}
		addedFiles = created
		defer func() {
			for _, f := range addedFiles {
				if rmErr := os.Remove(f); rmErr != nil && debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: remove %s failed: %v\n", f, rmErr)
				}
			}
		}()
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: created %d add-rule file(s)\n", len(addedFiles))
		}
	}

	// 3. Find current executable path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot find executable path: %v\n", err)
		os.Exit(1)
	}

	// 4. Configure toolexec flag
	toolexecFlag := fmt.Sprintf("-toolexec=%s toolexec", execPath)
	if errorTracking {
		toolexecFlag = fmt.Sprintf("-toolexec=%s toolexec --error-tracking", execPath)
	}

	// 5. Convert build target paths (when running from outside)
	cwd, _ := os.Getwd()
	convertedArgs := convertBuildTargets(args, projectDir, cwd)

	// 6. Resolve instrumented source output directory (§234).
	// Priority: CLI --output > cfg.Instrumentation.OutputDir > GO_API_AST_OUTPUT_DIR env.
	// Empty result = "do not save" (new default). No hardcoded whatap-instrumented
	// fallback — CLI NoOptDefVal handles that case.
	instrumentedOutputDir := outputDir
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = cfg.Instrumentation.OutputDir
	}
	if instrumentedOutputDir == "" {
		instrumentedOutputDir = os.Getenv("GO_API_AST_OUTPUT_DIR")
	}
	if instrumentedOutputDir != "" && !filepath.IsAbs(instrumentedOutputDir) {
		instrumentedOutputDir = filepath.Join(cwd, instrumentedOutputDir)
	}

	// 7. Configure go command (no -tags whatap_tools — §94 fix)
	var buildArgs []string
	buildArgs = append(buildArgs, subCmd)
	buildArgs = append(buildArgs, toolexecFlag)
	buildArgs = append(buildArgs, convertedArgs...)

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] ProjectDir: %s\n", projectDir)
		if instrumentedOutputDir != "" {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] InstrumentedOutput: %s\n", instrumentedOutputDir)
		}
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] go %s\n", strings.Join(buildArgs, " "))
	}

	// 8. Execute go command (in project directory)
	// Pass save path to toolexec via GO_API_AST_OUTPUT_DIR environment variable
	// Pass project directory to toolexec via GO_API_PROJECT_DIR environment variable (§72 fix)
	// Pass resolve cache path to toolexec via GO_API_RESOLVE_CACHE environment variable
	phaseStart = time.Now()
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Dir = projectDir
	// Build environment with our variables, filtering out any existing duplicates
	env := os.Environ()
	env = filterEnvVar(env, "GO_API_AST_OUTPUT_DIR")
	env = filterEnvVar(env, "GO_API_PROJECT_DIR")
	env = filterEnvVar(env, "GO_API_RESOLVE_CACHE")
	env = filterEnvVar(env, "GO_API_EXTERNAL_MODULES")
	env = filterEnvVar(env, "GO_API_REPORT_FRAG_DIR")
	if instrumentedOutputDir != "" {
		env = append(env, "GO_API_AST_OUTPUT_DIR="+instrumentedOutputDir)
	}
	if reportFragDir != "" {
		env = append(env, "GO_API_REPORT_FRAG_DIR="+reportFragDir)
	}
	env = append(env, "GO_API_PROJECT_DIR="+projectDir)
	if cacheFilePath != "" {
		env = append(env, "GO_API_RESOLVE_CACHE="+cacheFilePath)
	}
	// §174: Pass external-module list to toolexec for GOMODCACHE filtering
	if cfg.HasExternalModules() {
		env = append(env, "GO_API_EXTERNAL_MODULES="+strings.Join(cfg.ExternalModules, ","))
	}
	// §188: Pass vendor mode to toolexec
	if isVendor {
		env = append(env, "GO_API_VENDOR_MODE=true")
	}
	buildCmd.Env = env
	buildCmd.Stdin = os.Stdin
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	buildErr := buildCmd.Run()
	goBuildDuration := time.Since(phaseStart).Seconds()
	if verbose {
		fmt.Fprintf(os.Stderr, "[TIMING] fast_go_build: %.2fs\n", goBuildDuration)
	}

	// §200: No restore needed — go.mod/go.sum/vendor changes are permanent.
	// User opted to use whatap monitoring, so go-api dependency stays.

	// §234: Copy go.mod, go.sum and persist add-rule files only when
	// --output (or config/env equivalent) is set.
	if instrumentedOutputDir != "" {
		copyProjectFiles(projectDir, instrumentedOutputDir)
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] go.mod, go.sum copied: %s\n", instrumentedOutputDir)
		}

		// §211: Persist custom add-rule files to instrumentedOutputDir before
		// defer removes them from projectDir. Without this, add files whose
		// package is not imported by main (and therefore never touched by
		// toolexec) would be missing from whatap-instrumented/, breaking
		// source-level reproduction (`cd whatap-instrumented && go build .`).
		// Files that toolexec already dumped are overwritten with the same
		// bytes — harmless and keeps the logic simple.
		for _, src := range addedFiles {
			relPath, err := filepath.Rel(projectDir, src)
			if err != nil {
				continue
			}
			dst := filepath.Join(instrumentedOutputDir, relPath)
			if mkErr := os.MkdirAll(filepath.Dir(dst), 0755); mkErr != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: mkdir %s failed: %v\n", filepath.Dir(dst), mkErr)
				}
				continue
			}
			data, rdErr := os.ReadFile(src)
			if rdErr != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: read %s failed: %v\n", src, rdErr)
				}
				continue
			}
			if wrErr := os.WriteFile(dst, data, 0644); wrErr != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: write %s failed: %v\n", dst, wrErr)
				}
				continue
			}
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: add rule persisted %s\n", dst)
			}
		}

		// §234 step 8: finalise external-module tree under _modules/.
		// toolexec already wrote the instrumented .go files to
		// <outputDir>/_modules/<sanitized>/... via saveInstrumentedFile.
		// Here we copy the corresponding go.mod from GOMODCACHE, inject a
		// require for github.com/whatap/go-api, and add a replace directive
		// to the project's top-level go.mod so the emitted tree compiles.
		if cfg.HasExternalModules() {
			if err := persistExternalModulesForOutput(cfg, projectDir, instrumentedOutputDir, debug); err != nil {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] Warning: external-module output finalise failed: %v\n", err)
			}
		}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[TIMING] total: %.2fs\n", time.Since(totalStart).Seconds())
	}

	// §239: Before finalising, merge per-package fragment JSONs produced by
	// each toolexec child. Walks the temp dir sequentially — no locking
	// needed because children have exited by now.
	fragmentCount := 0
	if reportFragDir != "" {
		fragmentCount = mergeReportFragments(report.Get(), reportFragDir, debug)
	}
	report.Get().SetFragmentCount(fragmentCount)

	// §240: Timings — same phase durations runFastBuild already prints via
	// [TIMING] debug lines, now persisted to the report.
	totalDuration := time.Since(totalStart).Seconds()
	report.Get().SetTimings(&report.Timings{
		DepSetup:   depSetupDuration,
		PreResolve: preResolveDuration,
		GoBuild:    goBuildDuration,
		Total:      totalDuration,
	})

	// §240: Record build outcome. Success and failure paths both reach here
	// because the os.Exit below runs after this FinalizeReport.
	outcome := &report.BuildOutcome{Success: buildErr == nil}
	if buildErr != nil {
		outcome.Error = buildErr.Error()
		if exitErr, ok := buildErr.(*exec.ExitError); ok {
			outcome.ExitCode = exitErr.ExitCode()
		} else {
			outcome.ExitCode = 1
		}
	}
	report.Get().SetBuildOutcome(outcome)

	// §238: Save report before exit (covers both success and build-failure paths).
	// os.Exit below skips deferred calls, so finalize explicitly here.
	FinalizeReport()

	if buildErr != nil {
		if exitErr, ok := buildErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

// extractGoAPIInfo parses go.mod to find the go-api require + replace state.
// §240.
func extractGoAPIInfo(projectDir string) *report.GoAPIInfo {
	info := &report.GoAPIInfo{}
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return info
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return info
	}
	for _, req := range f.Require {
		if req.Mod.Path == "github.com/whatap/go-api" {
			info.Version = req.Mod.Version
			break
		}
	}
	for _, rep := range f.Replace {
		if rep.Old.Path == "github.com/whatap/go-api" {
			info.Replaced = true
			if rep.New.Path != "" {
				if rep.New.Version != "" {
					info.ReplacePath = rep.New.Path + "@" + rep.New.Version
				} else {
					info.ReplacePath = rep.New.Path
				}
			}
			break
		}
	}
	return info
}

// extractModuleInfo parses go.mod for module path + declared Go version. §240.
func extractModuleInfo(projectDir string) *report.ModuleInfo {
	info := &report.ModuleInfo{}
	goModPath := filepath.Join(projectDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return info
	}
	f, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return info
	}
	if f.Module != nil {
		info.Path = f.Module.Mod.Path
	}
	if f.Go != nil {
		info.GoVersion = f.Go.Version
	}
	return info
}

// buildConfigSnapshot captures the effective instrumentation config for the
// report. §240 — readers of the JSON can see which preset / enabled /
// disabled packages were in effect, external module list, etc., without
// re-reading the original whatap.yaml.
func buildConfigSnapshot(cfg *config.Config, outputDir string, errorTracking bool) *report.ConfigSnapshot {
	snap := &report.ConfigSnapshot{
		OutputDir:     outputDir,
		ErrorTracking: errorTracking,
	}
	if cfg == nil {
		return snap
	}
	snap.EnabledPackages = append([]string(nil), cfg.Instrumentation.EnabledPackages...)
	snap.DisabledPackages = append([]string(nil), cfg.Instrumentation.DisabledPackages...)
	snap.ExternalModules = append([]string(nil), cfg.ExternalModules...)
	snap.CustomRuleCount = len(cfg.Rules)
	if configLoader != nil {
		snap.ConfigPath = configLoader.GetConfigPath()
	}
	return snap
}

// mergeReportFragments walks the fragment dir and merges every *.json fragment
// into the master report. Best-effort — unreadable / malformed fragments are
// skipped with a debug warning so a single bad file cannot swallow the whole
// report. §239.
//
// Returns the number of fragments merged so the caller can surface it in the
// report's summary (§240 Summary.FragmentCount).
func mergeReportFragments(master *report.Report, fragDir string, debug bool) int {
	entries, err := os.ReadDir(fragDir)
	if err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §239: readdir %s failed: %v\n", fragDir, err)
		}
		return 0
	}
	merged := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		fragPath := filepath.Join(fragDir, e.Name())
		frag, err := report.LoadJSONReport(fragPath)
		if err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[whatap-go-inst] §239: load fragment %s failed: %v\n", fragPath, err)
			}
			continue
		}
		master.MergeFragment(frag)
		merged++
	}
	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] §239: merged %d report fragment(s)\n", merged)
	}
	return merged
}

// copyProjectFiles copies project files like go.mod, go.sum
func copyProjectFiles(srcDir, dstDir string) {
	files := []string{"go.mod", "go.sum"}
	for _, file := range files {
		srcPath := filepath.Join(srcDir, file)
		dstPath := filepath.Join(dstDir, file)

		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		os.MkdirAll(dstDir, 0755)
		os.WriteFile(dstPath, data, 0644)
	}
}

// applyAddRulesFast creates files from cfg.Add inside projectDir. Returns the
// absolute paths of created files for deferred removal.
//
// Safety contract (§211):
//   - Never overwrite an existing file — returns an error on conflict so the
//     caller can abort the build and undo previously created files.
//   - content_file is resolved relative to cfg.BaseDir (same as ast/custom/add.go).
//
// The duplicated implementation (vs. ast/custom/add.go) is intentional: that
// package writes to a dstDir copy (wrap/inject) and may overwrite. Fast mode
// writes into the user's live source tree, where overwriting is destructive.
//
// `append: true` was removed in v0.5.5 — the yaml loader rejects it before we
// get here, so rules reaching this point are always new-file creations.
func applyAddRulesFast(projectDir string, cfg *config.Config, debug bool) ([]string, error) {
	var created []string
	for _, rule := range cfg.Add {
		// Resolve target file path under projectDir.
		var filePath string
		if rule.Package == "main" || rule.Package == "." || rule.Package == "" {
			filePath = filepath.Join(projectDir, rule.File)
		} else {
			pkgPath := strings.ReplaceAll(rule.Package, "/", string(filepath.Separator))
			filePath = filepath.Join(projectDir, pkgPath, rule.File)
		}

		// Safety: never overwrite existing files.
		if _, err := os.Stat(filePath); err == nil {
			return created, fmt.Errorf("add rule target already exists (refusing to overwrite): %s", filePath)
		} else if !os.IsNotExist(err) {
			return created, fmt.Errorf("stat %s: %w", filePath, err)
		}

		// Resolve content (inline or content_file).
		content := rule.Content
		if rule.ContentFile != "" {
			contentPath := rule.ContentFile
			if !filepath.IsAbs(contentPath) {
				contentPath = filepath.Join(cfg.BaseDir, contentPath)
			}
			data, err := os.ReadFile(contentPath)
			if err != nil {
				return created, fmt.Errorf("read content_file %s: %w", contentPath, err)
			}
			content = string(data)
		}
		if content == "" {
			continue
		}

		// Create parent directory if missing (we do not track/remove the dir;
		// empty parent dirs stay, which is safer than attempting rmdir).
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return created, fmt.Errorf("mkdir %s: %w", filepath.Dir(filePath), err)
		}

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return created, fmt.Errorf("write %s: %w", filePath, err)
		}
		created = append(created, filePath)

		if debug {
			fmt.Fprintf(os.Stderr, "[whatap-go-inst] §211: add rule wrote %s\n", filePath)
		}
	}
	return created, nil
}
