package lint

import (
	"fmt"
	"time"
)

// StaleReviewed enforces whitepaper §4.2's drift-detection rule:
// `@reviewed` ages out of date and CI surfaces it. The aging artifact is
// the forcing function that pulls authors back to re-read their sidecars.
//
// No override key. Stale-reviewed has no `@allow-stale-reviewed:` because
// the right response is to re-read the file and update the date — that
// IS the rule's purpose. Adding a suppression would defeat the design.
//
// If a unit is genuinely archived and no longer reviewed, mark it via
// `@stability: deprecated`. (A future enhancement could let StaleReviewed
// short-circuit on deprecated stability; for v1 it does not, on the
// principle that even deprecated units benefit from periodic review.)
type StaleReviewed struct {
	// Now is the reference moment — typically time.Now() but injectable
	// for deterministic tests.
	Now time.Time
	// WarnAfter and ErrorAfter are durations from the @reviewed date past
	// which the rule fires. v1 defaults: 180 days warn, 365 days error.
	WarnAfter  time.Duration
	ErrorAfter time.Duration
}

// NewStaleReviewed returns a StaleReviewed configured with the v1 default
// thresholds (6 months warn, 12 months error) and Now set to time.Now()
// at construction. Default() calls this; tests can construct directly
// with custom values.
func NewStaleReviewed() *StaleReviewed {
	return &StaleReviewed{
		Now:        time.Now(),
		WarnAfter:  180 * 24 * time.Hour,
		ErrorAfter: 365 * 24 * time.Hour,
	}
}

func (*StaleReviewed) Code() string { return "stale-reviewed" }

func (r *StaleReviewed) Check(f *File) []Diagnostic {
	root := f.Tree.RootNode()
	fm := root.ChildByFieldName("frontmatter")
	if fm == nil {
		return nil
	}
	for i := uint(0); i < fm.NamedChildCount(); i++ {
		entry := fm.NamedChild(i)
		if entry.Kind() != "frontmatter_entry" {
			continue
		}
		keyNode := entry.ChildByFieldName("key")
		if keyNode == nil || nodeText(keyNode, f.Source) != "@reviewed" {
			continue
		}
		valNode := entry.ChildByFieldName("value")
		if valNode == nil || valNode.Kind() != "iso_date" {
			continue // grammar accepts identifier for malformed dates; ignore
		}
		reviewedAt, err := time.Parse("2006-01-02", nodeText(valNode, f.Source))
		if err != nil {
			continue
		}
		age := r.Now.Sub(reviewedAt)
		if age < r.WarnAfter {
			return nil
		}
		sev := SeverityWarning
		if age >= r.ErrorAfter {
			sev = SeverityError
		}
		days := int(age.Hours() / 24)
		return []Diagnostic{{
			Code:     "stale-reviewed",
			Severity: sev,
			Message:  fmt.Sprintf("@reviewed is %d days old (threshold: warn at %d, error at %d) — re-read the sidecar and bump the date", days, int(r.WarnAfter.Hours()/24), int(r.ErrorAfter.Hours()/24)),
			Path:     f.Path,
			Range:    RangeFromNode(valNode),
		}}
	}
	return nil
}
