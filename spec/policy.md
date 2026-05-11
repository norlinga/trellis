# The Trellis Policy Format

`*.trellis-policy` files declare architectural rules that the linter enforces
across the workspace. They are companion artifacts to `.trellis` sidecars:
sidecars say what each unit *is*, policy files say which units *may
depend on which other units*.

This document is the format reference. The whitepaper §6.2/§6.3 covers the
philosophy.

---

## At a Glance

```
# policies/architecture.trellis-policy

layer_dependencies:
  - domain MUST NOT consume infrastructure
  - application MAY consume domain

stability_tiers:
  - stable MUST NOT consume experimental
```

A policy file is line-oriented and shallow:

- **Comments** start with `#` and run to end-of-line. Whole-line and
  trailing comments after a rule are both supported anywhere except
  inside a predicate.
- **Section headers** are at column 0, lowercase identifier followed by
  `:` — `layer_dependencies:`, `stability_tiers:`.
- **Rule entries** are indented bullet lines: `  - <predicate>`.
- **Predicates** have one shape:
  `<source> (MUST NOT | MAY) consume <target>`.

Source and target are bareword identifiers (alphanumeric + `_` + `-`).
They name a value of the relevant frontmatter key — layer names for
`layer_dependencies:`, stability tier names for `stability_tiers:`.

---

## Recognized sections (v1)

| Section | Reads frontmatter | What it constrains |
|---|---|---|
| `layer_dependencies` | `@layer:` | Edges between architectural layers |
| `stability_tiers`    | `@stability:` | Edges between stability tiers (e.g. `stable` → `experimental` is unsafe) |

Unknown section names are a fatal parse error for that file. Adding a typo
like `layer_depndencies:` would otherwise silently drop every rule under
it; the parser is strict so authors hear about it immediately.

Future sections on the roadmap: `context_isolation:` (`@context:`),
ownership-aware constraints (`@owner:`), pack imports.

---

## Verbs

### `MUST NOT consume`

Enforced. A `Consumes:` edge from a sidecar tagged with the source value
to a sidecar tagged with the target value emits a
`policy-layer-violation` (or `policy-stability-violation`) diagnostic at
**error severity**. The linter exits non-zero.

### `MAY consume`

Parsed and validated, **but not currently enforced**. It is documentation
today. A future "default-deny" mode will treat the absence of a `MAY`
rule as an implicit `MUST NOT`, at which point `MAY` rules become
load-bearing. Authoring `MAY` rules now costs nothing and pays off when
that mode lands.

---

## What gets evaluated

For each `Consumes:` edge in the resolved dependency graph:

1. Read the consuming sidecar's frontmatter for the relevant key
   (`@layer:` or `@stability:`).
2. Read the producing sidecar's frontmatter for the same key.
3. **If either is missing, the edge is skipped.** Sidecars without a
   layer (or without a stability tag) are not part of any layer (or
   tier) constraint. This is what lets utility files, entry points, and
   in-progress units coexist with policy enforcement.
4. Otherwise, check every `MUST NOT` rule whose source/target match.
   Each match emits one diagnostic anchored at the consuming `Consumes:`
   bullet line.

Unresolved consumes (the consumer references a handle no sidecar
provides) never reach this rule — `broken-link` handles those.

---

## File discovery

`trellis lint` discovers `*.trellis-policy` files using the same walk as
`.trellis` files:

- Each lint root is searched recursively.
- Hidden directories (leading `.`) are skipped.
- **Directories named `examples` are skipped.** Example packs in
  `policies/examples/` are reference material, not active rules. Authors
  can opt back in by passing the example file path explicitly:
  `trellis lint . policies/examples/architecture.trellis-policy`.
- Files are deduplicated by absolute path and merged (rules are
  concatenated, not deduplicated).

The recommended layout for a real (not example) pack:

```
<repo>/
  policies/
    architecture.trellis-policy   ← active, picked up automatically
    examples/
      starter-pack.trellis-policy ← skipped by discovery
```

---

## No overrides

There is no `@allow-policy-violation:` frontmatter key, by design. A
violation has exactly two correct responses:

1. **Fix the architecture** — refactor so the offending edge no longer
   exists.
2. **Change the policy** — if the violation is intentional and the rule
   is wrong, edit `architecture.trellis-policy` in the same PR.

Both options leave a versioned, reviewable artifact behind. A
per-sidecar suppression would erode the architectural decision over
time without a paper trail. The friction principle that justifies
`@allow-many-consumes` (the override IS the documentation) doesn't
apply here, because the policy file *is already* the documentation.

---

## Diagnostic codes

| Code | Section | Severity |
|---|---|---|
| `policy-layer-violation` | `layer_dependencies` | error |
| `policy-stability-violation` | `stability_tiers` | error |
| `policy-parse-failed` | (any) | error — surfaced by the CLI when a `.trellis-policy` file fails to parse |

Diagnostic messages cite the policy file and line number so authors can
jump from the violation to the rule that fired it.

---

## Frontmatter conventions

To opt a sidecar into policy enforcement, declare the relevant keys:

```trellis
@owner: BillingTeam
@stability: stable
@layer: domain
@since: 2025-11-04
@reviewed: 2026-04-01
```

**Layer names are author-defined.** The format does not prescribe a
fixed layer vocabulary — `domain`/`application`/`infrastructure` is the
canonical hexagonal-architecture set, but `core`/`shell`,
`pure`/`io`, or any other split works as long as the policy file
matches.

**Stability values are also author-defined**, though a small canonical
set is conventional: `stable`, `experimental`, `deprecated`, `internal`.
The linter does not validate the value space — if you write
`@stability: half-baked`, the linter will faithfully match `half-baked`
in policy rules.

---

## Where to look next

- **Format reference** — `spec/format.md`
- **Whitepaper philosophy** — the whitepaper at the project website, §6
- **Example pack** — `policies/examples/architecture.trellis-policy`
