# Trellis Agent Skill

You are working in a Trellis-aware codebase. Source files have a paired
`.trellis` sidecar that declares the file's contract: what it provides, what
it consumes, what is true about it, what is explicitly outside its scope, and
how it behaves under representative scenarios. **The sidecar is the spec —
the source code is its realization.** Your job is to keep the two in agreement.

This document is a behavioral checklist. Read it once at the start of a
session in this repository; revisit it whenever you are about to create or
modify a file.

---

## How to recognize a Trellis-aware repository

- A `trellis` binary is on `$PATH`, **or** the repo contains `*.trellis`
  files alongside source files.
- `*.trellis` files appear paired with source files using the convention
  `<source>.<ext>.trellis` — e.g. `create_subscription.rb` ↔
  `create_subscription.rb.trellis`.

If neither is true, the rest of this skill does not apply — treat the
codebase as conventional and ignore these gates.

---

## Mental model in one paragraph

A sidecar is read **before** changes are made to its source file. It tells
you the unit's intent in a structured way: `Provides:` lists the public
handles other code may depend on; `Consumes:` lists the dependencies that
would force a change here if their contracts changed; `Invariants:` are the
properties that must hold; `OutOfScope:` names what this unit deliberately
does **not** do. `Scenarios` describe behavior with `Given/When/Then` steps
tagged by kind (`happy-path`, `negative`, `edge`). `RFC 2119` keywords
(`MUST`, `SHALL`, `SHOULD`, `MAY`, with their negations) are load-bearing
when capitalized.

The dependency graph is built from `Provides:`/`Consumes:` declarations
across the workspace. The linter enforces structural and design rules. The
LSP surfaces both as you edit. The skill (this document) is the workflow
discipline that turns those tools into a churn-prevention loop.

---

## Workflow gate — BEFORE creating any new file

If you are about to create a new source file, **first** check whether
existing units already provide what you intend to add.

1. **Search for similar Provides.** A semantic-search subcommand is on the
   roadmap; until it ships, grep the workspace:

   ```sh
   grep -rn "Provides:" --include="*.trellis" -A 5 .
   # or, more targeted:
   grep -rn -B 1 "<verb-noun fragment>" --include="*.trellis" .
   ```

   Look for handles whose paths suggest the same concept. Scan their
   `FeatureName` and the summary line just below it.

2. **If a match looks plausible, STOP and ask.** Do not silently create the
   new unit. Surface the candidate to the user with the question:

   > "This appears related to `<existing handle / file>`. Should I extend
   > it instead of creating a new unit?"

   Duplication is the single most expensive failure mode this format
   exists to prevent. Pay the 30-second confirmation cost every time.

3. **If creating new is confirmed, write the `.trellis` sidecar FIRST.**
   The order matters. The sidecar is the agreement; the source code is the
   implementation of that agreement. Drafting the sidecar before the code
   forces you to commit to a contract before writing 200 lines you'll have
   to retrofit. Surface the sidecar for review before writing implementation.

4. **After implementation passes tests, update `@reviewed:`** to today's
   date.

---

## Workflow gate — BEFORE modifying any existing file

If a `.trellis` sidecar exists for the file you are about to change,
**read it in full** before editing the source. This is non-negotiable.

Then, before making the change, run these checks against the sidecar:

1. **Is your change in the file's `OutOfScope:` block?** If yes, STOP. Ask:

   > "This change appears to fall under `<entry from OutOfScope>`. Is the
   > scope being intentionally moved? If so, the sidecar's `OutOfScope:`
   > and `Provides:` need to be updated as part of this change."

   `OutOfScope:` is not advisory; it is a deliberate negative-space
   declaration. Crossing it without acknowledgement is the
   "helpful agent" failure mode in action.

2. **Does your change violate an `Invariant:`?** Capitalized `MUST`,
   `SHALL`, `MUST NOT`, `SHALL NOT` lines are contractual. If yes, STOP. Ask:

   > "This change weakens the invariant `<text>`. Is the invariant being
   > intentionally relaxed? If so, the sidecar needs to be updated and the
   > weakening explained."

3. **Does your change add a new `Consumes:` entry?** Verify the consumed
   unit exists in the workspace. Then surface the new dependency to the
   user — adding a cross-unit dependency is a meaningful architectural act
   that deserves an explicit moment of attention.

   To check whether a handle exists:

   ```sh
   grep -rn "Provides:" -A 20 --include="*.trellis" . | grep "<HandleName>"
   ```

   Or use the LSP: open the file in your editor and use jump-to-definition
   on the new `Consumes:` entry — it will only resolve if a provider exists.

4. **After implementation, run the linter** before reporting the task done:

   ```sh
   trellis lint <changed paths>
   ```

   Then update the sidecar's `@reviewed:` date if you touched the source's
   contract surface in any meaningful way.

---

## What goes in `Consumes:` (and what does not)

This is the most error-prone field. The rule:

> A handle belongs in `Consumes:` if and only if a change to that handle's
> contract — its name, signature, semantics, or existence — would force a
> corresponding change in this file.

- ✅ Internal cross-package functions you call: `parser.ParseFile`
- ✅ External library APIs whose signatures you depend on:
  `sitter.Parser.SetLanguage`
- ✅ Framework types used as parameters: `cobra.Command`
- ❌ Stdlib incidentals: `fmt.Errorf`, `len(slice)`
- ❌ Language built-ins
- ❌ Methods on a type you've already listed (the type carries the contract)

Practical test: *"If a teammate refactored this handle, would I want them
to grep for sidecars that mention it before they did so?"* Yes → list it.
No → it doesn't belong.

When in doubt: **fewer entries with sharper meaning beats more entries
that catalog every line of code.**

For language-specific guidance, see `spec/format.md` §"The Consumes
Discipline."

---

## CLI cheatsheet

The `trellis` binary exposes three subcommands. Run any with `--help` for
flag details.

| Command | What it does | When to reach for it |
|---|---|---|
| `trellis graph build <paths>` | Discover, parse, summarize the workspace graph | First sanity check in a new repo, or after large changes |
| `trellis graph deps <file.trellis>` | What this sidecar's `Consumes:` resolves to | "What does this unit depend on?" |
| `trellis graph dependents <file.trellis>` | What sidecars consume something this one provides | **Blast radius for a contract change.** Run before changing any `Provides:` entry. |
| `trellis graph downstream <file.trellis>` | Transitive closure of `dependents` | "If I delete this, what else breaks?" |
| `trellis graph orphans <paths>` | Sidecars whose Provides have no consumers | Clean-up audit; candidates for removal |
| `trellis graph parse <file.trellis>` | Print the parsed S-expression tree | Debugging a malformed sidecar |
| `trellis lint <paths>` | Run all rules, return non-zero on errors | Before declaring a change complete; in CI |
| `trellis lsp` | Start the Language Server over stdio | The editor uses this; you generally don't invoke it directly |

The graph commands operate on **sidecar paths**, not source paths. Pass
`create_subscription.rb.trellis`, not `create_subscription.rb`.

---

## Linter rules at a glance

The linter emits a stable, named diagnostic code for each finding. When you
see a code, the corresponding remediation is:

| Code | What it means | What to do |
|---|---|---|
| `frontmatter-missing-required` | Missing `@owner` / `@stability` | Add the key — no override exists |
| `scenario-kind-canonical` | Used an alias instead of canonical kind | Rename to canonical (e.g. `failure` → `negative`) — no override |
| `missing-invariants` | Feature has zero `Invariants:` entries | Add invariants, or override with `@allow-no-invariants: "<reason>"` |
| `missing-negative-scenario` | No `negative` scenario | Add one, or override with `@allow-no-negative-scenario: "<reason>"` |
| `scenario-count` | More than ~10 scenarios | Split the unit, or override with `@allow-many-scenarios: "<reason>"` |
| `consumes-count` | More than ~5–8 `Consumes:` entries | Reduce coupling, or override with `@allow-many-consumes: "<reason>"` |
| `stale-reviewed` | `@reviewed:` is >180 days old | Re-read the sidecar against the current source and bump the date — **no override** |
| `broken-link` | A `Consumes:` handle has no provider | Either fix the consumer (typo / wrong handle) or add the missing sidecar — no override |
| `duplicate-provides` | Two sidecars declare the same `Provides:` handle | Deduplicate the architecture, not the diagnostic — no override |
| `orphan-source-file` | The paired source file is gone | Delete the sidecar, or override with `@allow-orphan-source: "<reason>"` |

**Override justifications are documentation, not noise.** Write a single
present-tense sentence naming the structural reason. `"TODO clean up later"`
is the wrong shape; `"AST projection adapter; per-method coupling is the
binding's surface, not this file's design"` is the right shape.

---

## Common mistakes to avoid

- **Sidecaring test files.** Tests realize scenarios that already live in
  the *source file's* sidecar. Writing a sidecar for `foo_test.go`
  fragments intent across two files that will drift. The exception is
  integration tests covering a unit of intent distinct from any single
  source file — those get their own sidecar describing the integration
  boundary.
- **Listing every stdlib call in `Consumes:`.** This buries real
  `broken-link` diagnostics under unresolved noise.
- **Empty `OutOfScope:`.** Omit the block if you have nothing to declare.
  `OutOfScope:` is a deliberate negative-space artifact, not a placeholder.
- **Lowercase `must` / `should` when you mean `MUST` / `SHOULD`.** Only
  capitalized RFC 2119 keywords are treated as load-bearing.
- **Inventing handle prefixes for taxonomy** (`Internal:Foo`,
  `Public:Bar`). Handle prefixes name the *kind* of thing
  (`Event:`, `Trait:`), not its category. Use frontmatter
  (`@stability: internal`) for properties of the unit.
- **Restating implementation in scenarios.** Scenarios capture *intent*,
  not code paths. "When the array is iterated, each element is processed"
  is not a scenario.

---

## Decision tree — quick reference

```
Going to create a new source file?
  └─ grep workspace for similar Provides
       ├─ Found a candidate ── STOP, ask user
       └─ Truly novel ── write .trellis FIRST, then implement, then bump @reviewed

Going to modify an existing source file?
  ├─ Sidecar exists?
  │    ├─ Read it in full
  │    ├─ Change in OutOfScope? ── STOP, ask
  │    ├─ Violates an Invariant? ── STOP, ask
  │    └─ Adds a Consumes? ── verify provider exists, surface to user
  └─ No sidecar?
       └─ Authoring conventions in spec/format.md may require one — check

Done with a change?
  ├─ trellis lint <changed paths>     (must be 0 errors)
  ├─ Update @reviewed: if contract changed
  └─ Update Provides:/Consumes:/Invariants: if the change touched those
```

---

## Where to look next

- **Format reference + authoring conventions** — `trellis/spec/format.md`
- **The whitepaper** (philosophy and motivation) — the project website
- **Grammar and design decisions** — annotated comments in
  `tree-sitter-trellis/grammar.js`
- **Worked example sidecars** —
  `tree-sitter-trellis/examples/`,
  `trellis/testdata/valid/`,
  and the `.trellis` files co-located with source in this repo
  (the toolchain dogfoods the format against itself).

The skill is short on purpose. The discipline is in *running the gates
every time*, not in remembering long rulebooks. When in doubt, read the
sidecar.
