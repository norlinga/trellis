package lint

import (
	"fmt"
	"io"
	"sort"
)

// SortDiagnostics orders by (Path, Start.Line, Start.Column, Code) for
// stable display. Mutates the slice in place.
func SortDiagnostics(diags []Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i], diags[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Range.Start.Line != b.Range.Start.Line {
			return a.Range.Start.Line < b.Range.Start.Line
		}
		if a.Range.Start.Column != b.Range.Start.Column {
			return a.Range.Start.Column < b.Range.Start.Column
		}
		return a.Code < b.Code
	})
}

// Format writes diagnostics in the conventional
// `path:line:col: severity [code] message` form, one per line.
func Format(w io.Writer, diags []Diagnostic) {
	for _, d := range diags {
		fmt.Fprintf(w, "%s:%d:%d: %s [%s] %s\n",
			d.Path,
			d.Range.Start.Line,
			d.Range.Start.Column,
			d.Severity,
			d.Code,
			d.Message,
		)
	}
}

// Summarize counts diagnostics by severity. Useful for CLI footer output
// ("3 warnings, 0 errors") and for exit-code decisions.
type Summary struct {
	Errors   int
	Warnings int
	Infos    int
}

func Summarize(diags []Diagnostic) Summary {
	var s Summary
	for _, d := range diags {
		switch d.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		case SeverityInfo:
			s.Infos++
		}
	}
	return s
}
