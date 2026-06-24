package common

// Replacement declares a single function replacement mapping.
// Used by DeclarativeTransformer to enable automated validation
// that whatap functions actually exist with correct signatures.
type Replacement struct {
	OriginalPkg  string // Original package import path (e.g., "database/sql")
	OriginalFunc string // Original function name (e.g., "Open")
	WhatapPkg    string // Whatap replacement package (e.g., ".../whatapsql")
	WhatapFunc   string // Whatap replacement function (e.g., "Open")
	Pattern      string // Transformation pattern: "replace", "wrap", "arg-insert", "blank-import"
}

// DeclarativeTransformer is an optional interface for transformers
// that declare their replacement mappings.
// Implementing this enables automated validation via TestAllReplacements.
type DeclarativeTransformer interface {
	Transformer
	Replacements() []Replacement
}
