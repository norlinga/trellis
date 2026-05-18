---
name: trellis
description: Use when creating, modifying, reviewing, or validating Trellis .trellis sidecar files; enforces Trellis-aware workflow gates, sidecar contracts, graph checks, and linting discipline.
---

# Trellis Agent Skill

You are working in a Trellis-aware codebase. Source files have a paired
`.trellis` sidecar that declares the file's contract: what it provides, what
it consumes, what is true about it, what is explicitly outside its scope, and
how it behaves under representative scenarios. **The sidecar is the spec —
the source code is its realization.** Your job is to keep the two in agreement.

This document is a behavioral checklist and a compact authoring reference.
Read it once at the start of a session in this repository; revisit it
whenever you are about to create or modify a file.

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

## Install and workspace assumptions

This skill is often copied out of the Trellis toolchain repo and into an
application repo. Do not assume the app repo contains the Trellis source tree.

- **Application repo root** means the root of the project whose source files
  have `.trellis` sidecars. Run `trellis lint`, `trellis graph ...`, `grep`,
  and editor/LSP checks from that root unless the user tells you otherwise.
- **Sidecars live next to source files.** A Rails file such as
  `app/controllers/files_controller.rb` pairs with
  `app/controllers/files_controller.rb.trellis`. An ERB template such as
  `app/components/ui/card_component.html.erb` pairs with
  `app/components/ui/card_component.html.erb.trellis`.
- **This skill file may live anywhere your agent loads rules.** Examples:
  `.claude/skills/trellis/SKILL.md`, `.cursor/rules/trellis.md`,
  `.agents/skills/trellis/SKILL.md`, or a plain context/rules directory. Its
  location does not change where sidecars belong.
- **References to `spec/format.md`, `testdata/valid/`, or
  `tree-sitter-trellis/` may point to the Trellis toolchain repo, not the app
  repo.** If those paths are missing locally, use the rules in this file as
  the minimum authoring contract and rely on `trellis lint` for enforcement.

When installation is unclear, first run:

```sh
command -v trellis
trellis --help
find . -name "*.trellis" | head
```

If the binary is missing, do not invent sidecar syntax from memory. Ask the
user whether to install/build Trellis or continue with a docs-only review.

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

## Handle authoring rules

`Provides:` and `Consumes:` bullets begin with exactly one Trellis handle.
That handle is the graph identity. Text after the handle is only description.

Use this shape:

```trellis
Provides:
  - Module.Class.method_name
  - Module.Class.method_name -> ReturnType
  - ComponentNameView.render

Consumes:
  - OtherModule.OtherClass.contract_method
```

Core rules:

- **Use dots for namespace and method paths.** Write
  `Api.V1.FilesController.index`, not `Api::V1::FilesController#index`.
- **Handles are globally unique across the workspace.** Two sidecars cannot
  both provide `Rendered`, `GET`, `Files.index`, or `QrCodes.create`.
- **One contract per bullet.** Do not put a list of methods, variables, or
  route names in one entry.
- **The first token matters.** `Rendered table wrapper` provides the handle
  `Rendered`; `Api.ErrorHandling render_authentication_error` provides
  `Api.ErrorHandling`. Both are usually too broad and will collide.
- **Do not use language sigils or placeholders in handles.** Avoid `@team`,
  `Foo#bar`, `Foo::Bar`, `Onboarding::<StepName>Form`, route strings, and
  numeric path segments such as `Files.204`.
- **Do not fake empty dependencies.** If there are no external contract
  dependencies, omit `Consumes:`. Do not write `Nothing external`,
  `None`, or `N/A`; those are parsed as handles.

Wrong and right examples:

```diff
-    - GET /api/v1/files
+    - Api.V1.FilesController.index

-    - Api.ErrorHandling render_authentication_error(message)
+    - Api.ErrorHandling.render_authentication_error

-    - Rendered table wrapper
+    - Ui.TableComponentView.render

-    - @team dashboard context
+    - DashboardController.index

-    - QrCodes.create
+    - Api.V1.QrCodesController.create
```

If you are unsure what handle to use, search existing sidecars and copy the
local convention before creating a new one.

---

## Rails handle conventions

Rails names are Ruby names in source code and Trellis names in sidecars.
Convert `::` and `#` to dots, then include enough namespace to be globally
unique.

| Rails unit | Trellis handle pattern | Example |
|---|---|---|
| Controller action | `Namespace.ControllerName.action` | `Api.V1.FilesController.create` |
| Non-namespaced controller action | `ControllerName.action` | `FilesController.destroy` |
| Concern method | `Namespace.ConcernName.method` | `Api.ErrorHandling.render_not_found_error` |
| Service object method | `ServiceName.method` | `StripeCheckoutService.create_session` |
| Model contract | `ModelName.method_or_contract` | `Team.current_bandwidth_usage` |
| ViewComponent Ruby class | `Namespace.ComponentName.method` | `Ui.CardComponent.card_classes` |
| ERB/ViewComponent template | `Namespace.ComponentNameView.render` | `Ui.CardComponentView.render` |
| Helper method | `HelperModule.method` | `FilesHelper.file_size_label` |
| Job | `JobName.perform` | `FormSubmissionDigestJob.perform` |
| JavaScript controller | `ControllerName.action` | `CollectionMembershipController.add` |

Rails-specific guardrails:

- Do not use HTTP verbs or route paths as `Provides:` handles. `GET /files`
  and `POST /checkout` collide quickly and do not name the unit's contract.
- Do not use bare resource/action handles when namespaces differ. In a Rails
  app with both browser and API controllers, `QrCodesController.create` and
  `Api.V1.QrCodesController.create` are distinct contracts; `QrCodes.create`
  is too ambiguous.
- For ERB templates, do not provide `Rendered`. Use a view-specific handle
  such as `BandwidthStatsComponentView.render`.
- Describe instance variables, assigns, route helpers, partials, and DOM
  targets in `Invariants:` or scenario steps unless they are real contracts
  provided by another sidecar.
- Framework classes such as `ApplicationController`, `ActiveRecord.Relation`,
  `Devise.SessionsController`, and `ViewComponent.Base` create `broken-link`
  warnings unless matching provider sidecars exist. List them only when that
  dependency is valuable enough to model in the graph.

---

## Workflow gate — BEFORE creating any new file

If you are about to create a new source file, **first** check whether
existing units already provide what you intend to add.

1. **Search for similar Provides.** A semantic-search subcommand is on the
   roadmap; until it ships, grep the workspace:

   ```sh
   grep -rn "Provides:" --include="*.trellis" -A 8 .
   # or, more targeted:
   grep -rn -B 1 "<verb-noun fragment>" --include="*.trellis" .
   ```

   Look for handles whose paths suggest the same concept. Scan the feature
   name and the summary line just below it. In Rails apps, search both the
   Ruby name and the Trellis dotted form, such as `Api::V1::FilesController`
   and `Api.V1.FilesController`.

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

4. **Lint the new sidecar before writing a batch of more sidecars.**
   Parse and duplicate-provider errors compound quickly. For batch coverage,
   run `trellis lint <new-file.trellis>` after each few files, not only at
   the end.

5. **After implementation passes tests, update `@reviewed:`** to today's
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
- ✅ Rails app contracts with provider sidecars: `Api.ErrorHandling.error_response`
- ❌ Stdlib incidentals: `fmt.Errorf`, `len(slice)`
- ❌ Language built-ins
- ❌ Methods on a type you've already listed (the type carries the contract)
- ❌ Placeholder prose: `Nothing external`, `No dependencies`, `None`
- ❌ Incidental Rails framework plumbing unless modeled with provider sidecars:
  route helpers, partial names, controller base classes, Active Record helper
  methods, and framework exception classes

Practical test: *"If a teammate refactored this handle, would I want them
to grep for sidecars that mention it before they did so?"* Yes → list it.
No → it doesn't belong.

When in doubt: **fewer entries with sharper meaning beats more entries
that catalog every line of code.**

For a new app or a high-volume sidecar batch, prefer sparse `Consumes:`.
Only list handles that already resolve, or add the provider sidecar in the
same change. Put incidental framework context in `Invariants:` prose instead
of turning the first lint run into a wall of `broken-link` warnings.

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

## First lint run triage

A first Trellis pass over an existing Rails app may produce many diagnostics.
Do not treat every line as equally urgent.

1. **Fix parse errors first.** These usually come from invalid handle syntax:
   `::`, `#`, `@ivar`, route paths, angle brackets, or numeric path segments.
   Use dotted Trellis handles.
2. **Fix `duplicate-provides` errors next.** These mean the graph cannot
   identify one owner for a contract. Rename generic handles like `GET`,
   `POST`, `Rendered`, `index`, or `QrCodes.create` to globally unique
   handles.
3. **Reduce `broken-link` warnings intentionally.** A `Consumes:` entry must
   match a `Provides:` handle somewhere in the linted workspace. Either add
   the provider sidecar, correct the handle, or remove the consume and describe
   the relationship in prose.
4. **Add negative scenarios over time.** `missing-negative-scenario` is design
   pressure. Add a realistic failure mode, or use
   `@allow-no-negative-scenario: "<reason>"` for files that genuinely have no
   meaningful negative behavior.
5. **Canonicalize scenario kinds.** Use only `happy-path`, `negative`, and
   `edge`. UI states such as "warning", "danger", and "success" belong in
   scenario text, not in the scenario kind.

The linter exits non-zero when any error-severity diagnostic remains. Warnings
should still be cleaned up, but a project can ratchet warnings down over time
after parse and duplicate-provider errors are gone.

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
- **Listing every Rails object touched by a file in `Consumes:`.** Models,
  policies, route helpers, partials, framework base classes, and Stimulus
  targets should be listed only when they are contract-bearing and have
  provider sidecars.
- **Using Ruby notation as Trellis handles.** `Foo::Bar`, `Foo#bar`, and
  `@ivar` are natural Ruby, but Trellis handles use dot paths.
- **Using HTTP verbs, paths, or `Rendered` as `Provides:` handles.** These are
  descriptions, not globally unique contract names.
- **Writing fake empty dependencies.** Omit `Consumes:` instead of writing
  `Nothing external`, `None`, or `N/A`.
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
  ├─ Fix parse-failed / duplicate-provides before moving on
  ├─ Remove or resolve new broken-link warnings you introduced
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

The skill keeps the workflow gates compact, but the handle rules are
load-bearing. The discipline is in *running the gates every time* and using
the canonical handle shape consistently. When in doubt, read the sidecar and
run the linter.
