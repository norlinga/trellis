// Package lint runs structural and design rules over .trellis sidecars.
//
// Two rule kinds:
//
//   - Rule: per-file. The vast majority of checks. Operates on one File
//     (path + source + tree + extracted Sidecar) and emits Diagnostics
//     anchored to AST node ranges.
//   - WorkspaceRule: cross-file. Runs once per lint pass with the whole
//     graph and the file set. Used for rules like duplicate-provides and
//     broken-link that have no sensible per-file expression.
//
// Severity drives the exit code: any Diagnostic with SeverityError causes
// the CLI to exit non-zero. Warnings and info do not.
//
// Diagnostic codes are stable identifiers. Treat them as a public API —
// once a code ships, downstream tooling (editors, CI scripts, policy
// files) will key off it.
package lint

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/norlinga/trellis/internal/graph"
)

// Severity orders diagnostics by attention required.
type Severity uint8

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	}
	return "unknown"
}

// Position is a 1-indexed line and column. Tree-sitter Points are
// 0-indexed; conversion happens at the lint boundary so rules can think
// in human terms throughout.
type Position struct {
	Line   uint
	Column uint
}

// Range is a half-open span [Start, End). When a Diagnostic doesn't have
// a meaningful node to anchor against (e.g. "missing frontmatter key"),
// rules anchor to the file start (1:1).
type Range struct {
	Start, End Position
}

// PositionFromPoint converts a tree-sitter Point to a 1-indexed Position.
func PositionFromPoint(p sitter.Point) Position {
	return Position{Line: uint(p.Row) + 1, Column: uint(p.Column) + 1}
}

// RangeFromNode extracts a 1-indexed Range from a tree-sitter node.
func RangeFromNode(n *sitter.Node) Range {
	return Range{
		Start: PositionFromPoint(n.StartPosition()),
		End:   PositionFromPoint(n.EndPosition()),
	}
}

// FileStart returns the canonical anchor for diagnostics that don't have
// a node to reference (e.g., a missing-required check).
func FileStart() Range {
	return Range{Start: Position{Line: 1, Column: 1}, End: Position{Line: 1, Column: 1}}
}

// Diagnostic is one lint finding.
type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Path     string
	Range    Range
}

// File is the per-file input to a Rule. The Tree is owned by the
// Workspace; rules MUST NOT Close it.
//
// Overrides holds every `@allow-*` frontmatter key declared on this
// sidecar, mapped to the quoted justification text. Rules that support
// overrides consult HasOverride to decide whether to short-circuit.
//
// Frontmatter is a generic map of every frontmatter key on this sidecar
// to its raw value text (quotes preserved). Use FrontmatterValue for
// quote-stripped access. The map includes `@allow-*` keys (also exposed
// via Overrides) so rules that want raw value access don't need a second
// extraction pass.
type File struct {
	Path        string
	Source      []byte
	Tree        *sitter.Tree
	Sidecar     *graph.Sidecar
	Overrides   map[string]string
	Frontmatter map[string]string
}

// Rule is a per-file check.
type Rule interface {
	Code() string
	Check(*File) []Diagnostic
}

// WorkspaceRule is a cross-file check that needs the whole graph.
type WorkspaceRule interface {
	Code() string
	Check([]*File, *graph.Graph) []Diagnostic
}

// Linter coordinates rule execution.
//
// Construct with Default() for the standard rule set, or with New() and
// AddRule/AddWorkspaceRule for custom configurations (tests, future
// policy-file consumption).
type Linter struct {
	rules   []Rule
	wsRules []WorkspaceRule
}

// New returns a Linter with no rules registered.
func New() *Linter { return &Linter{} }

func (l *Linter) AddRule(r Rule)                   { l.rules = append(l.rules, r) }
func (l *Linter) AddWorkspaceRule(r WorkspaceRule) { l.wsRules = append(l.wsRules, r) }

// Lint runs every registered rule against every file, plus every
// workspace rule against the full set. The returned slice is in
// rule-execution order — callers that want a stable display order should
// sort by (Path, Line, Column, Code) before printing.
func (l *Linter) Lint(files []*File, g *graph.Graph) []Diagnostic {
	var out []Diagnostic
	for _, f := range files {
		for _, r := range l.rules {
			out = append(out, r.Check(f)...)
		}
	}
	for _, r := range l.wsRules {
		out = append(out, r.Check(files, g)...)
	}
	return out
}

// PerFileLinter returns a Linter with only per-file rules registered.
// Single-file consumers (the LSP edit-loop, ad-hoc snippet checks)
// should use this instead of Default() because workspace rules without
// workspace context produce false positives or no signal.
func PerFileLinter() *Linter {
	l := New()
	l.AddRule(&FrontmatterRequiredKeys{})
	l.AddRule(&ScenarioKindCanonical{})
	l.AddRule(&MissingNegativeScenario{})
	l.AddRule(&MissingInvariants{})
	l.AddRule(&ScenarioCount{Warn: 10, Error: 15})
	l.AddRule(&ConsumesCount{Warn: 5, Error: 8})
	l.AddRule(&SourceAnchorShape{})
	l.AddRule(NewStaleReviewed())
	return l
}

// Default returns a Linter with the full v1 rule set: per-file plus
// workspace rules. Use this for `trellis lint` and other workspace-aware
// callers that have a graph available.
//
// Policy rules are NOT registered here. To enable them, call
// AddPolicyRules with a parsed Policy after construction.
func Default() *Linter {
	l := PerFileLinter()
	l.AddWorkspaceRule(NewBrokenLink())
	l.AddWorkspaceRule(&DuplicateProvides{})
	l.AddWorkspaceRule(&OrphanSourceFile{})
	return l
}

// AddPolicyRules registers the policy-enforcement workspace rules
// (LayerViolation, StabilityViolation) with the linter. Pass nil or an
// empty Policy to skip — both rule structs short-circuit when their
// rule slice is empty, so registering them with no rules is also a no-op
// but adds noise to rule iteration.
func (l *Linter) AddPolicyRules(p *Policy) {
	if p == nil {
		return
	}
	if len(p.LayerRules) > 0 {
		l.AddWorkspaceRule(&LayerViolation{Policy: p})
	}
	if len(p.StabilityRules) > 0 {
		l.AddWorkspaceRule(&StabilityViolation{Policy: p})
	}
}
