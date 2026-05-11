package lsp

import (
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// definition handles textDocument/definition.
//
// A "definition" of a Consumes handle is the matching Provides entry in
// another sidecar — the contract the consumer is depending on. Cursor on a
// Provides handle returns nothing: by convention LSP "definition" is
// asymmetric (use "references" / "implementations" for the other direction;
// neither is implemented yet).
//
// Multiple results are returned when more than one sidecar provides the
// handle. The duplicate-provides linter rule already errors on this; the
// LSP doesn't try to disambiguate.
//
// The jump target is the entire `handle_entry` (the bullet line) so the
// editor lands the cursor on the readable form, not just the bare handle
// token.
func (s *Server) definition(_ *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	s.mu.Lock()
	src, ok := s.open[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}
	site := resolveHandleAt(uriToPath(string(params.TextDocument.URI)), src, params.Position)
	if site == nil || site.Kind != SiteConsumes {
		return nil, nil
	}
	providers := s.workspace.providersOf(site.Handle)
	if len(providers) == 0 {
		return nil, nil
	}
	out := make([]protocol.Location, 0, len(providers))
	for _, p := range providers {
		out = append(out, protocol.Location{
			URI:   protocol.DocumentUri(pathToURI(p.Path)),
			Range: p.EntryRange,
		})
	}
	return out, nil
}
