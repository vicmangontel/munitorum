# Munitorum — CLAUDE.md

## Project Overview
**Goal:** Local terminal app for the treasurer of a residential coto to record cuotas,
cooperaciones, expenses, transfers and cash counts; see balances (available vs
adelantos vs funds) and per-casa debt; and export monthly Excel reports.
**Target users:** One treasurer (tesorero), sole operator, one Windows computer.
**Status:** Phase 0 complete — application shell, theme, text, money and month types,
test tooling. See `docs/roadmap.md` for the implementation order.

## Architecture and Design
All architecture decisions, design system choices, active conventions, and acknowledged
risks are documented in [`docs/architecture.md`](docs/architecture.md).

**Read `docs/architecture.md` before making any structural change.**
Update it whenever a decision changes — then update this file's Current State section.

Key decisions at a glance (see `docs/architecture.md` for full detail):
- Stack: Go · Bubble Tea/Bubbles/Lip Gloss · SQLite (`modernc.org/sqlite`) · excelize · slog
- Tenancy: single coto · DB: one SQLite file next to the exe (`--db` overrides) · Timestamps: local time
- Hosting: local, single user, no auth · Delete: soft (`deleted_at`) · Background jobs: none
- Monetary: INTEGER centavos, MXN only · Style: Dense operational (lazygit-like), terminal-adaptive colours
- UI language: Spanish only · Code, entities, tables, logs: English · Theme: coto domain vocabulary

## Noosphere

- **Context economy:** on

## Context loading

Read `docs/index.md` for the full document navigator. Quick reference:

| Task | Read |
|---|---|
| Any task | `CLAUDE.md` + `docs/index.md` |
| Architecture or conventions | `docs/architecture.md` |
| Working on a module | `docs/specs/{module}.md` + its plan file |
| Spanish term → code name | `docs/architecture.md` → Domain Glossary |
| Adding a DB migration | `docs/architecture.md` → Active Conventions → Data layer |
| Checking what's built | `CLAUDE.md` → Current State below |

## Directory Structure
```
cmd/
├── munitorum/           main: flags, open store, migrate, start TUI
└── devseed/             dev fixtures loader (Phase 0)
internal/
├── domain/              entities and pure rules — money, months, allocation, balances
├── app/                 use cases, one exported method per operation; repository interfaces
├── store/               SQLite repositories; migrations/ embedded NNN_description.sql
├── tui/                 Bubble Tea root model; one package per view
│   ├── theme/           all colours and borders (Phase 0)
│   └── text/            all user-visible Spanish strings
├── export/              Excel workbook writer
└── logging/             slog JSON daily file
docs/
├── prd.md · architecture.md · roadmap.md · index.md
├── specs/               one file per module
└── plans/               one file per implementation phase
```

## Naming Conventions
**Backend language:** Go
**Module path:** `github.com/vicmangontel/munitorum`
**Packages:** lower-case, one word, English — including view packages (`tui/houses`, not `tui/casas`)
**DB:** tables plural `snake_case`, columns `snake_case`, money columns suffixed `_cents`
**Identifiers:** English, per the Domain Glossary; Spanish only inside `internal/tui/text`
**Files:** `snake_case.go`; tests `*_test.go`; property tests `*_property_test.go`

## Development Rules

### Agent must ask before:
- Creating or modifying any SQL migration in `internal/store/migrations/`
- Creating or modifying any file in `docs/`
- Adding new top-level packages, commands, or modules

### Agent must follow:
- One exported method per operation in `internal/app` — verb + noun (`RecordFeePayment`, `ListHouses`)
- `internal/domain` imports nothing from `store`, `tui`, or `export`
- The TUI calls `internal/app` only — never `store` or SQL
- Validation in `internal/app`/`internal/domain`; operations return `app.ValidationError` with
  **every** invalid field so forms show all errors at once; the TUI does not duplicate rules
- Errors wrapped with `%w`; no panics across package boundaries
- Derived values (balances, month allocation, prepaid, arrears, funds, available) are computed,
  never stored
- Soft delete: `deleted_at`; every list/get filters `deleted_at IS NULL`; never issue `DELETE`
  on soft-deleted entities
- All monetary amounts are `domain.Money` / INTEGER centavos — never `float64`, never `REAL`
- Use `time.Now()` (local) everywhere — never `time.Now().UTC()`
- Multi-row writes run in one transaction
- No colour literals outside `internal/tui/theme`; no user-visible strings outside `internal/tui/text`
- Logging: one wide event per user action (`action`, `outcome`, `duration_ms`, ids, amounts in
  centavos); never log names, phones, emails, notes, or descriptions
- Nothing writes to stdout/stderr while the TUI runs

### Commands
| Command | Does |
|---|---|
| `just build` | `bin/munitorum.exe` |
| `just run` | build and run against `bin/munitorum.db` |
| `just run-db <path>` | run against another database |
| `just test` | all tests, including property tests |
| `just vet` | `go vet ./...` |
| `just test-coverage` | `artifacts/coverage/index.html` |
| `just test-mutation <pkg>` | Gremlins on a package (added in Phase 0) |
| `just reset-dev` | recreate `dev/munitorum-dev.db` with fixtures (added in Phase 0) |

### Documentation to keep in sync:
| Document | Update when... |
|---|---|
| `docs/index.md` | Adding or renaming any spec or plan file, or changing a phase status |
| `docs/roadmap.md` | A phase plan is written (Plan cell) or the sequence changes |
| `docs/specs/{module}.md` | Adding or changing a use case, operation, entity, or business rule |
| `docs/architecture.md` | Any architecture or design decision changes; Test Coverage Map after each phase |
| `CLAUDE.md` current state | After each completed phase |
| `cmd/devseed` | Adding a new table — add matching fixture data |

## Current State
- [x] PRD: `docs/prd.md`
- [x] Architecture decisions recorded: `docs/architecture.md`
- [x] Module specs: `docs/specs/setup.md`, `directory.md`, `ledger.md`, `houses.md`, `assessments.md`, `reminders.md`, `dashboard.md`, `reports.md`
- [x] Roadmap: `docs/roadmap.md`
- [x] Implementation plans written: `docs/plans/phase-0-setup.md`, `phase-1-setup.md`, `phase-2-directory.md`, `phase-3-ledger.md`, `phase-4-houses.md`
- [ ] Plans still to write: 5 Assessments, 6 Reminders, 7 Dashboard, 8 Reports
- [x] Barebones scaffold created (module, migrator, logging, placeholder TUI, justfile)
- [x] Phase 0 — application shell, theme, text, domain money/month types, test tooling (manual Windows Terminal checks deferred to Phase 3 Step 1)
- [ ] Phase 1 — setup wizard, Ajustes, form and confirmation components: implemented and reviewed; manual Windows Terminal verification pending (`docs/plans/phase-1-setup.md` → Manual Test Scenarios)
- [ ] **Next:** `docs/plans/phase-2-directory.md` — Directory
