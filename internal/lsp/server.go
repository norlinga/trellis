// Package lsp implements a Language Server Protocol server for .trellis
// sidecar files.
//
// v1 scope is intentionally narrow: lifecycle plus diagnostics. Each
// time a document is opened, changed, or saved, the server runs the
// per-file lint rule set and pushes the resulting diagnostics to the
// client. Hover, jump-to-definition, and code actions arrive in
// subsequent slices once the diagnostic loop has been validated.
//
// This package does no JSON-RPC framing or transport — that's glsp's
// job. The only dependency-aware code lives here in `handle*` methods
// that translate between LSP wire types and our internal lint types.
package lsp

import (
	"net/url"
	"strings"
	"sync"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"

	"github.com/norlinga/trellis/internal/lint"
)

// ServerName is the name reported back to the client during initialize.
// Client logs and editor UIs may surface it; keep it stable.
const ServerName = "trellis-lsp"

// Server is the LSP server. State has two parts:
//
//   - open: in-memory source for every document the editor has open. This
//     drives diagnostics-as-you-type and feeds the position-to-handle
//     resolver for hover/definition (so unsaved edits are reflected).
//   - workspace: an on-disk index of every .trellis file under the root,
//     used for cross-file resolution. Built at initialize, refreshed on
//     save. We deliberately do not refresh per-keystroke: handle
//     visibility lags by one save, which is a reasonable cost for not
//     re-walking the whole tree on every edit.
//
// Each edit re-parses the active document from scratch — .trellis files
// are small enough that incremental parsing is premature.
type Server struct {
	mu        sync.Mutex
	open      map[protocol.DocumentUri][]byte
	workspace *workspace
}

// New returns a fresh Server with no open documents and an empty workspace.
// The workspace is populated by Initialize when the client supplies a
// rootURI / workspace folder.
func New() *Server {
	return &Server{
		open:      map[protocol.DocumentUri][]byte{},
		workspace: newWorkspace(),
	}
}

// Run starts the LSP server over stdio. Blocks until the client sends
// `exit`. Returns any transport-level error encountered.
func (s *Server) Run() error {
	handler := &protocol.Handler{
		Initialize:            s.initialize,
		Initialized:           s.initialized,
		Shutdown:              s.shutdown,
		Exit:                  s.exit,
		TextDocumentDidOpen:   s.didOpen,
		TextDocumentDidChange: s.didChange,
		TextDocumentDidSave:   s.didSave,
		TextDocumentDidClose:  s.didClose,
		TextDocumentHover:     s.hover,
		TextDocumentDefinition: s.definition,
		SetTrace:              s.setTrace,
	}
	srv := server.NewServer(handler, ServerName, false)
	return srv.RunStdio()
}

// ---------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------

func (s *Server) initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	syncKind := protocol.TextDocumentSyncKindFull
	caps := protocol.ServerCapabilities{
		TextDocumentSync: protocol.TextDocumentSyncOptions{
			OpenClose: ptr(true),
			Change:    &syncKind,
			Save:      true,
		},
		HoverProvider:      true,
		DefinitionProvider: true,
	}
	// Index the workspace synchronously. Trellis workspaces are small
	// (.trellis files are sidecars, one per source file) so this is fast
	// in practice; if profiling later shows it dominates startup, kick
	// it onto a goroutine and have hover/definition wait on a `ready`
	// channel.
	if root := rootPathFromInitialize(params); root != "" {
		_ = s.workspace.load(root)
	}
	version := "0.1.0"
	return protocol.InitializeResult{
		Capabilities: caps,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    ServerName,
			Version: &version,
		},
	}, nil
}

func (s *Server) initialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (s *Server) shutdown(ctx *glsp.Context) error {
	s.mu.Lock()
	s.open = map[protocol.DocumentUri][]byte{}
	s.mu.Unlock()
	return nil
}

func (s *Server) exit(ctx *glsp.Context) error { return nil }

func (s *Server) setTrace(ctx *glsp.Context, params *protocol.SetTraceParams) error {
	return nil
}

// ---------------------------------------------------------------------
// Document synchronization → diagnostic publishing
// ---------------------------------------------------------------------

func (s *Server) didOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.track(params.TextDocument.URI, []byte(params.TextDocument.Text))
	s.publish(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) didChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	// Full-sync mode: each ContentChanges entry is a
	// TextDocumentContentChangeEventWhole. Take the last one — under
	// Full sync, intermediate entries cannot exist, but defensive.
	for _, change := range params.ContentChanges {
		if whole, ok := change.(protocol.TextDocumentContentChangeEventWhole); ok {
			s.track(params.TextDocument.URI, []byte(whole.Text))
		}
	}
	s.publish(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) didSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	if params.Text != nil {
		s.track(params.TextDocument.URI, []byte(*params.Text))
	}
	// A save is the explicit signal we use to refresh cross-file state.
	// Per-keystroke updates would re-walk the whole file's AST for every
	// edit; once-per-save keeps hover/definition fresh enough without that
	// cost.
	s.workspace.reloadFile(uriToPath(string(params.TextDocument.URI)))
	s.publish(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) didClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.mu.Lock()
	delete(s.open, params.TextDocument.URI)
	s.mu.Unlock()
	// Clear stale diagnostics — editors keep the last-published list
	// pinned to the file until told otherwise.
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []protocol.Diagnostic{},
	})
	return nil
}

func (s *Server) track(uri protocol.DocumentUri, src []byte) {
	s.mu.Lock()
	s.open[uri] = src
	s.mu.Unlock()
}

// publish runs the per-file linter against the current source for uri
// and pushes diagnostics. Called on every open/change/save.
func (s *Server) publish(ctx *glsp.Context, uri protocol.DocumentUri) {
	s.mu.Lock()
	src, ok := s.open[uri]
	s.mu.Unlock()
	if !ok {
		return
	}
	path := uriToPath(string(uri))
	lintDiags := lint.LintSingleFile(path, src)
	lspDiags := ConvertDiagnostics(lintDiags)
	ctx.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: lspDiags,
	})
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// ConvertDiagnostics maps lint Diagnostics (1-indexed positions) to LSP
// Diagnostics (0-indexed positions). Severity, code, and message survive
// unchanged.
func ConvertDiagnostics(diags []lint.Diagnostic) []protocol.Diagnostic {
	out := make([]protocol.Diagnostic, len(diags))
	for i, d := range diags {
		sev := convertSeverity(d.Severity)
		code := protocol.IntegerOrString{Value: d.Code}
		out[i] = protocol.Diagnostic{
			Range:    convertRange(d.Range),
			Severity: &sev,
			Code:     &code,
			Source:   ptr(ServerName),
			Message:  d.Message,
		}
	}
	return out
}

func convertRange(r lint.Range) protocol.Range {
	return protocol.Range{
		Start: convertPosition(r.Start),
		End:   convertPosition(r.End),
	}
}

func convertPosition(p lint.Position) protocol.Position {
	// LSP positions are 0-indexed for both line and character; lint
	// positions are 1-indexed. Subtract guarding the floor so a
	// 1:1-anchored diagnostic doesn't underflow into MaxUint.
	line := uint32(0)
	if p.Line > 0 {
		line = uint32(p.Line - 1)
	}
	col := uint32(0)
	if p.Column > 0 {
		col = uint32(p.Column - 1)
	}
	return protocol.Position{Line: line, Character: col}
}

func convertSeverity(s lint.Severity) protocol.DiagnosticSeverity {
	switch s {
	case lint.SeverityError:
		return protocol.DiagnosticSeverityError
	case lint.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	}
	return protocol.DiagnosticSeverityInformation
}

// uriToPath converts an LSP file URI to a filesystem path. LSP URIs use
// `file:///abs/path`; non-file URIs (untitled, scheme-less) round-trip
// to a string the linter can still log, even if the linter rules can't
// touch the filesystem with it.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		if u, err := url.Parse(uri); err == nil {
			return u.Path
		}
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func ptr[T any](v T) *T { return &v }
