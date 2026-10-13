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
├── webapp/            # React 19 SPA frontend
├── backend/           # Backend microservices
│   ├── core/          # Core Go API service
│   ├── exif/          # EXIF metadata service
│   └── ocr/           # OCR text detection service
├── tools/             # Utilities (embeddings-builder)
├── docs/              # Shared documentation & API contracts
├── scripts/           # Host-native dev/test scripts (dev.sh, bootstrap.sh)
└── Makefile           # Common commands
```

## General Coding Standards

These rules apply to the whole repository, regardless of language or component.

- English only (comments, names, docs)
- No `any` type without justification
- Follow clean architecture principles
- Keep i18n en/ru in sync — never ship one locale only
- Keep API field names aligned across the stack (Go JSON tags ↔ webapp TypeScript)
- Run the component's full check suite after every code change

## Component Requirements — read before touching component code

This file holds **only** project-wide rules. Component-specific requirements live
in `docs/agent/` and are mandatory reading:

- **Before changing code of a component, read that component's requirements
  document** and follow it. For that component its rules are authoritative and
  override this file wherever they differ.
- When a change spans components (e.g. renaming a Go JSON tag that the webapp
  consumes), read **every** affected component's document.
- New component-specific rules belong in the component document, not here.
- Component documents are part of the change: update them in the same commit as
  the code they describe.

| Component | Paths | Requirements document |
|---|---|---|
| Go services (core, exif, ocr, shared) | `backend/` | [`docs/agent/go-services.md`](docs/agent/go-services.md) |
| Go tooling (CLI utilities) | `tools/` | [`docs/agent/go-services.md`](docs/agent/go-services.md) |
| Webapp (React + TypeScript) | `webapp/` | [`docs/agent/webapp.md`](docs/agent/webapp.md) |
| Any file-changing task (process) | — | [`docs/agent/sdd-workflow.md`](docs/agent/sdd-workflow.md) |

Before implementing any webapp UI, also consult the authoritative design system
spec at [`openspec/specs/design-system/spec.md`](openspec/specs/design-system/spec.md)
and its drift audit at [`docs/design-system.md`](docs/design-system.md).

## DO NOT
- ❌ Use `any` without justification
- ❌ Use arbitrary strings for i18n keys
- ❌ Ship a change with only one locale (en/ru) updated
- ❌ Break the Go JSON tag ↔ TypeScript field mapping
- ❌ Skip the per-component checks listed in `docs/agent/*.md`
- ❌ Start file-changing work before the task mode is confirmed
- ❌ Bypass TypeScript strict checks or Go `vet`

## MUST
- ✅ Ask the user for the task mode (SDD cycle vs. direct) before any file-changing task
- ✅ Read the component requirements document before changing that component
- ✅ Follow clean architecture
- ✅ Keep i18n en/ru in sync
- ✅ Cover new code with unit tests
- ✅ Run the component's tests and linters after every code change
- ✅ Match TS properties to Go JSON tags
- ✅ Update the component requirements document when its rules change

## Local Agent Contexts

Some components ship their own `AGENTS.md` with local detail. Read it **together
with** the matching requirements document:

| Component | Local context | Component requirements |
|---|---|---|
| webapp | [webapp/AGENTS.md](webapp/AGENTS.md) | [docs/agent/webapp.md](docs/agent/webapp.md) |
| core | [backend/core/AGENTS.md](backend/core/AGENTS.md) | [docs/agent/go-services.md](docs/agent/go-services.md) |
| ocr | [backend/ocr/AGENTS.md](backend/ocr/AGENTS.md) | [docs/agent/go-services.md](docs/agent/go-services.md) |
| exif | — (no local file) | [docs/agent/go-services.md](docs/agent/go-services.md) |
| tools | — (no local file) | [docs/agent/go-services.md](docs/agent/go-services.md) |

## MCP Tools

| Server | Purpose |
|---|---|
| **filesystem** | Read, write, edit, move, and search project files |
| **github** | Manage repos, branches, PRs, issues, commits |
| **postgres** | Run read-only SQL queries against the database |
| **sequentialthinking** | Break down complex multi-step problems |
| **context7** | Up-to-date version-specific docs for external libraries |
