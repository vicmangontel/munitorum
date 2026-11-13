---
type: plan
phase: 0
module: setup
status: not-started
depends-on: none
---

# Phase 0 — Project Setup

## Goal

`munitorum.exe` starts in Windows Terminal and shows the full application shell —
summary panel, view list, main and detail panels, key-hint bar, `?` help overlay —
with placeholder views. The database next to the executable is created and
migrated on first run, an unreadable database stops with a clear message, logs go
to a daily file, and the domain money and month types are tested, including
property tests. `just test`, `just test-coverage`, and a first scoped mutation run
work.

## Prerequisites

- Barebones scaffold from `/noos-new-project` Phase 6 (module, `cmd/munitorum`,
  store with migrator, logging, minimal TUI, `justfile`).
- Go 1.26, `just`, Windows Terminal.

## Tasks

### Verify the scaffold
- [ ] `go build ./...` and `go vet ./...` pass with no output
- [ ] `just run` opens the placeholder TUI; `q` exits cleanly; `munitorum.db` and `logs/` appear next to the binary
- [ ] `just test` passes (migrator and startup tests)

### Domain types
- [ ] `internal/domain/money.go`: `Money` (`int64` centavos); `ParseMoney` (accepts `1234`, `1234.5`, `1,234.50`, `$1,234.50`; rejects >2 decimals, letters, empty); `Format` → `$1,234.50`, negative `−$350.00`
- [ ] `internal/domain/month.go`: `Month` (year, month); `ParseMonth`/`String` (`YYYY-MM`); `Add(n)`; `Before`; `Of(date)`; `LastDay`; `ClampDay(d)`
- [ ] Table-driven tests for both; property tests (`pgregory.net/rapid`): `ParseMoney(Format(m)) == m`; `Month.Add(a).Add(b) == Month.Add(a+b)`

### Spanish text and formatting
- [ ] `internal/tui/text`: package with every shell string (view names, key hints, help, errors at startup); Spanish month names and abbreviations
- [ ] Date and month display helpers: `12 nov 2026`, `nov 2026`

### Theme
- [ ] `internal/tui/theme`: `AdaptiveColor` tokens `Accent`, `Muted`, `Positive`, `Negative`, `Warning` with the hex values from `docs/architecture.md` → Design System; panel styles (focused / unfocused border), title, key-hint, selected-row, amount styles
- [ ] No colour literals anywhere else (`grep` check recorded in Done)

### Application shell
- [ ] Root model: layout (left column: summary panel over view list; right: main panel over detail panel; bottom: key-hint bar), resizes with the terminal, minimum size message below 80×24
- [ ] Focus cycling (`tab` / `shift+tab`, `1`–`8` jump to view), view list navigation (`↑/↓`, `enter`)
- [ ] View interface (`Init`, `Update`, `View`, `KeyHints`, `Title`) and eight placeholder views: Resumen, Casas, Movimientos, Cooperaciones, Obras, Recordatorios, Directorio, Pendientes
- [ ] Overlay host (centred box over the shell, `esc` closes) and `?` help overlay listing global keys and the focused view's keys
- [ ] Summary panel placeholder showing the coto title slot and account rows (static until Phase 1)

### Startup and infrastructure
- [ ] Database path: executable folder by default, `--db` flag; error screen (not the wizard, not a panic) when the file exists but cannot be opened or migrated — path and error shown, any key exits
- [ ] Logging: `log/slog` JSON to `logs/munitorum-YYYY-MM-DD.log`; delete files older than 30 days at startup; one wide event for startup (`action=startup`, `outcome`, `duration_ms`, `db_file` (base name only), `migrations_applied`)
- [ ] `cmd/devseed` skeleton and `just reset-dev`: recreates `dev/munitorum-dev.db` and runs migrations (data sections added by each later phase)

### Test tooling
- [ ] TUI model tests: send key messages to the root model; assert focus moves, view switches, help overlay opens and closes, too-small message renders
- [ ] `just test-coverage` writes `artifacts/coverage/coverage.out` and `artifacts/coverage/index.html`
- [ ] Gremlins added as a Go tool dependency; pilot `just test-mutation ./internal/domain`; record result, survivors investigated, and any blocker under `docs/architecture.md` → Mutation testing

## Manual Test Scenarios

1. Delete `munitorum.db`, run the binary: the shell opens; the file is recreated; the log file records the startup event.
2. Replace `munitorum.db` with a text file, run: the error screen shows the path and the error; the file is untouched.
3. Run with `--db C:\temp\prueba.db`: the database is created there.
4. Resize the terminal below 80×24 and back: the size message appears and the shell returns; `?` shows help; `1`–`8` switch views.

## Definition of Done

- [ ] `go build ./...` and `go vet ./...` clean
- [ ] `just test` passes, including property tests
- [ ] Coverage report generated; mutation pilot result recorded in `docs/architecture.md`
- [ ] No colour literals outside `internal/tui/theme`; no user-visible string literals outside `internal/tui/text`
- [ ] Shell works in Windows Terminal (dark and light profiles)
- [ ] `docs/architecture.md` Test Coverage Map has a row for the shell and domain types
- [ ] `CLAUDE.md` Current State updated
