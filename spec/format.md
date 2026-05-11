# The Trellis Format

This document is the authoring guide for `.trellis` sidecar files. It captures the conventions a human or agent must follow to produce sidecars that compose well across files, languages, and authors.

It is **not** the grammar (that lives in [`tree-sitter-trellis/grammar.js`](https://github.com/norlinga/tree-sitter-trellis/blob/main/grammar.js), the normative source of truth). It is **not** the philosophy (that lives in the whitepaper, `trellis-plan/trellis-whitepaper.md`). It is **not** the locked design rationale (that lives in `trellis-plan/TREE_SITTER_DECISIONS.md`).

This document covers what those three do not: **the authorial discipline you need so two people writing sidecars for similar code produce comparable results.**

When this document and the decisions doc disagree, the decisions doc wins.

---

## At a Glance

A `.trellis` sidecar is paired with a source file by name:

```
billing/create_subscription.rb
billing/create_subscription.rb.trellis    ← sidecar for the file above
```

The sidecar describes the file's *intent* — what it provides, what it depends on, what is true about it, what is explicitly outside its scope, and how it behaves under representative scenarios. It is not test code. It is not implementation. It is the contract a reader (human or agent) consults *before* changing the source file.

A minimal sidecar:

```trellis
@owner: BillingTeam
@stability: stable
@since: 2025-11-04
@reviewed: 2026-03-15

Feature: Create Subscription
  "Handles the idempotent creation of a user subscription and initial billing."

  Provides:
    - Subscription.create(user, plan_id) -> Subscription | raises PaymentError

  Consumes:
    - PaymentGateway.charge(token, amount) -> ChargeResult

  Invariants:
    - A user MUST NOT have two active 'pro' subscriptions

  Scenario (happy-path): Successful checkout
    Given a User with a valid stripe_token
    When create is called with plan_id
    Then a Subscription record is created
```

The whitepaper (§3) covers the structure in full, with every block kind and frontmatter key. This document picks up where the whitepaper leaves off: the conventions that the format alone doesn't enforce.

---

## File Pairing

### One sidecar per source file

Pair each source file with exactly one `.trellis` file by appending `.trellis` to the source file's full name (preserving any extension):

| Source | Sidecar |
|---|---|
| `create_subscription.rb` | `create_subscription.rb.trellis` |
| `widget.tsx` | `widget.tsx.trellis` |
| `payment_gateway.go` | `payment_gateway.go.trellis` |
| `migrate_users.sql` | `migrate_users.sql.trellis` |

The sidecar lives in the **same directory** as its source.

### Test files do not get their own sidecar

A test file (`parser_test.go`, `subscription_spec.rb`, `widget.test.tsx`, etc.) is the **realization** of the scenarios already declared in the source file's sidecar. Sidecaring tests separately fragments intent across two artifacts and lets them drift independently — exactly the failure mode Trellis exists to prevent.

**Rule:** scenarios live in the source file's sidecar; the implementation of those scenarios lives in the test file. One sidecar per *unit of intent*, not one sidecar per *file on disk*.

If a test file genuinely covers a different unit of intent than its paired source — e.g., an integration test that exercises three modules at once — it gets its own sidecar with its own `Provides:` describing that integration boundary.

### `main` packages and program entry points

A file like `cmd/trellis/main.go` provides only `main.main`, which the runtime (not another sidecar) consumes. The graph will report it as orphaned. This is structural, not an authoring error. The linter is expected to suppress orphan diagnostics for files that match the entry-point convention; until then, mark them with a frontmatter override (exact key TBD when the lint rule lands).

---

## The Consumes Discipline

This is the most error-prone part of authoring a sidecar. The whitepaper says `Consumes:` lists "what this unit depends on." That is true but underspecified. In a typical source file there are dozens to hundreds of external symbols touched (stdlib calls, framework APIs, language built-ins, helper utilities). Listing all of them buries real signal under noise; listing too few makes the dependency graph lie.

**The rule: a handle belongs in `Consumes:` if and only if a change to that handle's contract — its name, signature, semantics, or existence — would force a corresponding change in this file.**

This is the same heuristic an experienced engineer applies when writing a module-level docstring or a Go `// dependencies:` comment. The Trellis format is simply asking you to write it down.

### Worked examples

**Example 1 — internal API, contract-bearing.** A Go file that calls `parser.ParseFile(path)`:

```go
tree, src, err := parser.ParseFile(path)
```

Renaming `ParseFile`, changing its signature, or removing it would force this file to change. **List it.**

```trellis
Consumes:
  - parser.ParseFile
```

**Example 2 — stdlib, incidental.** The same file calls `fmt.Errorf("read %s: %w", path, err)`. The `fmt` package is incidental scaffolding — if it were renamed, the language ecosystem (not just this file) would break. The dependency is real, but it is not part of *this file's contract surface*.

**Do not list it.**

**Example 3 — external library, contract-bearing.** A file calls `tree_sitter.NewLanguage(ptr)` to wrap the parser. Upgrading `go-tree-sitter` from v0.25 to v0.26 changed `Parser.SetLanguage` from a `void` to an `error` return — that is precisely the kind of change this file needs to know about. The signature is part of this file's contract.

**List it.**

```trellis
Consumes:
  - sitter.NewLanguage
  - sitter.Parser.SetLanguage
```

**Example 4 — language built-in.** A file uses Go's built-in `len(slice)`. This is not a dependency in any meaningful sense.

**Do not list it.**

**Example 5 — framework type used as a parameter.** A function in this file accepts a `*cobra.Command`. The type `cobra.Command` is part of this file's interface to the rest of the codebase: callers must construct one to pass in.

**List the type.**

```trellis
Consumes:
  - cobra.Command
```

(The grammar accepts type-only handles with empty descriptions; see decision #6.)

### A practical test

Before adding an entry to `Consumes:`, ask: *"If a teammate refactored this handle, would I want them to grep for sidecars that mention it before they did so?"*

If yes → list it.
If no → it doesn't belong here.

This is the same test as the agent-skill workflow gate (whitepaper §4.4): the question presupposes a downstream tool will use the graph to ask "who depends on what I'm about to change." Anything that doesn't deserve to surface in that query also doesn't deserve to be in `Consumes:`.

### Idiomatic vs incidental, by language

Some communities draw the line differently. A few rules of thumb:

- **Go.** List exported symbols (`pkg.Func`, `pkg.Type`, `pkg.Type.Method`) from non-stdlib packages and from internal-but-cross-package dependencies. Do not list stdlib unless the contract specifically depends on a particular stdlib version's behavior.
- **Ruby.** List class names and module methods you depend on; do not list every method call. The Ruby community already thinks in terms of "duck-typed contracts" (see decision #6's `UserRecord (must respond to: id, email, payment_token)` example).
- **TypeScript / JavaScript.** List imports from non-relative paths if they are framework-load-bearing; treat relative imports the same way you treat internal-cross-package Go dependencies.
- **SQL migrations.** List the tables and views the migration reads or writes; do not list every column.

When in doubt, err on the side of *fewer entries with sharper meaning* than *more entries that catalog every line of code*. The graph's value comes from precision, not exhaustiveness.

---

## Authoring Conventions

These are short, opinionated rules for parts of the format that the grammar permits in multiple ways.

### Frontmatter

- **Always include** `@owner` and `@stability`. The graph and linter both treat these as load-bearing.
- **Include `@since`** with the ISO date the unit was introduced.
- **Include `@reviewed`** with the ISO date of the last deliberate review. The linter ages this and warns at 180 days, errors at 365 (`stale-reviewed` rule). There is no override — the aging artifact is the forcing function. To silence the diagnostic, re-read the file and bump the date.
- **Composition** (`@composition: [Trait1, Trait2]`) is for declared traits the unit implements; do not list parent classes or interfaces incidentally.
- See decision #5 for the value-shape rules and decision #5a for the full list of recognized keys.

### Handles

- One handle per `-` entry. The handle is the leftmost dotted-identifier path or `Prefix:path`.
- Handle case is preserved exactly. `Subscription.create` and `Subscription.Create` are different handles; pick one and stay consistent.
- For prefix-typed handles (`Event:`, `Trait:`, etc.), the prefix names the *kind* of thing being declared, not its category. Do not invent prefixes for taxonomy purposes — the handle path is for that.
- See decision #6 for the full handle/description split rules.

### Scenarios

- One canonical kind per scenario; use the canonical form, not an alias. The linter will suggest the canonical when you write an alias.
- A Feature with zero `negative` scenarios is incomplete — almost every real unit has at least one failure mode worth pinning.
- Scenario count above ~10 is the design-rule warning boundary. If you have 10+ scenarios, the unit is doing too much; split it before reaching for `@allow-many-scenarios`.
- See decision #8 for the canonical kind set and aliases.

### RFC 2119 keywords

- Capitalize when you mean it: `MUST`, `SHALL`, `SHOULD`, `MAY`, `MUST NOT`, `SHALL NOT`.
- Lowercase `must` is prose. Use the all-caps form only when the linter's "violating an invariant breaks the build" semantics should apply.
- Place RFC 2119 keywords inside `Invariants:` entries and inside scenario step text — those are the two contexts the format treats them as load-bearing.
- See decision #7 for the recognition rules.

### Single-quoted literals

- Wrap exact identifiers, statuses, error names, and keys in single quotes inside step text: `Then it MUST raise 'PaymentError'`, `When the status becomes 'active'`.
- This is forward-compatible with the linter's planned literal-validation rule (decision #7) — once that ships, mismatched literals become diagnostics. Authoring with quotes today costs nothing and pays off later.

### Comments

- `#` at the start of a line, full-line only. Do not attempt inline `#` comments — the format has no escape syntax for them.
- Comments are for genuinely non-obvious context (a bug ticket reference, a workaround note). Do not narrate the structure: `# Provides:` followed by `Provides:` adds no value.
- See decision #10.

---

## Overrides

Some lint rules surface diagnostics that are *correct* but that the author has deliberately chosen to live with — a wrapper file genuinely consumes many handles because that's the wrapper's job; a small data-class genuinely has no invariants worth stating; a `cmd/main.go` deliberately has no consumers. These cases need a way to suppress the diagnostic *while preserving the documentation artifact* of the choice.

The mechanism is a frontmatter key starting with `@allow-`, with a quoted-string justification:

```trellis
@allow-many-consumes: "AST projection adapter; collapses tree-sitter and graph types into a single materialization step"

Feature: AST Projection
  "..."
```

The friction principle: **the justification text is the documentation**. Writing it down once is the cost of the suppression. Empty (`""`) and bareword values are technically accepted by the grammar but defeat the mechanism — a future lint rule (`override-without-justification`) will warn on them. Only quoted-string values currently silence the corresponding rule.

### Override keys

| Override key | Silences | When to use |
|---|---|---|
| `@allow-many-scenarios` | `scenario-count` | The unit is a state machine or protocol with N legitimate states; splitting it would be churn |
| `@allow-many-consumes` | `consumes-count` | Adapter, wrapper, or coordinator file whose contract genuinely depends on many external handles |
| `@allow-no-invariants` | `missing-invariants` | Trivial value type, pure-data record, or thin proxy with no meaningful invariants |
| `@allow-no-negative-scenario` | `missing-negative-scenario` | Navigation file, registry, or wirer with no meaningful failure mode worth pinning |
| `@allow-orphan-source` | `orphan-source-file` | Sidecar intentionally without a paired source file (rare; mostly for synthetic test fixtures) |

### What does NOT have an override

Some rules are deliberately not overridable, because the right response is to fix the underlying issue, not silence the message:

- `frontmatter-missing-required` — add the missing key
- `scenario-kind-canonical` — canonicalize the kind name
- `duplicate-provides` — deduplicate the architecture, not the diagnostic
- `broken-link` — either the consumer is wrong or there's a missing sidecar; fix one of them. Workspace-wide external-prefix declarations will eventually live in the policy file.

### Convention for the justification text

- One sentence, present tense, names the structural reason — not "TODO" or "for now."
- Describes *why* the rule's normal expectation doesn't apply here, not *what* the file does.
- Outlives the author. The next reader should understand the choice without context.

**Good:** `@allow-many-consumes: "wraps the tree-sitter Go binding; the per-method coupling is the binding's surface, not this file's design"`

**Bad:** `@allow-many-consumes: "TODO clean up later"`

---

## Anti-Patterns

These are the failure modes most often seen during the dogfood pass. Avoid them.

| Anti-pattern | Why it's bad |
|---|---|
| Sidecaring test files | Splits intent; tests should realize the scenarios in the *source file's* sidecar |
| Listing every stdlib call in `Consumes:` | Buries real broken-link diagnostics under unresolved noise |
| Empty `OutOfScope:` | The block is for naming things you've considered and rejected, not a placeholder; omit it if there's nothing to say |
| Restating the source code's behavior in scenarios | Scenarios capture *intent*, not implementation. "When the array is iterated, then each element is processed" is not a scenario. |
| Using `must`, `should`, etc. (lowercase) when you mean MUST or SHOULD | The linter treats only all-caps forms as load-bearing. Lowercase is just prose. |
| Inventing handle prefixes for taxonomy (`Internal:Foo`, `Public:Bar`) | Handles are names, not labels. Use frontmatter (`@stability: internal`) for properties of the unit. |
| Sidecars that consume nothing | Either the file genuinely has no contract-bearing dependencies (rare; double-check), or you are under-listing. The test in §"A practical test" will tell you which. |

---

## Where to Look Next

- **Format philosophy and motivation** — `trellis-plan/trellis-whitepaper.md`
- **Locked grammar and design decisions** — `trellis-plan/TREE_SITTER_DECISIONS.md`
- **Normative grammar** — `tree-sitter-trellis/grammar.js`
- **Worked examples** — `tree-sitter-trellis/examples/`, `trellis/testdata/valid/`, and the `.trellis` sidecars co-located with the source files in this repo (which dogfood the format against the toolchain itself)

This document will grow as more conventions surface from real-world authoring. Each addition should be dogfood-driven: a friction point seen in practice, written down once, with a worked example.
