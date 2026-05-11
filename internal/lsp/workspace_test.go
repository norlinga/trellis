package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/norlinga/trellis/internal/graph"
)

// fixtureDir resolves the path to testdata/valid relative to this package.
// Tests rely on the three fixture sidecars there; if their content changes
// such that the cross-file resolution shape is different, update both the
// fixture and the asserted positions/handles together.
func fixtureDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../testdata/valid")
	if err != nil {
		t.Fatalf("resolve fixtureDir: %v", err)
	}
	return abs
}

func loadFixtureWorkspace(t *testing.T) *workspace {
	t.Helper()
	w := newWorkspace()
	if err := w.load(fixtureDir(t)); err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	return w
}

// ---------------------------------------------------------------------
// resolveHandleAt
// ---------------------------------------------------------------------

const fixtureSrc = "" +
	"@owner: BillingTeam\n" +
	"\n" +
	"Feature: Sample\n" +
	"  \"A short summary.\"\n" +
	"\n" +
	"  Provides:\n" +
	"    - Sample.create(arg) -> Result\n" +
	"\n" +
	"  Consumes:\n" +
	"    - Other.helper(x) -> Y\n"

// Position helpers — fixtureSrc is hand-aligned, so the expected coordinates
// are stable across edits to non-handle lines.
func TestResolveHandleAt_OnProvidesHandle(t *testing.T) {
	// Cursor on the `c` in `Sample.create` (line 6, char 8 in 0-indexed terms:
	// the bullet line is `    - Sample.create(...)` — 6 spaces of indent + `- `).
	pos := protocol.Position{Line: 6, Character: 11}
	got := resolveHandleAt("/tmp/sample.trellis", []byte(fixtureSrc), pos)
	if got == nil {
		t.Fatal("want a HandleSite, got nil")
	}
	if got.Kind != SiteProvides {
		t.Errorf("Kind = %v, want SiteProvides", got.Kind)
	}
	if got.Handle.Path != "Sample.create" {
		t.Errorf("Handle.Path = %q, want %q", got.Handle.Path, "Sample.create")
	}
}

func TestResolveHandleAt_OnConsumesHandle(t *testing.T) {
	// Cursor on `Other.helper` in the Consumes block (line 9 in 0-indexed).
	pos := protocol.Position{Line: 9, Character: 11}
	got := resolveHandleAt("/tmp/sample.trellis", []byte(fixtureSrc), pos)
	if got == nil {
		t.Fatal("want a HandleSite, got nil")
	}
	if got.Kind != SiteConsumes {
		t.Errorf("Kind = %v, want SiteConsumes", got.Kind)
	}
	if got.Handle.Path != "Other.helper" {
		t.Errorf("Handle.Path = %q, want %q", got.Handle.Path, "Other.helper")
	}
}

func TestResolveHandleAt_NotOnHandle(t *testing.T) {
	// Cursor in the `Feature:` keyword — not inside any handle node.
	pos := protocol.Position{Line: 2, Character: 2}
	got := resolveHandleAt("/tmp/sample.trellis", []byte(fixtureSrc), pos)
	if got != nil {
		t.Errorf("expected nil for non-handle position, got %+v", got)
	}
}

// ---------------------------------------------------------------------
// workspace.providersOf — cross-file resolution against real fixtures
// ---------------------------------------------------------------------

func TestWorkspace_ProvidersOf_PathHandle(t *testing.T) {
	w := loadFixtureWorkspace(t)
	h := graph.Handle{Kind: graph.PathHandle, Path: "PaymentGateway.charge"}
	got := w.providersOf(h)
	if len(got) != 1 {
		t.Fatalf("want 1 provider for %s, got %d", h, len(got))
	}
	if !strings.HasSuffix(got[0].Path, "payment_gateway.rb.trellis") {
		t.Errorf("provider path = %q, want suffix payment_gateway.rb.trellis", got[0].Path)
	}
	if got[0].Kind != SiteProvides {
		t.Errorf("provider Kind = %v, want SiteProvides", got[0].Kind)
	}
}

func TestWorkspace_ProvidersOf_PrefixedHandle(t *testing.T) {
	w := loadFixtureWorkspace(t)
	h := graph.Handle{Kind: graph.PrefixedHandle, Prefix: "Event", Path: "subscription.created"}
	got := w.providersOf(h)
	if len(got) != 1 {
		t.Fatalf("want 1 provider for %s, got %d", h, len(got))
	}
	if !strings.HasSuffix(got[0].Path, "create_subscription.rb.trellis") {
		t.Errorf("provider path = %q, want suffix create_subscription.rb.trellis", got[0].Path)
	}
}

func TestWorkspace_ProvidersOf_Unresolved(t *testing.T) {
	w := loadFixtureWorkspace(t)
	h := graph.Handle{Kind: graph.PathHandle, Path: "Nonexistent.thing"}
	got := w.providersOf(h)
	if len(got) != 0 {
		t.Errorf("want 0 providers, got %d", len(got))
	}
}

// ---------------------------------------------------------------------
// hover rendering
// ---------------------------------------------------------------------

func TestHoverMarkdown_ConsumesResolved(t *testing.T) {
	s := New()
	if err := s.workspace.load(fixtureDir(t)); err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	site := &HandleSite{
		Path:   "/dummy/consumer.rb.trellis",
		Handle: graph.Handle{Kind: graph.PathHandle, Path: "PaymentGateway.charge"},
		Kind:   SiteConsumes,
	}
	md := s.hoverMarkdown(site)
	if !strings.Contains(md, "PaymentGateway.charge") {
		t.Errorf("hover missing handle text:\n%s", md)
	}
	if !strings.Contains(md, "Provided by") {
		t.Errorf("hover missing 'Provided by' attribution:\n%s", md)
	}
	if !strings.Contains(md, "payment_gateway.rb.trellis") {
		t.Errorf("hover missing provider filename:\n%s", md)
	}
}

func TestHoverMarkdown_ConsumesUnresolved(t *testing.T) {
	s := New()
	if err := s.workspace.load(fixtureDir(t)); err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	site := &HandleSite{
		Handle: graph.Handle{Kind: graph.PathHandle, Path: "Mystery.thing"},
		Kind:   SiteConsumes,
	}
	md := s.hoverMarkdown(site)
	if !strings.Contains(md, "Unresolved") {
		t.Errorf("hover should mention 'Unresolved':\n%s", md)
	}
}

func TestHoverMarkdown_ProvidesShowsSelf(t *testing.T) {
	s := New()
	site := &HandleSite{
		Handle: graph.Handle{Kind: graph.PathHandle, Path: "Sample.create"},
		Kind:   SiteProvides,
	}
	md := s.hoverMarkdown(site)
	if !strings.Contains(md, "Sample.create") {
		t.Errorf("hover should echo handle:\n%s", md)
	}
	if !strings.Contains(md, "Provided by this sidecar") {
		t.Errorf("hover should self-document:\n%s", md)
	}
}

// ---------------------------------------------------------------------
// reloadFile — index updates after a save
// ---------------------------------------------------------------------

func TestWorkspace_ReloadFile_NoOpForUnknownPath(t *testing.T) {
	// reloadFile on a path that doesn't exist should not panic; the file
	// simply remains absent from the index.
	w := newWorkspace()
	w.reloadFile("/no/such/file.trellis")
	if got := w.providersOf(graph.Handle{Kind: graph.PathHandle, Path: "x"}); got != nil {
		t.Errorf("want nil providers for missing file, got %v", got)
	}
}

// ---------------------------------------------------------------------
// pathToURI / rootPathFromInitialize
// ---------------------------------------------------------------------

func TestPathToURI_RoundTrip(t *testing.T) {
	in := "/tmp/some/path.trellis"
	uri := pathToURI(in)
	if !strings.HasPrefix(uri, "file:///tmp/") {
		t.Errorf("pathToURI(%q) = %q, want file:///tmp prefix", in, uri)
	}
	if got := uriToPath(uri); got != in {
		t.Errorf("round-trip mismatch: pathToURI→uriToPath(%q) = %q", in, got)
	}
}

func TestRootPathFromInitialize_Precedence(t *testing.T) {
	// WorkspaceFolders wins over RootURI which wins over RootPath.
	rootURI := protocol.DocumentUri("file:///root-uri")
	rootPath := "/root-path"
	cases := []struct {
		name string
		in   *protocol.InitializeParams
		want string
	}{
		{
			name: "WorkspaceFolders wins",
			in: &protocol.InitializeParams{
				WorkspaceFolders: []protocol.WorkspaceFolder{{URI: "file:///wf-root"}},
				RootURI:          &rootURI,
				RootPath:         &rootPath,
			},
			want: "/wf-root",
		},
		{
			name: "RootURI when no folders",
			in: &protocol.InitializeParams{
				RootURI:  &rootURI,
				RootPath: &rootPath,
			},
			want: "/root-uri",
		},
		{
			name: "RootPath fallback",
			in: &protocol.InitializeParams{
				RootPath: &rootPath,
			},
			want: "/root-path",
		},
		{
			name: "Empty when nothing supplied",
			in:   &protocol.InitializeParams{},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rootPathFromInitialize(tc.in); got != tc.want {
				t.Errorf("rootPathFromInitialize: got %q, want %q", got, tc.want)
			}
		})
	}
}
