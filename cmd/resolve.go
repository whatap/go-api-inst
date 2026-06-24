package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ResolveCache holds pre-resolved whatap package archive paths.
// Created once by runFastBuild and read by each toolexec invocation.
type ResolveCache struct {
	// Packages maps import path to compiled archive (.a) file path.
	Packages map[string]string `json:"packages"`
	// ReplacedModules lists go.mod replace directive module paths (§205).
	// Transformers skip injection for these modules to avoid build failures.
	ReplacedModules []string `json:"replacedModules,omitempty"`
}

// goListEntry is a subset of `go list -json -export` output.
type goListEntry struct {
	ImportPath string `json:"ImportPath"`
	Export     string `json:"Export"`
}

// preResolveWhatapPackages runs `go list -json -export -e github.com/whatap/go-api/...`
// and returns a ResolveCache mapping import paths to their .a archive paths.
// The -e flag makes it continue on errors (e.g., instrumentation packages whose
// external dependencies are not in the app's go.sum). Only successfully compiled
// packages are included in the cache.
//
// §174: No separate GOCACHE needed. Pre-resolve uses the real build cache.
// Since toolexec only transforms project source and external-module packages
// (not third-party libraries like gin), pre-resolve's cached artifacts are
// compatible with the main build — library fingerprints remain unchanged.
func preResolveWhatapPackages(projectDir string, debug bool) (*ResolveCache, error) {
	// -deps: include transitive dependencies (needed for linker importcfg)
	// §191: Use -mod=vendor for vendor projects (whatap packages are now in vendor/
	// via Orchestrion pattern: tool file → tidy → vendor). This ensures pre-resolved
	// archives match the actual vendor build, avoiding fingerprint mismatch.
	modFlag := "-mod=mod"
	if isVendorProject(projectDir, nil) {
		modFlag = "-mod=vendor"
	}
	// §270: LLM 어댑터는 별도 nested module (github.com/whatap/go-api/instrumentation/llm)
	// 이므로 본체 패턴 (github.com/whatap/go-api/...) 에 안 잡힘. 별도 인자로 추가.
	// `-e` flag 가 사용자 go.mod 에 LLM module require 없을 때 not-found 흡수.
	cmd := exec.Command("go", "list", modFlag, "-json", "-export", "-e", "-deps",
		"github.com/whatap/go-api/...",
		"github.com/whatap/go-api/instrumentation/llm/...")
	cmd.Dir = projectDir
	cmd.Stderr = os.Stderr

	// cmd.Output() returns stdout even when the process exits non-zero.
	// With -e flag, go list reports errors as JSON fields but may still exit 1.
	// We process whatever output was produced regardless of exit code.
	out, _ := cmd.Output()

	cache := &ResolveCache{
		Packages: make(map[string]string),
	}

	// go list -json outputs a stream of JSON objects (not an array).
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var entry goListEntry
		if err := dec.Decode(&entry); err != nil {
			// Skip malformed entries.
			continue
		}
		if entry.ImportPath != "" && entry.Export != "" {
			cache.Packages[entry.ImportPath] = entry.Export
		}
	}

	// §191: Also pre-resolve stdlib packages that the injector may add as new imports.
	// Without this, toolexec adds e.g. "os" import but importcfg has no archive for it,
	// causing "could not import os" compiler error.
	stdlibPkgs := []string{"os", "context", "fmt", "log", "net/http"}
	stdCmd := exec.Command("go", "list", "-json", "-export", "-e")
	stdCmd.Args = append(stdCmd.Args, stdlibPkgs...)
	stdCmd.Dir = projectDir
	if stdOut, err := stdCmd.Output(); err == nil {
		stdDec := json.NewDecoder(strings.NewReader(string(stdOut)))
		for stdDec.More() {
			var entry goListEntry
			if err := stdDec.Decode(&entry); err != nil {
				continue
			}
			if entry.ImportPath != "" && entry.Export != "" {
				cache.Packages[entry.ImportPath] = entry.Export
			}
		}
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[whatap-go-inst] Resolved %d packages (whatap + stdlib)\n", len(cache.Packages))
	}

	if len(cache.Packages) == 0 {
		return nil, fmt.Errorf("no whatap packages resolved (go list returned no compilable packages)")
	}

	return cache, nil
}

// writeCacheFile writes the ResolveCache to a temporary JSON file
// and returns the file path.
func writeCacheFile(cache *ResolveCache) (string, error) {
	f, err := os.CreateTemp("", "whatap-resolve-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp cache file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	enc := json.NewEncoder(w)
	if err := enc.Encode(cache); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("encode cache: %w", err)
	}
	if err := w.Flush(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("flush cache: %w", err)
	}

	return f.Name(), nil
}

// readCacheFile reads a ResolveCache from the given JSON file path.
func readCacheFile(path string) (*ResolveCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache file: %w", err)
	}

	cache := &ResolveCache{}
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, fmt.Errorf("decode cache: %w", err)
	}

	return cache, nil
}

// lookupArchive looks up the compiled archive path for the given import path.
// Returns empty string if not found.
func (c *ResolveCache) lookupArchive(importPath string) string {
	if c == nil || c.Packages == nil {
		return ""
	}
	return c.Packages[importPath]
}
