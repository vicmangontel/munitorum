---
type: plan
phase: 1
module: setup
status: not-started
spec: docs/specs/setup.md
depends-on: phase-0
steps: 2
current-step: 1
---

# Phase 1 — Setup (wizard)

## Goal

A fresh database opens the seven-step setup wizard full-screen. Confirming the
summary writes the coto — settings, two accounts, the houses, the initial cuota
rate, and the apertura of every house — in one transaction and opens the shell
with the coto name and the opening balances in the summary panel. A completed
database opens the shell directly. Ajustes corrects the coto name, the account
names, and the opening balances. The form and confirmation components built here
are the ones every later phase uses.

Wizard steps validate as the treasurer moves forward without the TUI duplicating
rules: `app.SetupService` gains `ValidateSetup` (same input as `CompleteSetup`,
no write), and the wizard shows the errors for the fields of the current step.

## Prerequisites

- Phase 0 implemented: shell, `view.View`, overlay host, theme, text, `domain.Money`,
  `domain.Month`, migrator, logging, `cmd/devseed` skeleton.
- Approval of migration `001_setup.sql` before it is written (`CLAUDE.md` → Agent must ask).

## Steps

| Step | Slice | Spec | Status |
|---|---|---|---|
| 1 | The wizard creates the coto — carries the spine | `docs/specs/setup.md` | not-started |
| 2 | Ajustes | `docs/specs/setup.md` | not-started |

## Step 1 — The wizard creates the coto

Spec sections: Entities, Use Cases (`GetSetupState`, `CompleteSetup`, `GetSettings`),
UI Views (Wizard 1–7), Business Rules 1–11.

### Domain and application tasks
- [ ] Migration `internal/store/migrations/001_setup.sql`: `settings`, `accounts`, `houses`, `fee_rates` with the checks and unique constraints in the spec (ask first)
- [ ] `internal/app`: `ValidationError` carrying every invalid field (field key → message, row-indexed keys for house rows); sentinel `ErrNotFound`, `ErrConflict`; `Clock` interface with the system clock
- [ ] `internal/domain`: house code (`prefix + number` zero-padded to `max(2, digits(count))`) and per-house opening rules 8–9 as pure functions, reused by houses in Phase 4
- [ ] `app.SetupService` with its repository interface declared in `app`; `GetSetupState`; `GetSettings` (settings + accounts in `sort_order`)
- [ ] `ValidateSetup` and `CompleteSetup`: rules 1–9, every invalid field at once; `CompleteSetup` rejects a completed database with `ErrConflict` and writes settings, 2 accounts, N houses, and 1 `initial` fee rate in one transaction
- [ ] `internal/store`: SQLite implementation of the setup repository
- [ ] Wide event `action=complete_setup`: `outcome`, `duration_ms`, `house_count`, `fee_amount_cents`, opening balance per account in centavos; no names
- [ ] `cmd/devseed`: run `CompleteSetup` for 18 houses with mixed opening states — up to date, two months behind, a year prepaid, never paid, opening arrears

### TUI tasks
- [ ] Overlay host accepts an interactive model (forms, confirmation), not only static lines; `esc` still closes; key-hint bar shows the overlay's keys
- [ ] Form component `internal/tui/form`: labelled text fields, `tab`/`shift+tab`, error text under each field from `app.ValidationError`, cursor to the first invalid field
- [ ] Confirmation component (`Sí` / `No`, `No` selected by default)
- [ ] Startup routing in `cmd/munitorum`: after migrations, `GetSetupState` → wizard when not completed, shell when completed; a finished wizard opens the shell without restarting
- [ ] Wizard steps 1–5 (Coto; Casas with live code preview; Cuota; Cuentas with defaults `Efectivo`, `Banco (NU)`; Apertura): `enter` next after `ValidateSetup` passes for the step, `shift+tab`/`←` back keeping values, `esc` cancels after confirmation and writes nothing
- [ ] Wizard step 6 Casas al abrir: one row per house with `Cubierto hasta`, `Crédito`, `Adeudo inicial`; `↑/↓` row, `tab` field; errors shown on their row
- [ ] Wizard step 7 Resumen: houses, cuota, balance per account and total, houses with coverage, total credit, total opening arrears; `enter` calls `CompleteSetup`; a returned error moves to the step holding the first invalid field
- [ ] Summary panel: coto name as title, one row per account with its opening balance, total (live balances arrive in Phase 3)
- [ ] All wizard and summary strings in `internal/tui/text`

### Test tasks
- [ ] `internal/domain` table tests: codes for `CASA ` ×18, `C-` ×18, count 120 (3 digits), empty prefix; opening rules (coverage before the opening month, beyond +36 months, credit ≥ cuota, arrears with coverage)
- [ ] `internal/app` tests on a migrated temp database: every Step 1 acceptance criterion; a forced failure mid-write (test trigger raising on `fee_rates` insert) leaves no rows; second `CompleteSetup` rejected; future opening date rejected
- [ ] TUI tests: form shows every error at once; wizard back keeps values; `esc` + `Sí` writes nothing; step 7 confirm opens the shell; startup routing for fresh and completed databases

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] A fresh `--db` path runs the wizard, writes the coto, and opens the shell with its name and opening balances; relaunching opens the shell directly

## Step 2 — Ajustes

Spec sections: Use Cases (`UpdateCommunityName`, `RenameAccount`, `UpdateOpeningBalance`),
UI Views (Ajustes), Business Rules 1, 5, 6.

### Domain and application tasks
- [ ] `UpdateCommunityName` (rule 1)
- [ ] `RenameAccount` (rule 5: trimmed, 1–30 characters, unique among accounts case-insensitively)
- [ ] `UpdateOpeningBalance` (rule 6: money ≥ 0)
- [ ] Wide events `update_community_name`, `rename_account` (`account_id`), `update_opening_balance` (`account_id`, old and new centavos); no names

### TUI tasks
- [ ] Ajustes overlay from the shell on a global key (`F2`), listed in the help overlay and the key-hint bar: coto name, account names, opening balances; `e` edits the selected field with the form component
- [ ] Opening balance edit asks for confirmation first (every balance changes); declining writes nothing
- [ ] Summary panel reloads after any Ajustes save

### Test tasks
- [ ] `internal/app` tests: blank and 61-character names rejected; renaming an account to the other account's name in any case rejected; negative opening balance rejected
- [ ] TUI tests: declining the opening-balance confirmation writes nothing; a saved rename shows in the summary panel

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] Renaming the coto or an account, or changing an opening balance, shows in the summary panel immediately

The acceptance criterion "changing an opening balance changes every balance derived
from it" is verified in Phase 3 Step 1, once movements exist.

## Manual Test Scenarios

1. `just run-db C:\temp\coto.db` on a missing file: wizard runs; enter 18 houses with prefix `CASA `, cuota 150.00, cash 7,500.00, bank 0; set Casa 03 with arrears 150.00; confirm → shell shows the coto name and both balances.
2. On step 6 give a house both coverage and arrears and step forward: the row error shows and the wizard stays on step 6.
3. Cancel the wizard at step 4 with `esc` + `Sí`; relaunch: the wizard starts again from step 1.
4. In Ajustes rename `Banco (NU)` to `efectivo`: rejected. Change the cash opening balance, decline, then accept: the summary changes only after accepting.

## Definition of Done
- [ ] Every step is done
- [ ] `go build ./...` and `go vet ./...` clean
- [ ] All new tests pass (`just test`)
- [ ] Works end-to-end in Windows Terminal
- [ ] `docs/specs/setup.md` up to date, including `ValidateSetup` (ask first)
- [ ] `docs/architecture.md` Test Coverage Map row for setup (ask first)
- [ ] `CLAUDE.md` Current State updated
