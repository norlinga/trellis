# Trellis

A specification format and tooling layer for keeping codebases — and the
agents working in them — disciplined.

`.trellis` sidecars sit alongside source files and declare each unit's
contract: what it provides, what it consumes, what is true about it, and
what is explicitly outside its scope. The toolchain in this repository
turns those sidecars into a polyglot dependency graph, a structural and
design-rule linter, an LSP, and a workflow gate for AI coding agents.

For motivation, philosophy, and the format whitepaper, see the project
website (link TBD). This README is the operator's guide to the toolchain
— how to build it, how to use it, and where the pieces live.

---

## Status

| Component | State |
|---|---|
| Format spec (`spec/format.md`, `spec/policy.md`) | v0.1 stable |
| Tree-sitter grammar (`tree-sitter-trellis`, separate repo) | v0.1 — 37/37 corpus tests pass |
| Dependency graph CLI (`trellis graph …`) | v0.1 — six subcommands, dogfood-clean |
| Linter (`trellis lint`) | v0.1 — 10 rules + override mechanism + drift detection |
| Policy packs (`*.trellis-policy`) | v0.1 — `layer_dependencies` + `stability_tiers` |
| Language server (`trellis lsp`) | v0.1 — diagnostics, hover, jump-to-definition |
| Editor integrations (`editors/`) | Neovim + VS Code/Cursor |
| Agent skill (`skill/TRELLIS_SKILL.md`) | v0.1 — drop-in workflow gate |

Pre-v1: the format and on-disk artifacts (sidecar grammar, policy file
shape, diagnostic codes) are stable; rule thresholds and lint-rule
calibration will move with dogfood feedback.

---

## What's in this repository

- **The `trellis` CLI** — `cmd/trellis/`, with subcommands for graph
  queries, linting, and starting the LSP.
- **The format spec** — `spec/format.md` (sidecars), `spec/policy.md`
  (policy packs).
- **The agent skill** — `skill/TRELLIS_SKILL.md`. A single
  agent-agnostic markdown document containing the workflow gates an AI
  coding agent should run before creating or modifying files.
- **Editor integrations** — `editors/neovim/`, `editors/vscode/`. The
  VS Code extension also serves Cursor (Cursor consumes VS Code
  extensions natively).
- **Example policy pack** — `policies/examples/architecture.trellis-policy`.
- **Test fixtures** — `testdata/valid/` (sidecars that should parse and
  lint clean), used by both the test suite and as worked examples.
- **The toolchain dogfoods itself** — the `*.trellis` sidecars
  co-located with source files in `cmd/` and `internal/` are the
  canonical worked examples in a Go codebase.

The tree-sitter grammar lives in its own repository at
[`norlinga/tree-sitter-trellis`](https://github.com/norlinga/tree-sitter-trellis)
because the tree-sitter ecosystem expects grammars as standalone
targets.

---

## Quick start

### Build and install the binary

Requires Go 1.23+.

```sh
git clone https://github.com/norlinga/trellis.git
cd trellis
go build -o ./trellis ./cmd/trellis
sudo install -m 0755 ./trellis /usr/local/bin/trellis

# Sanity check
trellis --help
```

### Try it on the bundled fixtures

```sh
# Lint the sample sidecars (some warnings expected — they're fixtures).
trellis lint testdata/valid

# Build and inspect the dependency graph.
trellis graph build testdata/valid

# Walk one sidecar's outbound dependencies.
trellis graph deps testdata/valid/create_subscription.rb.trellis

# Find sidecars whose Provides have no consumer.
trellis graph orphans testdata/valid
```

### Run the test suite

```sh
go test ./...
```

The repo dogfoods its own format. Linting the toolchain's source-file
sidecars should be clean:

```sh
./trellis lint cmd internal
# → 0 error(s), 0 warning(s)
```

---

## Repository layout

```
trellis/
  cmd/trellis/          # CLI entry point (main.go); subcommands wired here
  internal/
    parser/             # tree-sitter binding wrapper; everything Trellis-aware goes through this
    graph/              # Provides:/Consumes: graph builder + queries
    lint/               # rule definitions, workspace loader, policy pack consumer
    lsp/                # Language Server (diagnostics, hover, definition)
  spec/
    format.md           # the sidecar format reference
    policy.md           # the policy pack format reference
  skill/
    TRELLIS_SKILL.md    # workflow gates for AI coding agents
  policies/
    examples/           # reference policy packs (skipped by lint discovery)
  editors/
    README.md           # install overview
    neovim/             # filetype + nvim-lspconfig client
    vscode/             # VS Code / Cursor extension (LSP client + TextMate grammar)
  testdata/valid/       # sample sidecars (parse + lint clean)
  trellis               # built binary (gitignored)
```

Plus `*.trellis` sidecars co-located with source files throughout
`cmd/` and `internal/` — the toolchain's own use of the format.

---

## CLI surface

| Command | What it does |
|---|---|
| `trellis graph build <paths>` | Discover, parse, summarize the workspace dependency graph |
| `trellis graph deps <file.trellis>` | What this sidecar's `Consumes:` resolves to |
| `trellis graph dependents <file.trellis>` | Sidecars that consume something this one provides |
| `trellis graph downstream <file.trellis>` | Transitive `dependents` — blast radius |
| `trellis graph orphans <paths>` | Sidecars whose `Provides:` have no consumer |
| `trellis graph parse <file.trellis>` | Print the parsed S-expression tree |
| `trellis lint <paths>` | Run all rules; non-zero exit on any error-severity diagnostic |
| `trellis lsp` | Run the Language Server over stdio (consumed by editors) |

All subcommands accept `--help`. Graph commands take **sidecar paths**
(`foo.rb.trellis`), not source paths.

---

## Authoring sidecars

Read `spec/format.md` first. It is the authoring guide — file-pairing
conventions, the Consumes Discipline (the most error-prone part), the
override mechanism, common anti-patterns.

For worked examples, look at:

- `testdata/valid/*.trellis` — three Rails-flavored sidecars that
  exercise prefixed handles (`Event:`), descriptions, and scenarios.
- The `*.trellis` sidecars co-located with source in `cmd/` and
  `internal/` — Go-flavored, dogfooded against the toolchain itself.
- `tree-sitter-trellis/examples/` — the canonical reference example.

The **normative grammar** is `tree-sitter-trellis/grammar.js`. When
prose docs and the grammar disagree, the grammar wins.

---

## Editor integration

See `editors/README.md` for the install path. Both editors get the same
LSP capabilities the server advertises today: diagnostics-as-you-type,
hover (consumer-handle → provider's feature info), and jump-to-definition
(consumer-handle → provider sidecar).

- **Neovim** — drop `editors/neovim/trellis.lua` into your runtime and
  call `require("trellis").setup()`.
- **VS Code / Cursor** — build and install the extension from
  `editors/vscode/`. The extension contributes language registration, a
  TextMate grammar for syntax highlighting, and the LSP client wiring.

The LSP server scans the workspace folder at startup to build the
cross-file index; cross-sidecar resolution (hover and F12) only works
when you open a folder, not a single file.

---

## Agent skill

Drop `skill/TRELLIS_SKILL.md` into the agent's context for any
Trellis-aware project. The skill is one self-contained Markdown document
covering:

- How to recognize a Trellis-aware repository.
- Workflow gates: before creating a new file, before modifying an
  existing one, after implementation.
- The Consumes Discipline summary.
- A CLI cheatsheet and a lint-rule remediation table.
- A decision tree for the common situations.

It is agent-agnostic: drop it wherever your agent system reads skill /
rule files (`.claude/skills/`, `.cursor/rules/`, a generic `.agents/`
directory at the project root, or anywhere your tooling collects them).

---

## Policy packs

`*.trellis-policy` files declare cross-sidecar architectural rules
enforced by the linter. v1 supports two rule kinds:

- **`layer_dependencies:`** over `@layer:` frontmatter — `domain MUST
  NOT consume infrastructure`, etc.
- **`stability_tiers:`** over `@stability:` frontmatter — `stable MUST
  NOT consume experimental`.

`MUST NOT` is enforced at error severity; `MAY` is parsed but currently
informational (substrate for a future default-deny mode).

Format reference: `spec/policy.md`. Worked example:
`policies/examples/architecture.trellis-policy`. Discovery walks lint
roots and skips `examples/` directories so reference packs don't
accidentally enforce.

---

## Companion repository

The tree-sitter grammar — the parser the rest of the toolchain depends
on — lives at
[`norlinga/tree-sitter-trellis`](https://github.com/norlinga/tree-sitter-trellis).
Its README documents the grammar slices, corpus tests, and pinned
versions for the Go bindings.

Both repos are pinned to specific dependency versions and tested
together; upgrading `tree-sitter` itself or `go-tree-sitter` should
go through both repos in lockstep.

---

## Contributing

Two non-negotiables before opening a PR:

1. `go test ./...` passes.
2. `./trellis lint cmd internal` passes (the toolchain's own
   sidecars stay clean).

Beyond that:

- New diagnostic codes are a public API. Once shipped, downstream
  tooling (editors, CI scripts, policy files) keys off them — naming
  changes need explicit migration notes.
- New format constructs require a corresponding tree-sitter grammar
  change in the companion repo plus a corpus test demonstrating it.
- New lint rules should land with a fixture in `testdata/` and at
  least one test case demonstrating both the firing path and a
  non-firing path.

---

## License

MIT. See `LICENSE`.
