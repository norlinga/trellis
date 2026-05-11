package parser_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/norlinga/trellis/internal/parser"
)

func TestParseProgressive(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"feature only", "Feature: Hi\n  \"summary\"\n"},
		{"feature + provides", "Feature: Hi\n  \"summary\"\n\n  Provides:\n    - Foo\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := parser.Parse([]byte(tc.src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			defer tree.Close()
			if tree.RootNode().HasError() {
				t.Fatalf("HasError on %q:\n%s", tc.src, tree.RootNode().ToSexp())
			}
		})
	}
}

// TestParseExampleFixture is the smoke test for the whole parsing pipeline:
// load grammar, parse the whitepaper-style example, confirm a clean tree.
// If this breaks, the grammar repo has drifted or the binding has changed.
func TestParseExampleFixture(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "valid", "create_subscription.rb.trellis")
	tree, src, err := parser.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("parse errors in fixture:\n%s", root.ToSexp())
	}
	if got := root.Kind(); got != "source_file" {
		t.Fatalf("root kind = %q, want %q", got, "source_file")
	}
	if root.NamedChildCount() == 0 {
		t.Fatalf("source_file has no named children; src len=%d", len(src))
	}

	feature := root.ChildByFieldName("feature")
	if feature == nil {
		t.Fatalf("no feature field on source_file; tree:\n%s", root.ToSexp())
	}
	if !strings.Contains(root.ToSexp(), "scenario_block") {
		t.Fatalf("expected at least one scenario_block in tree:\n%s", root.ToSexp())
	}
}
