# Munitorum — Architecture

> Single source of truth for all architecture and design decisions.
> Read this before making any structural change. Update it when any decision changes.

## Project Context
**Goal:** A local terminal application for the treasurer of a residential coto to
record household cuotas, special assessments, expenses, transfers and cash
adjustments, see live balances (available vs adelantos vs assessment funds) and
per-casa debt, and export monthly Excel reports.
**Target users:** One treasurer (tesorero), sole operator.
**Deployment:** One Windows computer, Windows Terminal, single executable, offline.
**Peak concurrent users:** 1.

**Stack:** Go 1.26 · Bubble Tea + Bubbles + Lip Gloss (TUI) · SQLite through
`modernc.org/sqlite` (pure Go, no CGO) · excelize v2 (Excel export) ·
`log/slog` (logging) · `go test` + `pgregory.net/rapid` (tests).
Exact module versions are pinned in `go.mod` during Phase 0.

This project departs from the Noosphere default stack (.NET backend + Vue
frontend over HTTP). There is no HTTP layer: the TUI calls application services
in-process. Conventions below are written for this stack.

### Layout

```
cmd/munitorum/        main: flags (--db), open store, migrate, start TUI
internal/domain/      entities and pure rules — money, months, allocation, balances
                      imports nothing from store, tui, or export
internal/app/         use cases, one exported method per operation, grouped by module;
                      repository interfaces are declared here
internal/store/       SQLite implementation of the app repositories; embedded migrations
internal/tui/         Bubble Tea models, one package per view; theme/ and text/ subpackages
internal/export/      Excel workbook generation
cmd/devseed/          dev fixtures loader (never shipped in a release build)
```

Dependency direction: `tui`, `export` → `app` → `domain`; `store` → `app` (implements
interfaces), `domain`. `cmd/munitorum` wires everything.

## Architecture Decisions

### Tenancy (Q1)
**Choice:** Single-tenant
**Means for this project:** One coto per database. No tenant identifiers anywhere.
The coto name is a row in `settings`.

### Database engine (Q2)
**Choice:** SQLite (`modernc.org/sqlite`)
**Means for this project:** Database file `munitorum.db` in the same folder as the
executable; `--db <path>` overrides it. One `*sql.DB` with `SetMaxOpenConns(1)`.
Every connection runs `PRAGMA foreign_keys = ON` and `PRAGMA busy_timeout = 5000`.
Journal mode stays `DELETE` (not WAL) so the single `.db` file is always a complete
copy for manual backup. New IDs come from `RETURNING id`.

### Language support (Q3)
**Choice:** Spanish UI only; English code
**Means for this project:** No i18n framework. Every user-visible string — labels,
messages, help, Excel headers — lives in `internal/tui/text` (and
`internal/export` uses the same package). Identifiers, entity names, DB tables and
columns, log fields, and comments are English. The glossary below maps them.

### Timestamp convention (Q4)
**Choice:** Local time
**Rule:** Use `time.Now()` (local) for every timestamp; never `time.Now().UTC()`.
Business dates (movement date, apertura date) are date-only `TEXT 'YYYY-MM-DD'`.
Months are `TEXT 'YYYY-MM'` and the `domain.Month` type. Audit timestamps
(`created_at`, `updated_at`, `deleted_at`) are `TEXT 'YYYY-MM-DD HH:MM:SS'` local.

### Hosting and access model (Q5)
**Choice:** A — Local / single user
**Means for this project:** No authentication, no users table, no sessions, no
network listener. Whoever runs the executable is the treasurer.

### Database isolation (Q6)
**Choice:** Single DB
**Means for this project:** One file, one connection, migrations applied once at startup.

### Delete strategy (Q7)
**Choice:** Soft delete
**Means for this project:** `deleted_at TEXT NULL` on main entities; a row is live
when `deleted_at IS NULL`. Every list and get query filters on it. Delete sets
the column; it never issues `DELETE`. Derived values (balances, allocation,
fund remaining) ignore deleted rows.
**Applies to:** movements, house contacts, vendors, assessments, projects,
reminders, todos. Not to: settings, accounts, houses (fixed at setup), categories
(deactivated with `active = 0` instead), fee rates, reminder occurrences,
schema migrations.

### Background jobs (Q8)
**Choice:** No
**Means for this project:** No goroutines doing timed work. Due reminders, debt,
and dashboard figures are computed when a view loads or after a save.

### Monetary data (Q9)
**Choice:** Yes — MXN only
**Means for this project:** Amounts are `INTEGER` centavos in the database and
`domain.Money` (`int64` centavos) in Go. Never `float64` for money, never `REAL`.
Parsing accepts `1234`, `1234.5`, `1,234.50`, `$1,234.50`, with an optional leading
`-` or `−` (U+2212); display is `$1,234.50`, negatives `−$350.00`. `ParseMoney` accepts
negatives so every formatted value parses back; forms that require positive amounts
check that in their own validation.
Rounding never happens implicitly; parsing more than 2 decimals is a validation error.

### Logging destination (Q10)
**Choice:** File only
**Means for this project:** `log/slog` JSON handler writing to
`logs/munitorum-YYYY-MM-DD.log` beside the database. Files older than 30 days are
removed at startup. Nothing is written to stdout/stderr while the TUI runs.
One wide event per user action (see Active Conventions → Logging).

### Seed data (QS)
The ledger migration (`003_ledger.sql`) inserts the default categories:
income `Cuota` (system), `Extraordinaria` (system), `Otros ingresos`;
expense `Insumos`, `Servicios`, `Mtto preventivo`, `Reparaciones`.
Accounts, houses, rates, and opening balances come from the setup wizard.
No admin user (no auth).

### Dev fixtures (QF)
**Choice:** Yes — `cmd/devseed` + `just reset-dev`
Deletes and recreates `dev/munitorum-dev.db` (never the real database), runs
migrations, and inserts a sample coto: 18 houses with mixed states (up to date,
2 months behind, a year prepaid, never paid), 6 months of movements across both
accounts, one partly collected assessment, one project with an advance, four
reminders including a bimonthly one, two todos. Update the loader whenever a
table is added.

## Domain Glossary

| UI (Spanish) | Code (English) | Table |
|---|---|---|
| Coto | `Settings.CommunityName` | `settings` |
| Casa | `House` | `houses` |
| Contacto de casa | `HouseContact` | `house_contacts` |
| Cuota | `Fee`, rate history `FeeRate` | `fee_rates` |
| Pago de cuota | `FeePayment` (movement kind) | `movements` |
| Mes cubierto / parcial / adeudado | `MonthStatus` covered / partial / owed | derived |
| Adelanto | `Prepaid` | derived |
| Adeudo | `Arrears`; adeudo inicial `OpeningArrears` | `houses.opening_arrears_cents` |
| Cooperación extraordinaria | `Assessment` | `assessments` |
| Obra | `Project` | `projects` |
| Anticipo | project payment (expense linked to a project) | `movements` |
| Apertura | `Opening` | `settings.opening_date`, `accounts.opening_balance_cents`, `houses.opening_*` |
| Cuenta | `Account` | `accounts` |
| Movimiento / Libro de caja | `Movement` / `Ledger` | `movements` |
| Transferencia | `Transfer` (movement kind) | `movements` |
| Arqueo / Ajuste | `CashCount` / `Adjustment` (movement kind) | `movements` |
| Categoría | `Category` | `categories` |
| Directorio | `Vendor` (kind person / company / service) | `vendors` |
| Recordatorio | `Reminder`, per-month state `ReminderOccurrence` | `reminders`, `reminder_occurrences` |
| Pendiente | `Todo` | `todos` |
| Recibo manual | `ManualReceipt` status none / pending / done | `movements.manual_receipt` |
| Estado de cuenta | `HouseStatement` | derived |
| Disponible | `Available` | derived |

Table shapes are defined in the module specs.

## Design System

### Visual style (Q11)
**Choice:** 12 — Dense operational, adapted to the terminal
**Character:** lazygit-like: terminal's own background, thin panel borders, focused
panel in the accent colour, compact tables, key hints always visible.

### Usage pattern (Q12)
**Choice:** B — Short-session reference and recording
**Implication:** The treasurer uses it a few minutes several times a month and will
not memorise keys. Bottom bar always shows the keys valid in the focused panel;
`?` opens a full help overlay; forms have labelled fields and show all validation
errors at once.

### Layout
Left column: summary panel (coto name, balance per account, total, disponible) above
the view list (Resumen, Casas, Movimientos, Cooperaciones, Obras, Recordatorios,
Directorio, Pendientes — with counts for due/pending items). Right: main panel
(list of the selected view) above a detail panel (selected row). Bottom: key-hint
bar. Forms and confirmations open as centred overlays.

### Colour palette (Q13)
No background or default foreground is painted; the terminal's own are used. Every
colour is a Lip Gloss `AdaptiveColor` (dark / light terminal).

| Token | Dark | Light | Usage |
|---|---|---|---|
| `Accent` | `#58a6ff` | `#0969da` | Focused panel border, selected row, titles |
| `Muted` | `#8b949e` | `#57606a` | Labels, key hints, inactive borders |
| `Positive` | `#3fb950` | `#1a7f37` | Income amounts, covered month ✓, success messages |
| `Negative` | `#f85149` | `#cf222e` | Expense amounts, owed months ✗, errors |
| `Warning` | `#d29922` | `#9a6700` | Partial months, due reminders, negative-balance warning |

Semantic tokens (`Positive`, `Negative`, `Warning`) keep their meaning everywhere and
are never reused for decoration.

**Typography:** terminal font; bold for titles and totals only.
**Formats:** money `$1,234.50`; dates `12 nov 2026`; months `nov 2026` (Spanish, lower case).
**Dark / light default:** adaptive — follows the terminal.
**Component density:** compact (one line per row, no blank lines between rows).
**Palette source:** suggested palette for Dense operational adapted to terminal
adaptive colours, agreed 2026-11-13.

### Thematic character (Q13b)
**Domain:** residential community treasury (coto / fraccionamiento), Mexico.

| # | Element | Where it applies | Where it must not |
|---|---|---|---|
| 1 | Domain vocabulary (Casa, Cuota, Adelanto, Adeudo, Cooperación, Obra, Anticipo, Arqueo, Libro de caja, Recibo manual, Pendientes) | Every screen, help, and Excel sheet | Code identifiers (English, per glossary) |
| 2 | Coto name from setup | Summary panel title | Tables, forms |
| 3 | Report header: coto name, month, generation date | Top rows of every Excel sheet | Data rows |

**Untouched by theming:** semantic colours, focus indication, validation messages,
table density, numeric columns.

### Styling approach (Q14)
**Choice:** Lip Gloss styles defined once in `internal/tui/theme`
**Reason:** Terminal UI has no CSS. One theme package keeps every colour and border in
one place and lets views stay free of colour literals.
**Setup:** See `docs/plans/phase-0-setup.md`.

## Active Conventions

### Data layer
- Money: `INTEGER` centavos, columns suffixed `_cents`. `domain.Money` in Go.
- Soft delete: `deleted_at TEXT NULL` on the entities listed under Q7; every list and
  get filters `deleted_at IS NULL`; delete sets the column.
- Timestamps: local time via `time.Now()`; formats per Q4.
- Column naming: `snake_case`; table names plural `snake_case`.
- Enums: `TEXT` with `CHECK (col IN (...))`.
- Primary keys: `id INTEGER PRIMARY KEY`.
- Derived values are never stored: account balances, month allocation, prepaid,
  arrears, fund remaining, and available are computed from movements by `internal/domain`.
- Migrations: SQL files embedded from `internal/store/migrations/NNN_description.sql`,
  applied in filename order at startup, each in its own transaction, recorded in
  `schema_migrations`. Applied files are never edited; a change is a new file.
- Every multi-row write (a payment plus its notes, a wizard run, a transfer) runs in one
  transaction.
- The database path is passed to the driver as a plain filename, never a `file:` URI,
  so `#` and `%` in folder names are literal. A path containing `?` is rejected at open
  with an error (the driver splits its DSN at `?`).

### Application layer
- One exported method per operation, verb + noun: `RecordFeePayment`, `ListHouses`.
- Inputs and outputs are plain structs in `internal/app`; the TUI never touches `store`
  or SQL.
- Validation lives in `internal/app` (shape) and `internal/domain` (rules). An
  operation returns `app.ValidationError` listing **every** invalid field, so a form
  can show all errors at once; the TUI does not duplicate the rules.
- Other failures are returned as wrapped `error`s with `%w`; sentinel errors
  (`app.ErrNotFound`, `app.ErrConflict`) for cases the UI must distinguish.
- No panics across package boundaries.

### TUI layer
- One Bubble Tea model per view in `internal/tui/<view>`; the root model owns layout,
  focus, overlays, and the key-hint bar.
- Views implement `view.View` (`internal/tui/view`: `Init`, `Update`, `View`,
  `KeyHints`, `Title`) so view packages never import the root. Until a view's phase
  builds it, the root shows a shared `view.Placeholder`; that phase replaces it with
  its own package.
- Views call `internal/app` through commands (`tea.Cmd`), never directly in `View()`.
- No colour or border literals outside `internal/tui/theme`; no user-visible string
  literals outside `internal/tui/text`.
- Every action reachable by keyboard; the key-hint bar shows the keys for the focused panel.

### Logging
- One wide event per user action, emitted when the action finishes: `action`,
  `outcome` (`ok` / `validation` / `error`), `duration_ms`, the entity ids touched,
  and amounts in centavos where relevant.
- Levels: `Info` for success, `Warn` for validation failures, `Error` for failures.
- Never log names, phones, emails, notes, or free-text descriptions. Ids only.
- Single-user app: no `UserId` field.
- Startup emits one event: `action=startup`, `outcome` (`ok` / `error`), `duration_ms`,
  `db_file` (base name only, never the full path), `migrations_applied`, and `error` on
  failure. A failed prune of old log files emits `action=log_prune`, `outcome=error`,
  `error` at `Warn` and does not stop startup.
- The setup wizard emits `action=complete_setup`: `outcome`, `duration_ms`,
  `house_count`, `fee_amount_cents`, `opening_balance_cash_cents`,
  `opening_balance_bank_cents`, and `invalid_field_count` on a validation failure.
- Ajustes emits `action=update_community_name`, `action=rename_account` (`account_id`),
  and `action=update_opening_balance` (`account_id`, `old_balance_cents`,
  `new_balance_cents`), each with `outcome`, `duration_ms`, `invalid_field_count` on a
  validation failure, and `error` on failure. No names. `ErrNotFound` is outcome
  `error` at `Error`.

## Testing Conventions

Follow `doctrine/testing.md` for scope, doubles, assertions, grouping, determinism,
property-based testing, and mutation testing.

- Tests are `*_test.go` beside the package they test. Group input variations of one
  rule as table-driven subtests (`t.Run`), even with two cases.
- `internal/domain`: example tests and property tests for allocation, balances,
  rates, funds, available. Highest value; most tests live here.
- `internal/app`: behaviour tests through the real SQLite store on a fresh temporary
  database with migrations applied (`t.TempDir()`). No mocked repositories for SQL
  behaviour. Replace only the clock (`app.Clock` interface) for determinism.
- `internal/store`: covered through `internal/app` tests; add direct tests only for
  migration and query behaviour not reachable otherwise.
- `internal/tui`: model tests that send key messages to `Update` and assert visible
  output of `View()` or emitted commands, for forms with validation, navigation, and
  confirmation flows. No snapshot test per view.
- `internal/export`: generate a workbook into a temp folder, reopen it with excelize,
  and assert sheet names and key cells.
- Assert outcomes; do not assert calls.
- **Property tests:** `pgregory.net/rapid`, files `*_property_test.go` beside the
  package's other tests; part of the ordinary `just test`. First business rule:
  fee allocation — for any sequence of payments and rates, the amounts applied to
  months plus remaining credit equal the total paid, and months are filled in order.
- **Commands:** `just test` runs `go test ./...` (all tests, including property tests).
  `just test-coverage` writes `artifacts/coverage/coverage.out` and an HTML report at
  `artifacts/coverage/index.html`; diagnostic, not a gate.

### Mutation testing
**Tool:** Gremlins (`github.com/go-gremlins/gremlins`), pinned as a Go tool dependency
(`go get -tool`) in Phase 0 — v0.6.0. **Status:** working on Windows. Command:
`just test-mutation <package path>`, which runs
`go tool gremlins unleash <path> --timeout-coefficient 20`; the default coefficient
marks most mutants as timed out because each mutant is recompiled, and timed-out
mutants are excluded from efficacy. Exclusions: `cmd/`, `internal/tui/text`. Cadence:
run on changed business rules in `internal/domain` and `internal/app` before closing a
phase. No repository-wide score gate.
**Pilot (Phase 0, 2026-11-13):** `just test-mutation ./internal/domain` (money and
month types with their property tests), default mutators — 57 killed, 0 lived, 0 not
covered, 0 timed out; efficacy 100%, mutator coverage 100%; identical over three
runs, about 16 s. No survivors to investigate. Next scoped run: the fee allocation rule.
**Phase 1 Step 1 (2026-11-14):** `./internal/app` — 37 killed, 0 lived, 2 not covered,
efficacy 100% (two survivors at the one-character-name boundary were killed by added
tests). `./internal/domain` — 69 killed, 0 lived, 9 not covered, efficacy 100%; the 9
not-covered mutants are all in the `HouseOpening.Check` switch, which `go tool cover`
reports at 100%, so they are treated as a Gremlins coverage-mapping artifact.
**Phase 1 Step 2 (2026-11-14):** `./internal/app` — 42 killed, 0 lived, 5 not covered,
efficacy 100%. Not covered: the `GetSettings` error branch (`setup.go`), one line in
`errors.go`, and the `parseBalance` switch (`setup_validate.go`), whose negative and
unreadable cases are tested; treated as the same coverage-mapping artifact, not confirmed.

## Known Deviations

None.

## Test Coverage Map

One row per module, updated as each phase completes. Percentages are statement
coverage from `just test-coverage`.

| Module | Coverage | Uncovered behavior |
|---|---|---|
| Domain (`internal/domain`: money, month, house code, opening rules, date/month input) | tested — 98.9%; mutation 69 killed, 0 lived | One `ParseMoney` branch. Allocation, balances, and funds are not built yet |
| Setup service (`internal/app`: `SetupService` incl. Ajustes operations, `ValidationError`, clock) | tested — 93.2%; mutation 42 killed, 0 lived | `GetSettings` error branch |
| TUI shell (`internal/tui`: root, layout, overlay, wizard-to-shell swap, startup error, F2 Ajustes) | tested — 96.6% | `Init` methods; right-edge guard in overlay placement; real rendering and colours in Windows Terminal (manual only) |
| Setup wizard (`internal/tui/wizard`) | tested — 93.0% | Interactive end-to-end run in Windows Terminal (manual only) |
| Ajustes (`internal/tui/settings`) | tested — 96.7% | Real rendering in Windows Terminal (manual only) |
| Form (`internal/tui/form`) | tested — 83.9% | Rarely used cursor paths |
| Confirmation (`internal/tui/confirm`) | partial — 58.8% in-package | Hosted in the Ajustes overlay and tested through `internal/tui/settings`; in-package tests are unit-only |
| Text (`internal/tui/text`) | partial — 26.7% | Strings mostly exercised through view tests in other packages; a test checks every `app` validation code has a message; excluded from mutation |
| Logging (`internal/logging`) | tested — 88.6% | Rare OS errors such as the log file failing to open |
| Startup (`cmd/munitorum`) | partial — 40.0% | `openDatabase` and startup routing (fresh and completed databases) tested; `main`, `run`, and default path resolution are interactive wiring, untested |
| Store (`internal/store`) | partial — 36.4% in-package | Setup repository, including the Ajustes `UPDATE` methods, is covered through `internal/app` tests, which per-package coverage does not count; migration failure and rollback paths untested |
| Dev fixtures (`cmd/devseed`) | partial — 66.7% | `main` wiring; recreate, migrate, and the 18-house `CompleteSetup` seed tested |

## In-flight Work

None.

## Acknowledged Risks

| Risk | Reason accepted | Mitigation |
|---|---|---|
| Reminders are only seen while the app is open; nothing notifies when it is closed | Interactive single-user tool; the treasurer opens it to record payments | Dashboard lists every due and overdue reminder until done or skipped |
| Spanish-only UI makes a later open-source release harder | v1 is for one coto | All user-visible strings in `internal/tui/text`; code and data model in English |
| Manual backup only | Treasurer accepted copying the file | Journal mode `DELETE` keeps the `.db` a complete copy; in-app backup listed for v2 |
