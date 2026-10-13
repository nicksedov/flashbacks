# Flashbacks — Agent Context

> **Project:** Flashbacks - Microservice-based duplicate image finder
> **Architecture:** Microservices (webapp, core, exif, ocr)
> **See:** [README.md](README.md) for project overview

## Task Mode — SDD or Direct

**The user decides the mode of a task, never the agent.**

Before starting any task that changes files, you MUST ask the user whether it
should be run through the SDD cycle ([`docs/agent/sdd-workflow.md`](docs/agent/sdd-workflow.md))
or done directly.

- Do NOT choose the mode yourself, and do NOT start file-changing work before the
  user answers.
- Ask per task: a previous answer, the size of the request, or the mode of an
  earlier task never authorizes skipping the question.
- A prompt that asks for code, fixes, or a new artifact is still an unanswered
  mode question — surface it before touching files.
- SDD mode means the full cycle (branch → propose → apply → sync → archive) under
  [`docs/agent/sdd-workflow.md`](docs/agent/sdd-workflow.md); direct mode means
  editing the files in place without a change proposal.

## Project Structure

```
flashbacks/
├── webapp/                   # React 19 SPA frontend
├── backend/                  # Backend microservices
│   ├── core/          # Core Go API service
│   ├── exif/                 # EXIF metadata service
│   └── ocr/                  # OCR text detection service
├── tools/                    # Utilities (embeddings-builder)
├── docs/                     # Shared documentation & API contracts
├── scripts/                  # Host-native dev/test scripts (dev.sh, bootstrap.sh)
├── skills/                   # Shared AI agent skills
└── Makefile                  # Common commands
```

## General Coding Standards

### All Languages
- English only (comments, names, docs)
- No `any` type without justification
- Follow clean architecture principles
- Keep i18n en/ru in sync

### Go Standards
- PascalCase exported, camelCase unexported
- Explicit error handling, no panics
- No identifier redeclaration in same scope
- Use `slog` for structured JSON logging
- i18n: Use `Msg*` constants, convert: `string(i18n.MsgX)`
- Validation: `i18n.CreateValidationError(i18n.ValidationError)`
- JSON tags must match frontend TypeScript names (camelCase)
- Run unit tests after every code change

### Design System
- Before implementing any webapp UI, consult the authoritative design system spec at [`openspec/specs/design-system/spec.md`](openspec/specs/design-system/spec.md) and follow its token, component, layout, interaction, responsive, theming, and accessibility rules.
- See [`docs/design-system.md`](docs/design-system.md) for the living audit trail of known UI drift vs the spec.

### TypeScript Standards
- Strict mode, `verbatimModuleSyntax`, no unused vars
- `import type` for type-only imports
- Strict `TranslationKey` type — no arbitrary strings in `t()`
- Path alias: `@/*` → `src/*`
- Field names must match Go JSON tags exactly
- Functional React components only (no classes)

## DO NOT
- ❌ Use `any` without justification
- ❌ Redeclare Go identifiers in same scope
- ❌ Use arbitrary strings for i18n keys
- ❌ Mix `Msg*`/`Err*` (use `Msg*`)
- ❌ Skip `MessageKey` → `string` conversion
- ❌ Create React class components
- ❌ Bypass TypeScript strict checks

## MUST
- ✅ Match TS properties to Go JSON tags
- ✅ Use `import type` for type-only imports
- ✅ Handle all Go errors explicitly
- ✅ Use context providers for shared state
- ✅ Follow clean architecture
- ✅ Keep i18n en/ru in sync
- ✅ Cover new Go code with unit-tests
- ✅ Run tests after every code change
- ✅ Functional React components only
- ✅ Ask the user for the task mode (SDD cycle vs. direct) before any file-changing task

## Local Agent Contexts

Each service has its own `AGENTS.md` with service-specific details:

| Service | Agent Context |
|---|---|
| webapp | [webapp/AGENTS.md](webapp/AGENTS.md) |
| core | [backend/core/AGENTS.md](backend/core/AGENTS.md) |
| tools | [tools/AGENTS.md](tools/AGENTS.md) |
| exif | [backend/exif/AGENTS.md](backend/exif/AGENTS.md) |
| ocr | [backend/ocr/AGENTS.md](backend/ocr/AGENTS.md) |

## MCP Tools

| Server | Purpose |
|---|---|
| **filesystem** | Read, write, edit, move, and search project files |
| **github** | Manage repos, branches, PRs, issues, commits |
| **postgres** | Run read-only SQL queries against the database |
| **sequentialthinking** | Break down complex multi-step problems |
| **context7** | Up-to-date version-specific docs for external libraries |
