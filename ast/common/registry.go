// Package common provides shared utilities for AST transformations.
package common

import (
	"github.com/dave/dst"
)

// Transformer is the per-package transformer interface
// Each package (gin, echo, sql, etc.) implements this interface to inject/remove instrumentation code.
type Transformer interface {
	// Name returns the transformer name (e.g., "gin", "echo", "sql")
	Name() string

	// ImportPath returns the original package import path (e.g., "github.com/gin-gonic/gin")
	ImportPath() string

	// WhatapImport returns the whatap instrumentation import path (e.g., ".../whatapgin")
	WhatapImport() string

	// Detect checks if the package is used in the file
	Detect(file *dst.File) bool

	// Inject injects instrumentation code
	// Returns: (whether transformation occurred, error)
	// - bool: true if code transformation actually happened, false otherwise
	// - error: returns error if occurred
	// Note: if bool is false, whatap import will not be added
	Inject(file *dst.File) (bool, error)

	// Remove removes instrumentation code
	Remove(file *dst.File) error
}

// TypedTransformer is an extended interface for transformers that use go/types
// Automatically infers method return types to generate version-independent instrumentation code.
type TypedTransformer interface {
	Transformer
	// InjectWithDir injects instrumentation code using go/types type information
	// dir is the directory path where the source file is located.
	// Returns: (whether transformation occurred, error)
	InjectWithDir(file *dst.File, dir string) (bool, error)
}

// AliasedImportTransformer is an extended interface for transformers that need import alias
// Used when WhatapImport() may conflict with other packages.
// e.g., github.com/whatap/go-api/sql conflicts with database/sql → needs whatapsql alias
type AliasedImportTransformer interface {
	Transformer
	// WhatapImportAlias returns the alias for whatap import (e.g., "whatapsql")
	WhatapImportAlias() string
}

// Global registry
var registry = make(map[string]Transformer)

// Register registers a transformer to the registry
// Called from each package's init() function.
func Register(t Transformer) {
	registry[t.Name()] = t
}

// GetTransformer retrieves a transformer by name
func GetTransformer(name string) Transformer {
	return registry[name]
}

// GetAllTransformers retrieves all registered transformers
func GetAllTransformers() []Transformer {
	list := make([]Transformer, 0, len(registry))
	for _, t := range registry {
		list = append(list, t)
	}
	return list
}

// GetDetectedTransformers retrieves transformers for packages detected in file
func GetDetectedTransformers(file *dst.File) []Transformer {
	var list []Transformer
	for _, t := range registry {
		if t.Detect(file) {
			list = append(list, t)
		}
	}
	return list
}

// GetTransformerNames retrieves all registered transformer names
func GetTransformerNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// HasTransformer checks if a transformer is registered
func HasTransformer(name string) bool {
	_, ok := registry[name]
	return ok
}

// ClearRegistry clears the registry (for testing)
func ClearRegistry() {
	registry = make(map[string]Transformer)
}

// GetEnabledTransformers retrieves transformers for enabled packages only
// If enabledPackages is nil, returns all transformers (backward compatibility)
func GetEnabledTransformers(enabledPackages []string) []Transformer {
	if enabledPackages == nil {
		return GetAllTransformers()
	}

	// Convert enabledPackages to map for O(1) lookup
	enabled := make(map[string]bool, len(enabledPackages))
	for _, pkg := range enabledPackages {
		enabled[pkg] = true
	}

	var list []Transformer
	for _, t := range registry {
		if enabled[t.Name()] {
			list = append(list, t)
		}
	}
	return list
}

// GetFilteredTransformers retrieves enabled transformers from packages detected in file
// If enabledPackages is nil, returns all detected transformers (backward compatibility)
func GetFilteredTransformers(file *dst.File, enabledPackages []string) []Transformer {
	if enabledPackages == nil {
		return GetDetectedTransformers(file)
	}

	// Convert enabledPackages to map for O(1) lookup
	enabled := make(map[string]bool, len(enabledPackages))
	for _, pkg := range enabledPackages {
		enabled[pkg] = true
	}

	var list []Transformer
	for _, t := range registry {
		if enabled[t.Name()] && t.Detect(file) {
			list = append(list, t)
		}
	}
	return list
}
