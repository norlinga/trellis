package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/norlinga/trellis/internal/lint"
)

// loadDir runs the workspace loader against a directory and returns the
// loaded workspace. Closes Trees via t.Cleanup.
func loadDir(t *testing.T, dir string) *lint.Workspace {
	t.Helper()
	ws, err := lint.LoadWorkspace(dir)
	if err != nil {
		t.Fatalf("LoadWorkspace: %v", err)
	}
	t.Cleanup(ws.Close)
	return ws
}

func writeSidecar(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------
// BrokenLink
// ---------------------------------------------------------------------

func TestBrokenLink_FiresOnUnresolvedNonExternal(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "x.go.trellis"), `Feature: X
  "consumes a thing nobody provides"

  Consumes:
    - MyTeam.thing
`)
	ws := loadDir(t, dir)
	diags := lint.NewBrokenLink().Check(ws.Files, ws.Graph)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "MyTeam.thing") {
		t.Errorf("message missing handle name: %q", diags[0].Message)
	}
}

func TestBrokenLink_SilentOnExternalPrefix(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "x.go.trellis"), `Feature: X
  "consumes stdlib only"

  Consumes:
    - os.Stat
    - filepath.Abs
    - cobra.Command
`)
	ws := loadDir(t, dir)
	diags := lint.NewBrokenLink().Check(ws.Files, ws.Graph)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diags (all external), got %d: %v", len(diags), diags)
	}
}

func TestBrokenLink_FiresOnUnresolvedPrefixedHandle(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "x.go.trellis"), `Feature: X
  "consumes an unresolved Event"

  Consumes:
    - Event: subscription.cancelled
`)
	ws := loadDir(t, dir)
	diags := lint.NewBrokenLink().Check(ws.Files, ws.Graph)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag for prefixed handle, got %d: %v", len(diags), diags)
	}
	if !strings.Contains(diags[0].Message, "Event:subscription.cancelled") {
		t.Errorf("expected canonical handle form in message: %q", diags[0].Message)
	}
}

func TestBrokenLink_AnchorsToConsumeEntry(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "x.go.trellis"), `Feature: X
  "consume on a specific line"

  Consumes:
    - MyTeam.thing
`)
	ws := loadDir(t, dir)
	diags := lint.NewBrokenLink().Check(ws.Files, ws.Graph)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d", len(diags))
	}
	// The Consumes: keyword is line 4 (1-indexed); the handle entry is line 5.
	if diags[0].Range.Start.Line != 5 {
		t.Errorf("diagnostic should anchor to the handle_entry on line 5; got %d", diags[0].Range.Start.Line)
	}
}

// ---------------------------------------------------------------------
// DuplicateProvides
// ---------------------------------------------------------------------

func TestDuplicateProvides_FiresOnEachProviderSite(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "a.go.trellis"), `Feature: A
  "provides Foo"

  Provides:
    - Foo
`)
	writeSidecar(t, filepath.Join(dir, "b.go.trellis"), `Feature: B
  "also provides Foo"

  Provides:
    - Foo
`)
	ws := loadDir(t, dir)
	diags := (&lint.DuplicateProvides{}).Check(ws.Files, ws.Graph)
	if len(diags) != 2 {
		t.Fatalf("want 2 diags (one per provider site), got %d", len(diags))
	}
	for _, d := range diags {
		if d.Severity != lint.SeverityError {
			t.Errorf("duplicate-provides should be error severity; got %v", d.Severity)
		}
	}
}

func TestDuplicateProvides_SilentWhenAllUnique(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "a.go.trellis"), `Feature: A
  "x"

  Provides:
    - Foo
`)
	writeSidecar(t, filepath.Join(dir, "b.go.trellis"), `Feature: B
  "x"

  Provides:
    - Bar
`)
	ws := loadDir(t, dir)
	diags := (&lint.DuplicateProvides{}).Check(ws.Files, ws.Graph)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags, got %v", diags)
	}
}

// ---------------------------------------------------------------------
// OrphanSourceFile
// ---------------------------------------------------------------------

func TestOrphanSourceFile_FiresWhenSourceMissing(t *testing.T) {
	dir := t.TempDir()
	// Sidecar named foo.go.trellis but no foo.go in the same dir.
	writeSidecar(t, filepath.Join(dir, "foo.go.trellis"), `Feature: F
  "no source on disk"
`)
	ws := loadDir(t, dir)
	diags := (&lint.OrphanSourceFile{}).Check(ws.Files, ws.Graph)
	if len(diags) != 1 {
		t.Fatalf("want 1 diag, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "foo.go") {
		t.Errorf("message should name the missing source: %q", diags[0].Message)
	}
}

func TestOrphanSourceFile_SilentWhenSourceExists(t *testing.T) {
	dir := t.TempDir()
	writeSidecar(t, filepath.Join(dir, "foo.go.trellis"), `Feature: F
  "source exists"
`)
	// Create the paired source file.
	if err := os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := loadDir(t, dir)
	diags := (&lint.OrphanSourceFile{}).Check(ws.Files, ws.Graph)
	if len(diags) != 0 {
		t.Errorf("expected 0 diags when source exists, got %v", diags)
	}
}
