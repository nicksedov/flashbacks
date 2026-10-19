# SDD Workflow — Spec-Driven Development

This document is the operating manual for spec-driven development (SDD) in the
Flashbacks repository. It describes the OpenSpec lifecycle, the mandatory
branching step, and the scenario-authoring/evidence contract owned by
`focused-spec`.

| Concern | Owner |
|---|---|
| Lifecycle stages (propose → apply → sync → archive) | `.roo/skills/openspec-*` |
| Branch creation before planning | this document, §4.1 |
| Scenario contract + executable evidence | `.roo/skills/focused-spec` |
| Code validation of the apply result | this document, §7 |
| Project artifact rules & guidance | `openspec/config.yaml` |

---

## 1. Core principles

1. **The user prompt is the intent.** A request states *what* the user wants, not
   *which stage* to run. The stage skills are the authority on what a request
   authorizes: the request that starts `propose` authorizes **planning only**, the
   request that starts `apply` authorizes **implementation only**. Never carry an
   implementation instruction from a planning prompt into code.
2. **Specs are the source of truth.** Behavior is specified as requirements and
   scenarios before it is implemented; code follows specs, not the other way round.
3. **One scenario = one minimal outcome.** A scenario asserts exactly one
   observable behavior delta (a single `WHEN` → `THEN`).
4. **Evidence is executed, never asserted.** A behavioral requirement is only
   "done" when its evidence has been run through the CLI and observed to pass.

---

## 2. The cycle

```
        user prompt (intent)
                │
                ▼
   ┌───────────────────────┐
   │ 1. PROPOSE  (plan)    │  §4.1 branch     → branch off main FIRST
   │                       │  openspec-propose → proposal/specs/design/tasks
   │                       │  focused-spec     → scenario contract (planned:)
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 2. APPLY  (build)     │  openspec-apply-change → implement tasks
   │                       │  focused-spec → validate + run EVIDENCE (mutation check)
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 3. VALIDATE (review)  │  this document §7 → audit the diff, findings report
   │                       │  PASS → continue; CHANGES REQUIRED → back to apply
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 4. SYNC   (merge)     │  openspec-sync-specs → delta specs merged into main specs
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 5. ARCHIVE (close)    │  openspec-archive-change → changes/archive/YYYY-MM-DD-<name>/
   └───────────────────────┘

   side paths: openspec-explore (think, before/within a change)
               openspec-update-change (revise plan; never edits code)
```

| Stage | Skill | May write | Must not |
|---|---|---|---|
| propose | §4.1 branch step, `openspec-propose`, `focused-spec` | `openspec/changes/<name>/**` | project code |
| apply | `openspec-apply-change`, `focused-spec` | project code, `tasks.md` checkboxes | main specs, change artifacts beyond tasks |
| validate | none — this document §7 | the findings report only | project code, change artifacts, main specs |
| sync | `openspec-sync-specs` | `openspec/specs/**` | change artifacts |
| archive | `openspec-archive-change` | moves `changeRoot` into `changes/archive/` | deleting implementation |

---

## 3. Store selection (before every stage)

If the work lives in a registered store, run `openspec store list --json` and
keep `--store <id>` on every `new change`, `status`, `instructions`, `list`,
`show`, `validate`, `archive`, `doctor`, `context`, `schemas`, and `view` call.
Otherwise the commands act on the nearest local `openspec/` root.

---

## 4. Stage 1 — propose (planning only)

### 4.1 Branch first — mandatory

**Before the propose step, a feature branch MUST already exist.** Creating it is
part of this document — there is no separate branch skill — and is never done ad
hoc:

1. **Name the branch from the intent.** Derive a `short_description` from the
   user's description of the feature (lowercase kebab-case, Latin letters + digits
   + hyphens only, ≤ 30 characters, filler words stripped, still meaningful and
   recognizable). If the prompt does not describe an actual feature, ask the user
   for a short description — do not invent a branch name and do not proceed.
2. `git status --porcelain` — abort if the working tree is dirty; ask the user to
   commit or stash, and never discard their work.
3. If the current branch is not `main`: `git checkout main`, `git pull origin main`,
   `git merge <current> --no-ff`, `git push origin main`. On conflicts:
   `git merge --abort`, report, stop. Do not create the branch until `main` is pushed.
4. `git checkout -b feature/<short_description>`
5. `git push -u origin feature/<short_description>`

Branch step error handling, beyond the merge rules above:

- **`main` push fails** — report and stop; the new branch is not created.
- **`feature/<short_description>` already exists on `origin`** — stop and ask the
  user to pick a different name; force-push that branch only on an explicit request.
- **Publishing the new branch fails** — report and stop; do not start `propose` on
  an unpublished branch.
- **Any other step fails** — stop and report; never continue silently.

The branch step runs `git` in the workspace shell; do not route these operations
through the GitHub MCP server. Never force-push `main`, never use `--no-verify` or
otherwise bypass hooks unless the user explicitly asks. Planning artifacts,
implementation, and evidence all land on this branch.

Close the step with a short report: branch name, base (`main`, merged from
`<previous_branch>`), and the published ref `origin/feature/<short_description>`.

### 4.2 Scaffold the change

```bash
openspec new change "<kebab-case-name>"        # add --schema <name> only if explicitly requested
openspec status --change "<name>" --json       # applyRequires, artifacts, requires, planningHome, changeRoot
openspec instructions <artifact-id> --change "<name>" --json
```

Never hand-create a change directory: the CLI scaffold writes `.openspec.yaml`
and the path metadata. With the default `spec-driven` schema the required
artifact set is:

| Artifact | Path | Content |
|---|---|---|
| `proposal` | `proposal.md` | what and why, plus a Non-goals section |
| `specs` | `specs/<capability-path>/spec.md` | delta spec (requirements + scenarios) |
| `design` | `design.md` | how (conditional per its own `instruction`) |
| `tasks` | `tasks.md` | small, independently verifiable implementation steps |

`<capability-path>` is the spec directory relative to `specs/` (for example
`trash` or `identity/user-auth`). Preserve an existing capability's full path.

Read each artifact's `instruction` (authoritative), `template` (structure),
`rules`, and `context` (constraints — never copied into the file). After writing
each artifact, re-run `openspec status --change "<name>" --json` and continue
until every artifact in the required set is `done`, `skipped`, or deliberately
skipped as conditional. Finish with the unscoped status command and stop — the
apply workflow starts only on a new user request.

`openspec/config.yaml` rules that always apply here: proposals stay focused on a
single capability and reference affected OpenAPI/MCP contracts; delta specs use
`## ADDED Requirements` / `## MODIFIED Requirements`; JSON field names stay
camelCase and aligned with Go JSON tags and webapp TS types; every new
user-facing string specifies both `en` and `ru`.

---

## 5. Authoring scenario specs — the `focused-spec` contract

**Load the `focused-spec` skill before creating or changing any behavioral
requirement or scenario.** This is a MUST for every authoring or revision step —
including a later step in the same session, a revision of an existing
`specs/**/spec.md`, and cases where the framework is not OpenSpec or the user
never names `focused-spec`. Load it *alongside* the host workflow skill
(`openspec-propose`, `openspec-update-change`, `openspec-explore`).

The skill is a loader: it does not embed the workflow. It requires resolving the
**project-local** `focused-spec` installation and requesting its workflow
guidance as a standalone command:

```bash
# resolve the project-local install (never a PATH/global/npx fallback)
node_modules/.bin/focused-spec instructions --protocol 1 --topic workflow --json
# later, only when the workflow calls for runner work:
node_modules/.bin/focused-spec instructions --protocol 1 --topic runners --json
```

Accept the guidance only if stdout is one valid JSON object with
`protocolVersion: 1`, `package.name: focused-spec`, a `package.version` equal to
the resolved installation's version, the requested `topic`, and non-empty
`content`. Otherwise stop and report that the project needs a complete,
compatible local installation.

> **Local install prerequisite.** `focused-spec` is a project-local dependency:
> [`package.json`](../../package.json:1) declares `focused-spec@^0.9.1` plus the
> evidence toolchain it drives — `@focused-spec/runner-go`,
> `@focused-spec/runner-pytest`, `@focused-spec/runner-vitest`,
> `@focused-spec/format-gherkin`, `@focused-spec/format-markdown`. The installed
> binary resolves at `node_modules/.bin/focused-spec`, which is also what
> `npx --no-install` uses — `--no-install` never downloads a copy, so both forms
> always run the project-local CLI. If the binary is missing (fresh clone), run
> `npm install` at the repository root before authoring specs. Treat a missing or
> incompatible local installation as a blocker for spec authoring, not as a reason
> to fall back to a global copy (see `.roo/skills/focused-spec/SKILL.md`).

### 5.1 The contract of a scenario

Every scenario carries four things:

| Element | Meaning |
|---|---|
| **ID** | Stable identifier of the scenario. This is the `<name>` used by `--scope <name>` in the CLI during apply. |
| **WHEN** | The single triggering condition / input. |
| **THEN** | The single observable outcome the system must produce. |
| **EVIDENCE** | The executable check that proves the `THEN`. At authoring time it is declared as `planned:` because the test does not exist yet; it must become a real, runnable check before implementation is declared complete. |

The evidence lifecycle:

```
propose        apply
  │             │
  │  EVIDENCE: planned: <what will prove the THEN>
  │             ├─ write the test / runnable check
  │             ├─ focused-spec validate --scope <name>   (structure + binding ok)
  │             ├─ focused-spec run --scope <name>        (behavior observed = expected)
  │             └─ mutation check for new/changed EVIDENCE (see §6.2)
  ▼             ▼
spec.md       green, executed evidence → task may be checked off
```

### 5.2 Minimality and structure rules

- **One scenario — one minimal outcome.** Exactly one `WHEN` and one `THEN` per
  scenario. Split compound `AND`-chains into separate scenarios; a scenario that
  asserts two independent outcomes cannot be evidenced cleanly.
- **Use the default OpenSpec structure** inside the change: a delta spec at
  `specs/<capability-path>/spec.md` with `## ADDED Requirements` /
  `## MODIFIED Requirements` / `## REMOVED Requirements` / `## RENAMED Requirements`
  sections, `### Requirement:` blocks, and `#### Scenario:` blocks containing
  `- **WHEN** …` / `- **THEN** …` bullets. Main specs are never edited at propose
  time; the delta is merged later by sync/archive.
- **Requirement text is normative** (`SHALL`), scenarios are concrete examples of
  it. Do not restate the requirement inside the scenario.
- A `MODIFIED` block carries the whole requirement — body plus every scenario that
  survives — `openspec validate` and `openspec archive` reject one that drops a
  scenario the main spec still has.
- `## Purpose` in a delta is only for a brand-new capability and seeds the new main
  spec; the main spec's own Purpose wins afterwards.

---

## 6. Stage 2 — apply (implementation + evidence)

### 6.1 Implement the tasks

```bash
openspec status --change "<name>" --json
openspec instructions apply --change "<name>" --json   # contextFiles, progress, tasks,
                                                       # optional context + operationGuidance
```

Read **every** file under `contextFiles` from disk (not from conversation memory)
before coding. Then work the task list: keep changes minimal and scoped to the
task, mark `- [ ]` → `- [x]` immediately after the task's specified behavior is
fully implemented, and pause instead of silently narrowing, deferring, or
absorbing scope beyond the spec.

Per project rules, every change also carries its service checks: Go
`go test ./internal/application/... -count=1` (core) or `go test ./... -count=1`
(exif/ocr), webapp `npm run lint && npx tsc -b` plus `npm test`, and `make
generate-types` when an OpenAPI contract changed. `openspec/config.yaml`
`operations.apply.guidance` adds the advisory equivalents of the same rules.

### 6.2 Evidence execution gate — before declaring implementation complete

**Every scenario in the change's `spec.md` MUST have its EVIDENCE executed through
the CLI.** Before finishing the apply step, run for each scenario scope:

```bash
npx --no-install focused-spec validate --scope <name>
focused-spec run --scope <name>
```

- `validate` proves the scenario's evidence is well-formed and actually bound to
  the scenario's `WHEN`/`THEN` (not merely present as a test).
- `run` proves the behavior is observed: the check passes against the implemented
  system. A `planned:` evidence entry that was never turned into a runnable check
  fails here.

**Mutation check for new or changed EVIDENCE.** Green is not proof; a test that
cannot fail proves nothing. For every new or modified evidence entry:

1. Temporarily revert the behavior under test with a **reversible diff** (revert
   the implementation hunk only — keep the test untouched).
2. Run `focused-spec run --scope <name>`; it **MUST FAIL** on the behavioral
   assertion (a failure for the wrong reason — compile error, missing fixture,
   unrelated crash — does not count).
3. Restore the behavior and run again; it **MUST PASS**.

**Do not declare the implementation complete until every scenario's EVIDENCE has
been executed.** Until then:
- the task stays `- [ ]`;
- the apply summary must not say the change is implemented;
- `archive` must not be started.

Report the evidence run inline (scope, command, observed result), and record the
mutation-check pair (fail → restore → pass) for each new/changed entry.

### 6.3 Pause conditions

Stop and surface the issue instead of guessing when a task is ambiguous, when the
implementation reveals a design problem (suggest `openspec-update-change`), when
the work exceeds what the spec and tasks describe, or when a blocker appears.

---

---

## 7. Stage 3 — validate (code review of the diff)

Apply proves the scenarios run; validate inspects the diff that made them run. It
is a read-only, evidence-consuming review: the output is a findings report, not a
patch, and a blocking finding returns the change to apply.

### 7.1 Inputs and boundaries

Required inputs, read from disk:

- the branch diff against `main` — `git diff --stat origin/main...HEAD`, then the
  per-file diff for every changed path;
- the change's delta `spec.md` (scenarios) and `tasks.md`;
- the apply report: executed component checks (§6.1) and per-scope evidence results
  including the mutation-check pairs (§6.2).

The stage itself must not:

- **re-run the apply gates as a substitute for reading the diff.** Component
  checks belong to §6.1 and the evidence gate to §6.2. Re-run only what the diff
  changed *after* the apply report was written; a missing or stale report is the
  blocking finding, not something validation repairs.
- **write anything.** No code edits, no spec rewrites, no artifact updates. A
  blocking finding reopens apply; if it shows the *spec* is wrong, suggest
  `openspec-update-change`.
- **restate component rules.** Their authority is
  [`docs/agent/go-services.md`](go-services.md) and
  [`docs/agent/webapp.md`](webapp.md); §7 defines how a diff is audited against
  them and which deviations block.

How §7 relates to the other stages:

| Instruction | Lives in | Relation to validate |
|---|---|---|
| Service checks (tests, lint, build, type generation) | `go-services.md` §2, `webapp.md` §8, §6.1 | consume the reported results; never duplicate them |
| Scenario contract and executable evidence | §5, §6.2 | verify the diff matches the scenarios; never re-author or re-run evidence |
| Minimal, scoped changes | §6.1, §6.3 | reject scope creep instead of absorbing or re-planning it |
| Component rules and definitions of done | `go-services.md` §9, `webapp.md` §9 | applied as a checklist, never restated here |

If the diff touches both a Go service and the webapp, run both audits (§7.3, §7.4).

### 7.2 Diff hygiene (all components)

- Every hunk traces to a task in `tasks.md`; every task with user-visible behavior
  has a scenario and an executed evidence entry. Untasked changes are blocking
  scope creep (see §6.3).
- No dead or debug residue: `fmt.Print*`/`console.log`, commented-out blocks,
  skipped tests (`.skip`, `t.Skip`, `@pytest.mark.skip`), disabled lint rules, or
  `TODO` without a tracked issue.
- No secrets, credentials, tokens, or machine-specific absolute paths; no
  environment-specific values baked into code.
- Generated artifacts that the diff invalidates are regenerated and committed
  (`wire_gen.go` after a DI change, `webapp/src/types/api.ts` after an OpenAPI
  change, migrations alongside the model they serve).
- Public API changes are additive or explicitly requested; a breaking change
  without a requirement and a coordinated consumer update is blocking.

### 7.3 Backend audit (Go services)

`go-services.md` §1–§9 is the rule source. Beyond those rules, a backend diff is
audited for the following failure modes.

**Service boundaries and resilience**

- No cross-service database access in either direction — *reads included*. A GORM
  query from `core` against exif-owned tables (`image_metadata`,
  `geolocation_caches`), or from `exif` against core-owned tables, is blocking
  even when read-only; cross-service joins are forbidden.
- Cross-service traffic goes through the typed client, never `http.Get` or ad-hoc
  `net/http` calls from application code.
- Every outbound call carries a `context` timeout and propagates cancellation; a
  slow or unavailable downstream degrades (fallback, partial data, explicit error)
  instead of hanging the request.
- Retries are bounded with backoff; new or changed mutating endpoints and client
  calls are idempotent or deduplicated.
- A multi-step cross-service operation defines its failure mode and compensating
  action, and a retry can resume rather than corrupt state.
- Schema ownership stays with the owning service: migrations and models do not
  leak across services, and shared domain types live in `backend/shared/domain/`.

**Concurrency and lifecycle**

- Every goroutine and background job has an owner and stops on context
  cancellation; no leaked timers, pollers, or sends after shutdown.
- Shared maps, slices, and caches are synchronized; upstream cancellation reaches
  downstream calls.
- Transaction boundaries cover the whole operation; an error path leaves no
  partial write.

**Security**

- Auth, CSRF, and login rate limiting are not weakened; protected endpoints still
  validate the session.
- Input is validated at the handler boundary; file paths and external tool input
  (`exiftool`, Tesseract) cannot be driven by unsanitized values.
- Secrets are never logged or hardcoded; client-facing errors stay structured and
  use the i18n layer.

**Observability**

- `slog` entries carry request/correlation context and no sensitive data.
- Trace IDs propagate across service boundaries; health checks report real
  dependency state rather than a constant.

Advisory, never blocking on their own: smaller functions, naming, duplication,
query micro-optimizations, extra logging context, documented future improvement.

### 7.4 Webapp audit (TypeScript/React)

`webapp.md` §3–§7 is the rule source. Beyond those rules:

- **Effects and lifecycle:** dependency arrays are complete; subscriptions,
  intervals, and listeners are cleaned up on unmount; callbacks that outlive a
  render do not close over stale state.
- **Data flow:** shared state stays in the existing providers; no prop drilling
  through three or more levels; business logic lives in hooks, not JSX.
- **Contracts:** DTO field names match the Go JSON tags exactly; typed API helpers
  are used; API errors are translated before display; an expired session routes to
  login instead of failing silently.
- **Async states:** loading, error, and empty states are handled as deliberately
  as the success state; no unhandled promise rejections.
- **Performance:** maps, lightboxes, viewers, and other heavy components are
  lazy-loaded; lists and grids render thumbnails rather than full-size images; no
  new dependency without justification and no unbounded re-render loop.
- **Design system:** UI changes follow
  [`openspec/specs/design-system/spec.md`](../../openspec/specs/design-system/spec.md)
  with theme tokens; dark/light and keyboard accessibility are not regressed.

Advisory, never blocking on their own: extracting a smaller component, clearer
prop names, memoization where a re-render is measurable, stricter narrowing.

### 7.5 Blocking vs advisory findings

Blocking — the change returns to apply before sync:

- a component definition-of-done item is unmet (`go-services.md` §9,
  `webapp.md` §9);
- the apply report is missing, stale, or claims a check that does not pass;
- implemented behavior has no scenario, or a scenario has no matching diff;
- contract drift: JSON tag ↔ TS field, OpenAPI/MCP schema, or generated types out
  of sync with the code;
- a service-boundary, concurrency, security, or timeout rule above is violated;
- untasked scope creep, dead/debug residue, or a committed secret;
- a breaking public API change without an explicit requirement.

Advisory — recorded in the report, never a gate:

- readability, naming, duplication, function size;
- micro-optimizations without a measured problem;
- additional observability or comments;
- anything that is a preference rather than a rule in `go-services.md`,
  `webapp.md`, `openspec/config.yaml`, or the change's spec.

Validation never introduces new requirements. A "would be nicer" observation with
no rule or scenario behind it stays advisory, and no blocking finding may rest on
taste alone.

### 7.6 Report and conclusion

Report inline, in the same style as the apply summary: reviewed scope (paths and
hunks), one line per audit area, then each finding as

```
<blocking|advisory> — <file>:<line> — <rule or scenario violated> — <why it matters> — <concrete fix>
```

Conclude with exactly one verdict:

- **PASS** — no blocking finding open; proceed to sync (§8).
- **CHANGES REQUIRED** — at least one blocking finding; return to apply, fix, and
  re-validate the *new* diff. Never hand an open blocking finding to sync or
  archive as a known issue.

### 7.7 Heuristics

- Would this still behave correctly with partial, stale, or missing data?
- Would it survive a slow or unavailable downstream service?
- Would it survive two concurrent requests or an unmount mid-operation?
- Does the test prove behavior, or only execute code? (§6.2 is the proof.)
- Is the error path as deliberate as the success path?
- Is every renamed field consistent across Go tag, TS type, and OpenAPI?
- Would this be understandable in six months without the conversation that
  produced it?

---

## 8. Stage 4 — sync (merge delta into main specs)

`openspec-sync-specs` is an **agent-driven intelligent merge**, not a copy:

```bash
openspec status --change "<name>" --json            # planningHome.root, artifactPaths.specs.existingOutputPaths
openspec instructions specs --change "<name>" --json # fetch the specs rules once before the first write
openspec validate --specs
```

For each delta path in `artifactPaths.specs.existingOutputPaths`: read the delta,
read the main spec at `<planningHome.root>/openspec/specs/<capability-path>/spec.md`,
then apply `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED` semantically, preserving
everything the delta does not mention. Never copy a delta file into a main spec
as-is: main specs keep a single `## Requirements` section with no delta operation
headers. Capability retirement (deleting a drained `spec.md`) requires every
condition in the skill, including `retire_capabilities: true` in the change's
`.openspec.yaml`. The operation must be idempotent. Validation failure means the
sync did not succeed — report it.

---

## 9. Stage 5 — archive (close the change)

```bash
openspec instructions archive --change "<name>" --json   # advisory only, never blocking
openspec status --change "<name>" --json                 # artifact completion
openspec list --json
```

Then: warn (and confirm) on incomplete artifacts or unchecked tasks; assess the
delta sync state and offer **Sync now** / archive-without-syncing; if syncing, run
the `openspec-sync-specs` workflow **inline** — never delegated to a background
task, because archiving moves `changeRoot` out from under an in-flight sync; then
re-verify every capability against its delta before moving anything. Finally move
`changeRoot` to `<planningHome.changesDir>/archive/YYYY-MM-DD-<name>/`
(never stack a second date; fail if the target exists). `.openspec.yaml` moves
with the directory. Summarize the change, schema, archive location, sync outcome,
and any warning.

---

## 10. Definition of done

A change is done only when all of the following hold:

- [ ] Planning artifacts complete (`proposal`, `specs`, `design`, `tasks` as required).
- [ ] Every scenario has exactly one `WHEN`/`THEN` and an ID used as its `--scope`.
- [ ] No EVIDENCE entry is still `planned:` — each is a runnable check.
- [ ] `focused-spec validate --scope <name>` passes for every scenario scope.
- [ ] `focused-spec run --scope <name>` passes for every scenario scope.
- [ ] New/changed EVIDENCE survived the mutation check (fails when reverted, passes when restored).
- [ ] Validate stage (§7) run on the final diff with verdict **PASS** and no open blocking finding.
- [ ] All tasks are `- [x]`; service tests and linters are green.
- [ ] `openspec validate --specs` passes after sync.
- [ ] Delta specs are merged into main specs before the change is archived.

---

## 11. Anti-patterns

| Anti-pattern | Why it is wrong |
|---|---|
| Starting `propose` on `main` (or on a stale feature branch) | Planning artifacts miss the mandatory `feature/<short_description>` branch and the merge-into-main step. |
| Writing `specs/**/spec.md` without loading `focused-spec` | Produces scenarios that are not bound to executable evidence. |
| Compound scenarios ("WHEN A and B … THEN X and Y") | Cannot be evidenced or mutation-checked as one minimal outcome. |
| Leaving `EVIDENCE: planned:` after implementing | Implementation cannot be verified; the task may not be checked off. |
| Passing tests accepted without a mutation check | A test that cannot fail proves nothing. |
| Mutating implementation instead of the behavior for the mutation check | The revert must be a reversible behavior diff, not a change to the evidence. |
| Declaring "implemented" before running the EVIDENCE CLI | Violates the completion gate in §6.2. |
| Skipping §7 validate because "the tests are green" | Tests prove scenarios; they do not prove that the diff respects boundary, concurrency, and contract rules. |
| Re-running the apply gates during validate instead of reading the diff | Duplicates §6.1/§6.2 work and masks a missing apply report, which is itself blocking. |
| Blocking a change on a style preference with no rule or scenario behind it | Turns review into taste; such observations are advisory only. |
| Starting sync or archive with an open blocking finding | Ships a change that never passed its own gate. |
| Editing code during `propose`/`explore`/`update-change` | Violates the stage boundary; the prompt authorized planning only. |
| Copying a delta spec verbatim into a main spec | Leaves delta headers in a main spec; breaks `validate --specs`. |
| Archiving with an in-flight spec sync | The archived change is never reflected in the main specs. |

---

## 12. Command cheat sheet

```bash
# branch (§4.1)
git branch --show-current
git checkout main && git pull origin main
git merge <current> --no-ff -m "Merge branch '<current>' into main" && git push origin main
git checkout -b feature/<short_description> && git push -u origin feature/<short_description>

# propose
openspec new change "<name>"
openspec status --change "<name>" --json
openspec instructions <artifact-id> --change "<name>" --json

# focused-spec guidance (project-local only)
node_modules/.bin/focused-spec instructions --protocol 1 --topic workflow --json
node_modules/.bin/focused-spec instructions --protocol 1 --topic runners --json

# apply
openspec instructions apply --change "<name>" --json
npx --no-install focused-spec validate --scope <name>
focused-spec run --scope <name>

# validate (read-only review of the diff, §7)
git diff --stat origin/main...HEAD
git diff origin/main...HEAD -- <path>

# sync
openspec instructions specs --change "<name>" --json
openspec validate --specs

# archive
openspec instructions archive --change "<name>" --json
openspec list --json
```

### Related files

- `openspec/config.yaml` — project context, artifact rules, apply/archive guidance.
- `openspec/specs/**/spec.md` — main capability specs (post-sync truth).
- `openspec/changes/<name>/` — active change; `openspec/changes/archive/` — closed changes.
- `.roo/skills/` — the stage skills and `focused-spec` loader referenced above;
  code review is a workflow stage (§7), not a skill.
- `AGENTS.md` — shared coding standards that every apply task must satisfy.
