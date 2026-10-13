# SDD Workflow — Spec-Driven Development

This document is the operating manual for spec-driven development (SDD) in the
Flashbacks repository. It describes the OpenSpec lifecycle, the mandatory
branching step, and the scenario-authoring/evidence contract owned by
`focused-spec`.

| Concern | Owner |
|---|---|
| Lifecycle stages (propose → apply → sync → archive) | `.roo/skills/openspec-*` |
| CLI availability before every stage | `.roo/skills/openspec-install` |
| Branch creation before planning | `.roo/skills/feature-branch` |
| Scenario contract + executable evidence | `.roo/skills/focused-spec` |
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
   │ 0. preflight          │  openspec-install  → CLI present & current
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 1. PROPOSE  (plan)    │  feature-branch  → branch off main FIRST
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
   │ 3. SYNC   (merge)     │  openspec-sync-specs → delta specs merged into main specs
   └───────────┬───────────┘
               ▼
   ┌───────────────────────┐
   │ 4. ARCHIVE (close)    │  openspec-archive-change → changes/archive/YYYY-MM-DD-<name>/
   └───────────────────────┘

   side paths: openspec-explore (think, before/within a change)
               openspec-update-change (revise plan; never edits code)
```

| Stage | Skill | May write | Must not |
|---|---|---|---|
| preflight | `openspec-install` | CLI install/PATH, `openspec update` regeneration | touch `openspec/specs/`, `openspec/changes/` |
| propose | `feature-branch`, `openspec-propose`, `focused-spec` | `openspec/changes/<name>/**` | project code |
| apply | `openspec-apply-change`, `focused-spec` | project code, `tasks.md` checkboxes | main specs, change artifacts beyond tasks |
| sync | `openspec-sync-specs` | `openspec/specs/**` | change artifacts |
| archive | `openspec-archive-change` | moves `changeRoot` into `changes/archive/` | deleting implementation |

---

## 3. Stage 0 — preflight (run at the start of every stage)

`openspec-install` checks that the `openspec` CLI is available and recent:
`command -v openspec && openspec --version`, then a PATH/global-bin search, then
`npm install -g @fission-ai/openspec@latest` when it is genuinely missing.
Run `openspec update` after a fresh install or upgrade so the generated
`openspec-*` skills and commands in `.roo/` stay aligned with the CLI version.

Store selection: if the work lives in a registered store, run
`openspec store list --json` and keep `--store <id>` on every `new change`,
`status`, `instructions`, `list`, `show`, `validate`, `archive`, `doctor`,
`context`, `schemas`, and `view` call. Otherwise the commands act on the nearest
local `openspec/` root.

---

## 4. Stage 1 — propose (planning only)

### 4.1 Branch first — mandatory

**Before the propose step, a feature branch MUST already exist.** Creating it is
delegated to the `feature-branch` skill, never done ad hoc:

1. Derive a `short_description` from the intent (lowercase kebab-case, Latin +
   digits + hyphens, ≤ 30 characters, filler words stripped).
2. `git status --porcelain` — abort if the working tree is dirty (commit/stash first).
3. If the current branch is not `main`: `git checkout main`, `git pull origin main`,
   `git merge <current> --no-ff`, `git push origin main`. On conflicts:
   `git merge --abort`, report, stop. Do not create the branch until `main` is pushed.
4. `git checkout -b feature/<short_description>`
5. `git push -u origin feature/<short_description>`

Never force-push `main`, never bypass hooks. Planning artifacts, implementation,
and evidence all land on this branch.

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

## 7. Stage 3 — sync (merge delta into main specs)

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

## 8. Stage 4 — archive (close the change)

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

## 9. Definition of done

A change is done only when all of the following hold:

- [ ] Planning artifacts complete (`proposal`, `specs`, `design`, `tasks` as required).
- [ ] Every scenario has exactly one `WHEN`/`THEN` and an ID used as its `--scope`.
- [ ] No EVIDENCE entry is still `planned:` — each is a runnable check.
- [ ] `focused-spec validate --scope <name>` passes for every scenario scope.
- [ ] `focused-spec run --scope <name>` passes for every scenario scope.
- [ ] New/changed EVIDENCE survived the mutation check (fails when reverted, passes when restored).
- [ ] All tasks are `- [x]`; service tests and linters are green.
- [ ] `openspec validate --specs` passes after sync.
- [ ] Delta specs are merged into main specs before the change is archived.

---

## 10. Anti-patterns

| Anti-pattern | Why it is wrong |
|---|---|
| Starting `propose` on `main` (or on a stale feature branch) | Planning artifacts miss the mandatory `feature/<short_description>` branch and the merge-into-main step. |
| Writing `specs/**/spec.md` without loading `focused-spec` | Produces scenarios that are not bound to executable evidence. |
| Compound scenarios ("WHEN A and B … THEN X and Y") | Cannot be evidenced or mutation-checked as one minimal outcome. |
| Leaving `EVIDENCE: planned:` after implementing | Implementation cannot be verified; the task may not be checked off. |
| Passing tests accepted without a mutation check | A test that cannot fail proves nothing. |
| Mutating implementation instead of the behavior for the mutation check | The revert must be a reversible behavior diff, not a change to the evidence. |
| Declaring "implemented" before running the EVIDENCE CLI | Violates the completion gate in §6.2. |
| Editing code during `propose`/`explore`/`update-change` | Violates the stage boundary; the prompt authorized planning only. |
| Copying a delta spec verbatim into a main spec | Leaves delta headers in a main spec; breaks `validate --specs`. |
| Archiving with an in-flight spec sync | The archived change is never reflected in the main specs. |

---

## 11. Command cheat sheet

```bash
# preflight
command -v openspec && openspec --version
openspec update

# branch (feature-branch skill)
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
- `.roo/skills/` — the stage skills and `focused-spec` loader referenced above.
- `AGENTS.md` — shared coding standards that every apply task must satisfy.
