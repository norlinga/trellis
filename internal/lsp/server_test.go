package lsp_test

import (
	"testing"

	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/norlinga/trellis/internal/lint"
	"github.com/norlinga/trellis/internal/lsp"
)

func TestConvertDiagnostics_PositionsAndSeverity(t *testing.T) {
	in := []lint.Diagnostic{
		{
			Code:     "missing-invariants",
			Severity: lint.SeverityWarning,
			Message:  "blah",
			Path:     "/x/foo.trellis",
			Range: lint.Range{
				Start: lint.Position{Line: 6, Column: 1},
				End:   lint.Position{Line: 6, Column: 8},
			},
		},
		{
			Code:     "duplicate-provides",
			Severity: lint.SeverityError,
			Message:  "boom",
			Path:     "/x/bar.trellis",
			Range: lint.Range{
				Start: lint.Position{Line: 1, Column: 1},
				End:   lint.Position{Line: 1, Column: 1},
			},
		},
	}
	got := lsp.ConvertDiagnostics(in)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	// LSP positions are 0-indexed; lint positions are 1-indexed.
	if got[0].Range.Start.Line != 5 || got[0].Range.Start.Character != 0 {
		t.Errorf("0: bad position conversion: %+v", got[0].Range.Start)
	}
	if got[0].Range.End.Line != 5 || got[0].Range.End.Character != 7 {
		t.Errorf("0: bad end conversion: %+v", got[0].Range.End)
	}
	if got[0].Severity == nil || *got[0].Severity != protocol.DiagnosticSeverityWarning {
		t.Errorf("0: severity = %v, want Warning", got[0].Severity)
	}
	if got[1].Severity == nil || *got[1].Severity != protocol.DiagnosticSeverityError {
		t.Errorf("1: severity = %v, want Error", got[1].Severity)
	}

	// Code is wired through as IntegerOrString.
	if got[0].Code == nil || got[0].Code.Value != "missing-invariants" {
		t.Errorf("0: code wiring failed: %+v", got[0].Code)
	}
	// Source is the server name so editor UIs can group diagnostics.
	if got[0].Source == nil || *got[0].Source != lsp.ServerName {
		t.Errorf("0: Source = %v, want %q", got[0].Source, lsp.ServerName)
	}
}

func TestConvertDiagnostics_FileStartGuard(t *testing.T) {
	// A diagnostic anchored to the file start (1:1) should convert
	// safely to (0:0), not underflow to MaxUint.
	in := []lint.Diagnostic{
		{
			Code:     "frontmatter-missing-required",
			Severity: lint.SeverityWarning,
			Range:    lint.Range{Start: lint.Position{Line: 1, Column: 1}, End: lint.Position{Line: 1, Column: 1}},
		},
	}
	got := lsp.ConvertDiagnostics(in)
	if got[0].Range.Start.Line != 0 || got[0].Range.Start.Character != 0 {
		t.Errorf("file-start anchor: want (0,0), got %+v", got[0].Range.Start)
	}

	// And a 0-anchored input (would never happen from real lint output,
	// but defensive) must not underflow.
	zero := []lint.Diagnostic{
		{Range: lint.Range{Start: lint.Position{Line: 0, Column: 0}}},
	}
	gotZero := lsp.ConvertDiagnostics(zero)
	if gotZero[0].Range.Start.Line != 0 {
		t.Errorf("zero anchor: should clamp to 0, got %d", gotZero[0].Range.Start.Line)
	}
}
