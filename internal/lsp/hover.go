package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// hover handles textDocument/hover.
//
// What you get when the cursor sits on…
//   - …a Provides handle: a self-documenting "defined here" line, no
//     cross-file lookup. The user is reading the source of truth already.
//   - …a Consumes handle with one provider: that provider's FeatureName,
//     summary, and source path. This is the slice's marquee feature — you
//     can read the contract of every dependency without leaving the file.
//   - …a Consumes handle with multiple providers: every provider listed
//     (the linter's duplicate-provides rule already flags this as an error;
//     hover doesn't try to disambiguate further).
//   - …a Consumes handle with no provider: an "Unresolved" message. We
//     don't try to distinguish "external dependency" from "broken link"
//     here — the linter's BrokenLink rule + external-prefix allowlist owns
//     that judgement.
//
// Returns nil (no hover) when the cursor isn't inside a handle node.
func (s *Server) hover(_ *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	s.mu.Lock()
	src, ok := s.open[params.TextDocument.URI]
	s.mu.Unlock()
	if !ok {
		return nil, nil
	}
	site := resolveHandleAt(uriToPath(string(params.TextDocument.URI)), src, params.Position)
	if site == nil {
		return nil, nil
	}
	md := s.hoverMarkdown(site)
	if md == "" {
		return nil, nil
	}
	rng := site.HandleRange
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.MarkupKindMarkdown,
			Value: md,
		},
		Range: &rng,
	}, nil
}

// hoverMarkdown renders the hover doc as Markdown. Pulled out from the LSP
// handler so tests can assert directly on the rendered text without spinning
// up a full Server.
func (s *Server) hoverMarkdown(site *HandleSite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**`%s`**\n\n", site.Handle)

	if site.Kind == SiteProvides {
		b.WriteString("_Provided by this sidecar._")
		return b.String()
	}

	providers := s.workspace.providersOf(site.Handle)
	if len(providers) == 0 {
		b.WriteString("_Unresolved_ — no sidecar in this workspace provides this handle.")
		return b.String()
	}

	for i, p := range providers {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		s.renderProviderHover(&b, &p)
	}
	return b.String()
}

func (s *Server) renderProviderHover(b *strings.Builder, site *HandleSite) {
	sc := s.workspace.sidecar(site.Path)
	if sc != nil {
		if sc.FeatureName != "" {
			fmt.Fprintf(b, "**Feature**: %s\n\n", sc.FeatureName)
		}
		if sc.FeatureSummary != "" {
			fmt.Fprintf(b, "%s\n\n", sc.FeatureSummary)
		}
	}
	fmt.Fprintf(b, "_Provided by_ `%s`", filepath.Base(site.Path))
}
