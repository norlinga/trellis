package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture lays out a temp workspace with .trellis files plus a
// policy file, returns the workspace root path.
type fixtureFile struct {
	rel  string
	body string
}

func writeFixture(t *testing.T, files ...fixtureFile) string {
	t.Helper()
	root := t.TempDir()
	for _, f := range files {
		path := filepath.Join(root, f.rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(f.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// validSidecar returns a minimal valid .trellis body with the given
// frontmatter and Provides/Consumes blocks. The header is invariant
// across every fixture so changes to required-frontmatter rules don't
// silently flip these tests.
func validSidecar(frontmatter, provides, consumes string) string {
	return frontmatter + `
@since: 2025-01-01
@reviewed: 2026-04-01

Feature: F
  "S."

  Provides:
` + provides + `
  Consumes:
` + consumes + `
  Invariants:
    - the unit MUST behave
`
}

const policyLayer = `layer_dependencies:
  - domain MUST NOT consume infrastructure
`

const policyStability = `stability_tiers:
  - stable MUST NOT consume experimental
`

// findDiagnostic returns the first diagnostic whose Code matches and
// whose Message contains all the given substrings, or nil if none match.
// Used to assert on the presence of specific diagnostics without depending
// on their order in the result slice.
func findDiagnostic(diags []Diagnostic, code string, contains ...string) *Diagnostic {
	for i := range diags {
		if diags[i].Code != code {
			continue
		}
		ok := true
		for _, s := range contains {
			if !strings.Contains(diags[i].Message, s) {
				ok = false
				break
			}
		}
		if ok {
			return &diags[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Layer rule
// ---------------------------------------------------------------------

func TestLayerViolation_FiresOnDeniedEdge(t *testing.T) {
	root := writeFixture(t,
		fixtureFile{
			rel: "domain/user.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable\n@layer: domain",
				"    - User.create(name) -> User\n",
				"    - InfraStore.put(key, value) -> bool\n",
			),
		},
		fixtureFile{
			rel: "infra/store.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable\n@layer: infrastructure",
				"    - InfraStore.put(key, value) -> bool\n",
				"    - os.Stat\n",
			),
		},
		fixtureFile{rel: "policies/arch.trellis-policy", body: policyLayer},
		// Source files so OrphanSourceFile doesn't fire.
		fixtureFile{rel: "domain/user.go", body: "// stub\n"},
		fixtureFile{rel: "infra/store.go", body: "// stub\n"},
	)

	ws, err := LoadWorkspace(root)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	defer ws.Close()

	if len(ws.PolicyErrs) != 0 {
		t.Fatalf("policy parse errors: %v", ws.PolicyErrs)
	}
	if len(ws.Policy.LayerRules) != 1 {
		t.Fatalf("loaded policy has %d layer rules, want 1", len(ws.Policy.LayerRules))
	}

	l := New()
	l.AddPolicyRules(ws.Policy)
	diags := l.Lint(ws.Files, ws.Graph)

	d := findDiagnostic(diags, "policy-layer-violation", "domain", "infrastructure")
	if d == nil {
		t.Fatalf("expected policy-layer-violation diagnostic. all diagnostics:\n%+v", diags)
	}
	if d.Severity != SeverityError {
		t.Errorf("severity = %v, want SeverityError", d.Severity)
	}
	if !strings.HasSuffix(d.Path, "user.go.trellis") {
		t.Errorf("diagnostic anchored to %q, want suffix user.go.trellis (the consumer)", d.Path)
	}
	// Provenance: the diagnostic should cite the policy file that fired the rule.
	if !strings.Contains(d.Message, "arch.trellis-policy") {
		t.Errorf("message missing policy provenance: %s", d.Message)
	}
}

func TestLayerViolation_SkipsEdgesWithoutLayer(t *testing.T) {
	// If either side lacks @layer, the edge is not policy-evaluated.
	// This keeps utility files / cmd entry points / experimental new
	// units out of the policy rule set until they declare a layer.
	root := writeFixture(t,
		fixtureFile{
			rel: "lib/util.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable",
				"    - Util.helper(x) -> y\n",
				"    - InfraStore.put(key, value) -> bool\n",
			),
		},
		fixtureFile{
			rel: "infra/store.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable\n@layer: infrastructure",
				"    - InfraStore.put(key, value) -> bool\n",
				"    - os.Stat\n",
			),
		},
		fixtureFile{rel: "policies/arch.trellis-policy", body: policyLayer},
		fixtureFile{rel: "lib/util.go", body: "// stub\n"},
		fixtureFile{rel: "infra/store.go", body: "// stub\n"},
	)
	ws, _ := LoadWorkspace(root)
	defer ws.Close()
	l := New()
	l.AddPolicyRules(ws.Policy)
	diags := l.Lint(ws.Files, ws.Graph)
	if d := findDiagnostic(diags, "policy-layer-violation"); d != nil {
		t.Errorf("did not expect policy-layer-violation when source lacks @layer; got %+v", d)
	}
}

func TestLayerViolation_DoesNotFireOnAllowedEdge(t *testing.T) {
	// `application MAY consume domain` — and domain has no rule against
	// being consumed by application, so no diagnostic.
	root := writeFixture(t,
		fixtureFile{
			rel: "domain/user.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable\n@layer: domain",
				"    - User.create(name) -> User\n",
				"    - os.Stat\n",
			),
		},
		fixtureFile{
			rel: "app/handler.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable\n@layer: application",
				"    - Handler.do() -> Result\n",
				"    - User.create(name) -> User\n",
			),
		},
		fixtureFile{rel: "policies/arch.trellis-policy", body: policyLayer},
		fixtureFile{rel: "domain/user.go", body: "// stub\n"},
		fixtureFile{rel: "app/handler.go", body: "// stub\n"},
	)
	ws, _ := LoadWorkspace(root)
	defer ws.Close()
	l := New()
	l.AddPolicyRules(ws.Policy)
	diags := l.Lint(ws.Files, ws.Graph)
	if d := findDiagnostic(diags, "policy-layer-violation"); d != nil {
		t.Errorf("did not expect violation for allowed edge; got %+v", d)
	}
}

// ---------------------------------------------------------------------
// Stability rule
// ---------------------------------------------------------------------

func TestStabilityViolation_FiresOnStableConsumingExperimental(t *testing.T) {
	root := writeFixture(t,
		fixtureFile{
			rel: "stable/api.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: stable",
				"    - StableAPI.serve() -> Response\n",
				"    - ExperimentalThing.feature() -> X\n",
			),
		},
		fixtureFile{
			rel: "exp/thing.go.trellis",
			body: validSidecar(
				"@owner: Team\n@stability: experimental",
				"    - ExperimentalThing.feature() -> X\n",
				"    - os.Stat\n",
			),
		},
		fixtureFile{rel: "policies/stab.trellis-policy", body: policyStability},
		fixtureFile{rel: "stable/api.go", body: "// stub\n"},
		fixtureFile{rel: "exp/thing.go", body: "// stub\n"},
	)
	ws, _ := LoadWorkspace(root)
	defer ws.Close()
	l := New()
	l.AddPolicyRules(ws.Policy)
	diags := l.Lint(ws.Files, ws.Graph)

	d := findDiagnostic(diags, "policy-stability-violation", "stable", "experimental")
	if d == nil {
		t.Fatalf("expected policy-stability-violation; got %+v", diags)
	}
	if d.Severity != SeverityError {
		t.Errorf("severity = %v, want SeverityError", d.Severity)
	}
}

// ---------------------------------------------------------------------
// AddPolicyRules registration
// ---------------------------------------------------------------------

func TestAddPolicyRules_NoOpForNilPolicy(t *testing.T) {
	l := New()
	before := len(l.wsRules)
	l.AddPolicyRules(nil)
	if got := len(l.wsRules); got != before {
		t.Errorf("AddPolicyRules(nil) registered %d rules; want 0", got-before)
	}
}

func TestAddPolicyRules_SkipsEmptySections(t *testing.T) {
	// A policy with only stability rules should not register the layer rule
	// (and vice versa). Avoids no-op rules in the iteration loop.
	l := New()
	l.AddPolicyRules(&Policy{
		StabilityRules: []PolicyRule{{Source: "a", Verb: VerbMustNot, Target: "b"}},
	})
	if len(l.wsRules) != 1 {
		t.Errorf("registered %d rules, want 1 (stability only)", len(l.wsRules))
	}
}
