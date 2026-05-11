package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// PolicyVerb discriminates a rule's directive.
//
// VerbMustNot is enforced — a violating Consumes edge becomes a
// `policy-*-violation` diagnostic at error severity.
//
// VerbMay is parsed and validated for source/target consistency, but
// currently emits no diagnostic. It is documentation today and the
// substrate for a future "default-deny" mode where rules without an
// explicit MAY are presumed forbidden. Authoring with MAY rules now
// costs nothing and pays off when that mode lands.
type PolicyVerb int

const (
	VerbMustNot PolicyVerb = iota
	VerbMay
)

func (v PolicyVerb) String() string {
	switch v {
	case VerbMustNot:
		return "MUST NOT"
	case VerbMay:
		return "MAY"
	}
	return "?"
}

// PolicyRule is one parsed predicate. Source and Target name a layer or
// stability tier (depending on the section). From/Line are the
// provenance, used to anchor diagnostics back to the policy file.
type PolicyRule struct {
	Source string
	Verb   PolicyVerb
	Target string
	From   string // policy file path
	Line   int    // 1-indexed line number
}

// Policy is the in-memory representation of one or more parsed
// .trellis-policy files. Rule kinds are kept in separate slices so the
// workspace rules can iterate the relevant set without filtering.
//
// An empty Policy (no rules in any section) is valid and treats every
// dependency edge as permitted — i.e., it disables enforcement without
// disabling discovery.
type Policy struct {
	LayerRules     []PolicyRule
	StabilityRules []PolicyRule
}

// PolicyParseError is one issue in one .trellis-policy file. Line is
// 1-indexed; 0 means the error was not anchored to a line (e.g., file
// could not be read).
type PolicyParseError struct {
	Path    string
	Line    int
	Message string
}

func (e PolicyParseError) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
}

// ---------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------

// Recognized section names. Adding a new section means: add the constant,
// add a slice on Policy, route it in ParsePolicy, and add a workspace
// rule that consumes it.
const (
	sectionLayerDependencies = "layer_dependencies"
	sectionStabilityTiers    = "stability_tiers"
)

var (
	// Section header: bareword key followed by `:` at end-of-line.
	// Indented lines never match — section headers must be at column 0.
	sectionHeaderRE = regexp.MustCompile(`^([a-z_]+):\s*$`)

	// Rule entry: any indentation, then `-` and the predicate body.
	ruleEntryRE = regexp.MustCompile(`^\s+-\s+(.+?)\s*$`)

	// Predicate: `<source> <verb> consume <target>` with `MUST NOT` as a
	// two-word verb. Source and target are bareword identifiers.
	predicateRE = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*)\s+(MUST NOT|MAY)\s+consume\s+([A-Za-z][A-Za-z0-9_-]*)\s*$`)
)

// ParsePolicy parses src as the contents of a .trellis-policy file.
// path is recorded on every rule for diagnostic provenance and is also
// the value reported in returned PolicyParseErrors. Pass an empty path
// in tests where provenance doesn't matter.
//
// The parser is intentionally line-oriented: the file format is shallow
// enough that introducing a YAML dependency would add weight for no
// readability gain. Comments (`#`) and blank lines are ignored. An
// unrecognized section name is a fatal error for that file (the rest is
// not parsed) — silent acceptance would let a typo become "no rules in
// the typo-named section" without anyone noticing.
func ParsePolicy(path string, src []byte) (*Policy, []PolicyParseError) {
	p := &Policy{}
	var errs []PolicyParseError

	currentSection := ""
	for i, raw := range strings.Split(string(src), "\n") {
		line := i + 1
		trimmed := strings.TrimRight(raw, "\r")
		if trimmed == "" || strings.HasPrefix(strings.TrimSpace(trimmed), "#") {
			continue
		}
		if m := sectionHeaderRE.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			switch name {
			case sectionLayerDependencies, sectionStabilityTiers:
				currentSection = name
			default:
				errs = append(errs, PolicyParseError{
					Path:    path,
					Line:    line,
					Message: fmt.Sprintf("unknown section %q (recognized: %s, %s)", name, sectionLayerDependencies, sectionStabilityTiers),
				})
				return p, errs
			}
			continue
		}
		if m := ruleEntryRE.FindStringSubmatch(trimmed); m != nil {
			if currentSection == "" {
				errs = append(errs, PolicyParseError{
					Path:    path,
					Line:    line,
					Message: "rule entry appears before any section header",
				})
				continue
			}
			rule, ok := parsePredicate(m[1])
			if !ok {
				errs = append(errs, PolicyParseError{
					Path:    path,
					Line:    line,
					Message: fmt.Sprintf("invalid predicate %q (expected: '<source> MUST NOT consume <target>' or '<source> MAY consume <target>')", m[1]),
				})
				continue
			}
			rule.From = path
			rule.Line = line
			switch currentSection {
			case sectionLayerDependencies:
				p.LayerRules = append(p.LayerRules, rule)
			case sectionStabilityTiers:
				p.StabilityRules = append(p.StabilityRules, rule)
			}
			continue
		}
		errs = append(errs, PolicyParseError{
			Path:    path,
			Line:    line,
			Message: fmt.Sprintf("unrecognized line %q (expected section header `name:`, indented rule `- ...`, or comment `#...`)", trimmed),
		})
	}
	return p, errs
}

func parsePredicate(s string) (PolicyRule, bool) {
	m := predicateRE.FindStringSubmatch(s)
	if m == nil {
		return PolicyRule{}, false
	}
	verb := VerbMay
	if m[2] == "MUST NOT" {
		verb = VerbMustNot
	}
	return PolicyRule{Source: m[1], Verb: verb, Target: m[3]}, true
}

// LoadPolicyFile reads and parses a single .trellis-policy file from
// disk. Returns an empty Policy plus a single error entry if the file
// cannot be read.
func LoadPolicyFile(path string) (*Policy, []PolicyParseError) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &Policy{}, []PolicyParseError{{Path: path, Line: 0, Message: err.Error()}}
	}
	return ParsePolicy(path, data)
}

// MergePolicies combines policies from multiple files. Rules are
// concatenated in the order policies are passed; provenance (From/Line)
// on each rule disambiguates which file it came from.
//
// No deduplication: two files declaring the same rule yield two rules,
// and downstream diagnostics may fire twice. That's intentional — a
// duplicate rule from two policy files is an organizational signal worth
// surfacing, and v1 errs on the side of visibility.
func MergePolicies(policies []*Policy) *Policy {
	out := &Policy{}
	for _, p := range policies {
		if p == nil {
			continue
		}
		out.LayerRules = append(out.LayerRules, p.LayerRules...)
		out.StabilityRules = append(out.StabilityRules, p.StabilityRules...)
	}
	return out
}

// ---------------------------------------------------------------------
// Discovery
// ---------------------------------------------------------------------

// PolicyExtension is the file extension that DiscoverPolicyFiles matches.
const PolicyExtension = ".trellis-policy"

// DiscoverPolicyFiles walks each root for *.trellis-policy files,
// applying these rules:
//
//   - Hidden directories (leading `.`) are skipped.
//   - Directories named `examples` are skipped — example packs are
//     reference material, not active rules. Authors can opt back in by
//     passing the example file path explicitly.
//   - Symlinks are not followed.
//   - Output is deduplicated by absolute path and sorted.
func DiscoverPolicyFiles(roots []string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		out = append(out, abs)
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}
		if !info.IsDir() {
			if strings.HasSuffix(root, PolicyExtension) {
				add(root)
			}
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path == root {
					return nil
				}
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "examples" {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), PolicyExtension) {
				add(path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}
	}
	sort.Strings(out)
	return out, nil
}
