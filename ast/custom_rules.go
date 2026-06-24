package ast

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/whatap/go-api-inst/config"
)

// LoadCustomRules converts the unified `rules:` array from the user yaml
// (config.Config) into v2 Rules using the same loader pipeline as the
// internal rules.yaml. It supports the full 13 yaml type discriminators
// (10 internal + hook/inject + add) defined in dev-docs/design/v2/rule-yaml-schema.md.
//
// The `add:` type is rejected here because it is handled outside the Engine
// by ast/custom/add.go (legacy path retained per §227 design Q2).
//
// §227 Step 5 (2026-04-14) wired this into the Engine registry via
// injector.buildRegistry / RegisterUser, so user yaml rules now share the
// exact same matching semantics (precise filters, go/types ImportRef
// counting, etc.) as built-in rules — a single Engine pass covers both.
//
// §231 (2026-04-16) applies strict field-name checking to each rule by
// re-serialising the preserved yaml.Node through a KnownFields decoder.
// This catches field typos inside `rules[i]` (e.g. `tpye:` / `tagret:`)
// that would otherwise be silently dropped — yaml.v3 Decoder.KnownFields(true)
// does not propagate through yaml.Node.Decode, so we take a round-trip
// via yaml.Marshal + a fresh Decoder.
func LoadCustomRules(cfg *config.Config) ([]*Rule, error) {
	if cfg == nil {
		return nil, nil
	}
	if len(cfg.Rules) == 0 && len(cfg.Imports) == 0 && len(cfg.ImportAliases) == 0 {
		return nil, nil
	}
	rc := &RulesConfig{
		Version:       cfg.Version,
		Imports:       cfg.Imports,
		ImportAliases: cfg.ImportAliases,
	}
	for i, node := range cfg.Rules {
		spec, err := decodeRuleSpecStrict(&node)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]: %w", i, err)
		}
		// §272 — `reverseTarget` yaml key was removed; the strict decoder
		// above now reports it as an unknown field, so a user yaml carrying
		// the legacy key surfaces as a clear `rules[i]: ...` error here.
		rc.Rules = append(rc.Rules, spec)
	}
	return BuildRules(rc)
}

// decodeRuleSpecStrict re-serialises a preserved yaml.Node and decodes it
// through a KnownFields(true) Decoder so that unknown fields anywhere in
// the RuleSpec tree (including nested SignatureSpec / ReceiverSpec /
// FieldMatchSpec / InsertedArgSpec) surface as explicit errors. See §231.
//
// `type: add` is special-cased: add rules belong in the top-level `add:`
// array (§227 Step 5), not inside `rules:`. A non-strict peek at the
// discriminator lets us emit a clear "handled outside the Engine" error
// instead of leaking unknown-field errors about package/file/content.
func decodeRuleSpecStrict(node *yaml.Node) (RuleSpec, error) {
	var spec RuleSpec
	var peek struct {
		Type string `yaml:"type"`
	}
	_ = node.Decode(&peek) // strict version runs below; ignore peek errors
	if peek.Type == "add" {
		return spec, fmt.Errorf(`type "add" is handled outside the Engine — declare add rules in the top-level "add:" array, not inside "rules:"`)
	}
	raw, err := yaml.Marshal(node)
	if err != nil {
		return spec, fmt.Errorf("marshal: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&spec); err != nil {
		return spec, err
	}
	return spec, nil
}
