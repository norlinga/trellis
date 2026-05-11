// Package parser wraps the tree-sitter-trellis Go binding.
//
// All Trellis-aware code in this repo (graph builder, linter, LSP) goes
// through this package rather than importing tree-sitter directly, so
// upgrades to the binding library or grammar repository touch one file.
package parser

import (
	"fmt"
	"os"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_trellis "github.com/norlinga/tree-sitter-trellis/bindings/go"
)

// Language returns a freshly wrapped *sitter.Language for the Trellis
// grammar. Cheap; safe to call repeatedly.
func Language() *sitter.Language {
	return sitter.NewLanguage(tree_sitter_trellis.Language())
}

// Parse parses src as Trellis source. The returned Tree owns C memory; the
// caller must Close it.
func Parse(src []byte) (*sitter.Tree, error) {
	p := sitter.NewParser()
	defer p.Close()
	if err := p.SetLanguage(Language()); err != nil {
		return nil, fmt.Errorf("set language: %w", err)
	}
	tree := p.Parse(src, nil)
	if tree == nil {
		return nil, fmt.Errorf("parser returned nil tree")
	}
	return tree, nil
}

// ParseFile reads path and parses its contents. Returns the tree and the
// raw source bytes — callers usually need source to extract node text via
// byte ranges, since tree-sitter does not retain the input itself.
func ParseFile(path string) (*sitter.Tree, []byte, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	tree, err := Parse(src)
	if err != nil {
		return nil, nil, err
	}
	return tree, src, nil
}
