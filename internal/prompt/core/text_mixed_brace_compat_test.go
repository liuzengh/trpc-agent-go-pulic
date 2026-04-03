//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

package promptcore

import (
	"regexp"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers that reproduce the old state_injection.go logic
// ---------------------------------------------------------------------------

var oldMustachePlaceholderRE = regexp.MustCompile(
	`\{\{\s*([A-Za-z_][A-Za-z0-9_]*:(?:[A-Za-z_][A-Za-z0-9_]*)|[A-Za-z_][A-Za-z0-9_]*)(\?)?\s*\}\}`,
)

var oldSingleBraceRE = regexp.MustCompile(`\{([^{}]+)\}`)

func oldNormalizePlaceholders(s string) string {
	if s == "" {
		return s
	}
	return oldMustachePlaceholderRE.ReplaceAllString(s, `{$1$2}`)
}

func oldIsIdentifier(s string) bool {
	if s == "" {
		return false
	}
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_') {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func oldIsValidStateName(varName string) bool {
	if varName == "" {
		return false
	}
	if strings.HasPrefix(varName, "artifact.") {
		return true
	}
	if oldIsIdentifier(varName) {
		return true
	}
	parts := strings.Split(varName, ":")
	if len(parts) == 2 {
		prefix := parts[0] + ":"
		for _, vp := range []string{"app:", "user:", "temp:", "invocation:"} {
			if prefix == vp {
				return oldIsIdentifier(parts[1])
			}
		}
	}
	return false
}

func oldInjectSimulation(template string, vars map[string]string) string {
	if template == "" {
		return template
	}
	template = oldNormalizePlaceholders(template)

	return oldSingleBraceRE.ReplaceAllStringFunc(template, func(match string) string {
		varName := strings.Trim(match, "{}")
		optional := false
		if strings.HasSuffix(varName, "?") {
			optional = true
			varName = strings.TrimSuffix(varName, "?")
		}
		if strings.HasPrefix(varName, "artifact.") {
			if optional {
				return ""
			}
			return match
		}
		if !oldIsValidStateName(varName) {
			return match
		}
		if value, ok := vars[varName]; ok {
			return value
		}
		if optional {
			return ""
		}
		return match
	})
}

func newRender(template string, vars map[string]string) string {
	rendered, _ := Render(
		template,
		SyntaxModeMixedBrace,
		Env{Vars: vars},
		PreserveUnknown,
		WithAcceptName(stateSubsetAcceptName),
	)
	return rendered
}

// ---------------------------------------------------------------------------
// Compatibility tests: old and new MUST produce identical output.
// ---------------------------------------------------------------------------

func TestMixedBraceCompat_BasicSingleBrace(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"simple replacement", "Hello {name}!", map[string]string{"name": "Alice"}},
		{"multiple replacements", "The {country} capital is {capital_city}.", map[string]string{"country": "France", "capital_city": "Paris"}},
		{"optional present", "Hello {name?}!", map[string]string{"name": "Alice"}},
		{"optional missing", "Hello {name?}!", map[string]string{}},
		{"non-optional missing preserved", "Hello {name}!", map[string]string{}},
		{"mixed optional and non-optional", "Hello {name?}, age {age}.", map[string]string{}},
		{"prefixed app", "Config: {app:setting}", map[string]string{"app:setting": "dark"}},
		{"prefixed user", "Pref: {user:preference}", map[string]string{"user:preference": "compact"}},
		{"prefixed temp", "Tmp: {temp:value}", map[string]string{"temp:value": "ctx"}},
		{"invocation prefix", "Case: {invocation:case_id}", map[string]string{"invocation:case_id": "c-1"}},
		{"underscore-prefixed name", "V: {_private}", map[string]string{"_private": "secret"}},
		{"artifact non-optional", "File: {artifact.report.txt}", map[string]string{}},
		{"artifact optional", "File: {artifact.report.txt?}", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_DoubleBraceResolved(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"simple resolved", "Hello {{name}}!", map[string]string{"name": "Alice"}},
		{"with spaces resolved", "Hello {{ name }}!", map[string]string{"name": "Alice"}},
		{"optional present", "Hello {{name?}}!", map[string]string{"name": "Alice"}},
		{"optional missing", "Hello {{name?}}!", map[string]string{}},
		{"namespace resolved", "Pref: {{user:preference}}", map[string]string{"user:preference": "dark"}},
		{"namespace with spaces resolved", "Pref: {{ user:preference }}", map[string]string{"user:preference": "dark"}},
		{"optional namespace missing", "Pref: {{user:preference?}}", map[string]string{}},
		{"optional namespace with spaces missing", "Pref: {{ user:preference? }}", map[string]string{}},
		{"invocation prefix resolved", "Case: {{invocation:case_id}}", map[string]string{"invocation:case_id": "c-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_InvalidNames(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"hyphen single brace", "Hello {invalid-name}!", map[string]string{"invalid-name": "v"}},
		{"hyphen double brace", "Hello {{invalid-name}}!", map[string]string{"invalid-name": "v"}},
		{"numeric leading single", "Hello {123name}!", map[string]string{"123name": "v"}},
		{"numeric leading double", "Hello {{123name}}!", map[string]string{"123name": "v"}},
		{"unknown prefix single", "Hello {unknown:name}!", map[string]string{"unknown:name": "v"}},
		{"invalid optional single", "Hello {invalid-name?}!", map[string]string{}},
		{"invalid optional double", "Hello {{invalid-name?}}!", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_WhitespaceCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"single brace spaces stays literal", "hi { name } there", map[string]string{"name": "alice"}},
		{"single brace optional spaces stays literal", "hi { name ? } there", map[string]string{}},
		{"double brace spaces resolves", "hi {{ name }} there", map[string]string{"name": "alice"}},
		{"double brace optional spaces collapses", "hi {{ name? }} there", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_MixedDelimitersResolved(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"both resolved", "Hello {name} from {{city}}", map[string]string{"name": "alice", "city": "paris"}},
		{"double resolved single unresolved", "Hello {name} from {{city}}", map[string]string{"city": "paris"}},
		{"namespaces resolved", "User {user:name} config {{app:setting}}", map[string]string{"user:name": "alice", "app:setting": "on"}},
		{"optional in both empty", "A {opt1?} B {{opt2?}} C", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"empty braces single", "hello {} world", map[string]string{}},
		{"empty braces double", "hello {{}} world", map[string]string{}},
		{"adjacent double braces", "{{name}}{{city}}", map[string]string{"name": "alice", "city": "paris"}},
		{"nested braces attempted", "hello {{na{me}}} world", map[string]string{"name": "alice"}},
		{"no closing brace", "hello {name world", map[string]string{"name": "alice"}},
		{"only opening double brace", "hello {{name world", map[string]string{"name": "alice"}},
		{"plain text", "Hello, world!", map[string]string{}},
		{"empty template", "", map[string]string{}},
		{"value contains braces no recursion", "Result: {name}", map[string]string{"name": "{city}"}},
		{"value contains double braces no recursion", "Result: {name}", map[string]string{"name": "{{city}}"}},
		{"multiple question marks", "hello {na?me?} world", map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_DoubleOptionalWithSpace(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"space before question mark stays literal", "hi {{ name ? }} there", map[string]string{"name": "alice"}},
		{"space after name optional collapses", "hi {{ name? }} there", map[string]string{}},
		{"space after name optional resolves", "hi {{ name? }} there", map[string]string{"name": "alice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_IncompleteDoubleBrace(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"name present", "prefix {{name}", map[string]string{"name": "alice"}},
		{"name absent", "prefix {{name}", map[string]string{}},
		{"optional missing", "prefix {{name?}", map[string]string{}},
		{"optional present", "prefix {{name?}", map[string]string{"name": "alice"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_NamespacedEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{"empty prefix single", "hello {:value}!", map[string]string{":value": "v"}},
		{"empty suffix single", "hello {user:}!", map[string]string{"user:": "v"}},
		{"empty prefix double", "hello {{:value}}!", map[string]string{":value": "v"}},
		{"empty suffix double", "hello {{user:}}!", map[string]string{"user:": "v"}},
		{"multiple colons single", "hello {user:name:extra}!", map[string]string{"user:name:extra": "v"}},
		{"multiple colons double", "hello {{user:name:extra}}!", map[string]string{"user:name:extra": "v"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_RealWorldTemplates(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{
			"agent instructions with state",
			"You are a customer service agent.\n" +
				"Customer name: {user:name}\n" +
				"Issue category: {app:category}\n" +
				"Previous context: {temp:context?}\n" +
				"Please help the customer with their issue.",
			map[string]string{
				"user:name":    "Alice",
				"app:category": "billing",
			},
		},
		{
			"agno-style mustache template resolved",
			"You are a research assistant.\n" +
				"Topic: {{research_topic}}\n" +
				"Max results: {{max_results?}}\n" +
				"User preferences: {{ user:preferences? }}\n" +
				"Please summarize the latest findings.",
			map[string]string{
				"research_topic": "quantum computing",
			},
		},
		{
			"mixed template with JSON-like content",
			"Process this data: {input_data}\n" +
				"Config: {\"key\": \"value\", \"count\": 5}\n" +
				"Output format: {output_format?}",
			map[string]string{
				"input_data": "test data",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

// ---------------------------------------------------------------------------
// Formerly divergent cases: these used to differ between old and new
// systems. After restoring backward compatibility, the new system now
// normalizes unresolved double-brace placeholders to single-brace form
// to match the legacy state-injection behavior.
//
// The triple-brace case ({{{name}}}) remains intentionally different:
// the old regex cascade accidentally resolved it, while the new parser
// correctly treats it as opaque literal text.
// ---------------------------------------------------------------------------

func TestMixedBraceCompat_UnresolvedDoubleBraceNormalized(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		{
			name:     "unresolved double brace bare name",
			template: "Hello {{name}}!",
			vars:     map[string]string{},
		},
		{
			name:     "mixed: single resolved double unresolved",
			template: "Hello {name} from {{city}}",
			vars:     map[string]string{"name": "alice"},
		},
		{
			name:     "unresolved double brace with namespace",
			template: "Hello {{name}} from {{user:city}}",
			vars:     map[string]string{"name": "alice"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

func TestMixedBraceCompat_MultiBraces(t *testing.T) {
	vars := map[string]string{"name": "alice"}
	empty := map[string]string{}

	tests := []struct {
		name     string
		template string
		vars     map[string]string
	}{
		// {{{name}}} — 3 braces
		{"3brace resolved", "A {{{name}}} B", vars},
		{"3brace unresolved", "A {{{name}}} B", empty},
		{"3brace optional resolved", "A {{{name?}}} B", vars},
		{"3brace optional unresolved", "A {{{name?}}} B", empty},
		{"3brace namespace resolved", "A {{{user:name}}} B", map[string]string{"user:name": "alice"}},
		{"3brace namespace unresolved", "A {{{user:name}}} B", empty},
		// {{{{name}}}} — 4 braces
		{"4brace resolved", "A {{{{name}}}} B", vars},
		{"4brace unresolved", "A {{{{name}}}} B", empty},
		// {{{name}}}} — 3 open, 4 close
		{"3o4c resolved", "A {{{name}}}} B", vars},
		{"3o4c unresolved", "A {{{name}}}} B", empty},
		// {{{{name}}} — 4 open, 3 close
		{"4o3c resolved", "A {{{{name}}} B", vars},
		{"4o3c unresolved", "A {{{{name}}} B", empty},
		// {{{{{name}}}}} — 5 braces
		{"5brace resolved", "A {{{{{name}}}}} B", vars},
		{"5brace unresolved", "A {{{{{name}}}}} B", empty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertCompat(t, tt.template, tt.vars)
		})
	}
}

// ---------------------------------------------------------------------------
// Additional targeted scenarios for MixedBrace correctness.
// ---------------------------------------------------------------------------

func TestMixedBrace_DoubleBraceNameExtractedWithoutBraces(t *testing.T) {
	names := PlaceholderNames("{{user:name}} {{app:setting?}}", SyntaxModeMixedBrace)
	want := []string{"app:setting", "user:name"}
	if len(names) != len(want) {
		t.Fatalf("PlaceholderNames: got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("PlaceholderNames[%d]: got %q, want %q", i, names[i], want[i])
		}
	}
}

func TestMixedBrace_SingleAndDoubleBraceYieldSameName(t *testing.T) {
	single := PlaceholderNames("{name} {user:city?}", SyntaxModeMixedBrace)
	double := PlaceholderNames("{{name}} {{user:city?}}", SyntaxModeMixedBrace)
	if len(single) != len(double) {
		t.Fatalf("length mismatch: single=%v double=%v", single, double)
	}
	for i := range single {
		if single[i] != double[i] {
			t.Fatalf("name[%d]: single=%q double=%q", i, single[i], double[i])
		}
	}
}

func TestMixedBrace_ErrorOnUnknownNormalizesToSingleBrace(t *testing.T) {
	rendered, err := Render(
		"Hello {{name}} from {{city}}",
		SyntaxModeMixedBrace,
		Env{Vars: map[string]string{"name": "alice"}},
		ErrorOnUnknown,
	)
	if err == nil {
		t.Fatal("expected error for unresolved {city}")
	}
	if !strings.Contains(err.Error(), "{city}") {
		t.Fatalf("error should reference {city}: %v", err)
	}
	const want = "Hello alice from {city}"
	if rendered != want {
		t.Fatalf("got %q, want %q", rendered, want)
	}
}

func TestMixedBrace_ResolveCallbackReceivesBareNames(t *testing.T) {
	var seen []string
	_, _ = Render(
		"{{user:name}} {city} {{app:theme?}}",
		SyntaxModeMixedBrace,
		Env{
			Resolve: func(name string) (string, bool, error) {
				seen = append(seen, name)
				return "", false, nil
			},
		},
		PreserveUnknown,
	)
	want := []string{"user:name", "city", "app:theme"}
	if len(seen) != len(want) {
		t.Fatalf("resolve calls: got %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("resolve[%d]: got %q, want %q", i, seen[i], want[i])
		}
	}
}

func TestMixedBrace_ConsecutivePlaceholders(t *testing.T) {
	rendered, err := Render(
		"{a}{b}{{c}}{{d}}",
		SyntaxModeMixedBrace,
		Env{Vars: map[string]string{
			"a": "1", "b": "2", "c": "3", "d": "4",
		}},
		PreserveUnknown,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rendered != "1234" {
		t.Fatalf("got %q, want %q", rendered, "1234")
	}
}

func TestMixedBrace_LongValidIdentifier(t *testing.T) {
	name := "a" + strings.Repeat("b", 200)
	template := "{" + name + "}"
	rendered, err := Render(
		template,
		SyntaxModeMixedBrace,
		Env{Vars: map[string]string{name: "ok"}},
		PreserveUnknown,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rendered != "ok" {
		t.Fatalf("got %q, want %q", rendered, "ok")
	}
}

func TestMixedBrace_NonASCIILetterNames(t *testing.T) {
	rendered, err := Render(
		"hi {naïve} and {名前}",
		SyntaxModeMixedBrace,
		Env{Vars: map[string]string{
			"naïve": "val1",
			"名前":    "val2",
		}},
		PreserveUnknown,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rendered != "hi val1 and val2" {
		t.Fatalf("got %q, want %q", rendered, "hi val1 and val2")
	}
}

// assertCompat verifies that old and new systems produce identical output.
func assertCompat(t *testing.T, template string, vars map[string]string) {
	t.Helper()
	oldResult := oldInjectSimulation(template, vars)
	newResult := newRender(template, vars)
	if oldResult != newResult {
		t.Errorf(
			"mismatch for template %q:\n  old: %q\n  new: %q",
			template, oldResult, newResult,
		)
	}
}
