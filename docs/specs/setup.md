---
type: spec
module: setup
status: draft
entities: Settings, Account, House, FeeRate
operations: GetSetupState, ValidateSetup, CompleteSetup, GetSettings, UpdateCommunityName, RenameAccount, UpdateOpeningBalance
depends-on: none
---

# Setup — Specification

## Purpose

Creates a usable coto from an empty database in one guided run: the coto name,
the houses and their codes, the starting cuota, the two accounts, and the
apertura — the opening balance per account and what each house had already
paid when the treasurer took over. It also owns the few settings that may be
corrected afterwards. Every other module assumes setup has completed.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Settings` (`settings`, single row `id = 1`) | `community_name TEXT NOT NULL`, `house_prefix TEXT NOT NULL`, `house_count INTEGER NOT NULL`, `opening_date TEXT NOT NULL` (`YYYY-MM-DD`), `setup_completed_at TEXT NOT NULL`, `updated_at` | Row exists only after a completed wizard. `CHECK (id = 1)`. Opening month = month of `opening_date`. |
| `Account` (`accounts`) | `id`, `name TEXT NOT NULL UNIQUE`, `kind TEXT CHECK (kind IN ('cash','bank'))`, `opening_balance_cents INTEGER NOT NULL CHECK (>= 0)`, `sort_order INTEGER`, `created_at`, `updated_at` | Exactly two rows in v1: one `cash`, one `bank`. Not soft-deleted. |
| `House` (`houses`) | `id`, `number INTEGER NOT NULL UNIQUE`, `code TEXT NOT NULL UNIQUE`, `opening_covered_through TEXT NULL` (`YYYY-MM`), `opening_credit_cents INTEGER NOT NULL DEFAULT 0`, `opening_arrears_cents INTEGER NOT NULL DEFAULT 0`, `created_at`, `updated_at` | Fixed set created by the wizard. Opening fields are edited later from the houses module. Not soft-deleted. |
| `FeeRate` (`fee_rates`) | `id`, `effective_month TEXT NOT NULL UNIQUE` (`YYYY-MM`), `amount_cents INTEGER NOT NULL CHECK (> 0)`, `policy TEXT CHECK (policy IN ('initial','honor_prepaid','charge_difference'))`, `recorded_at TEXT NOT NULL` | Setup writes the one `initial` row with `effective_month` = opening month. Later rows belong to the houses module. |

Migration: `internal/store/migrations/001_setup.sql` creates the four tables above.
The migrator itself creates `schema_migrations` (Phase 0).

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `GetSetupState` | System at startup | — | Returns `completed` when the settings row exists | `{Completed bool}` |
| `ValidateSetup` | Treasurer (wizard, each step) | Same `SetupInput` as `CompleteSetup` | Rules 1–9. Writes nothing. | Every invalid field, or the `SetupSummary` shown on step 7 when valid |
| `CompleteSetup` | Treasurer (wizard) | `SetupInput`: `CommunityName`, `HouseCount`, `HousePrefix`, `FeeAmount`, `Accounts[2]{Name, OpeningBalance}`, `OpeningDate`, `Houses[]{CoveredThrough, Credit, Arrears}` | Rules 1–9. Rejected with `ErrConflict` when setup already completed. One transaction. | Created settings |
| `GetSettings` | Any view | — | — | Settings + accounts |
| `UpdateCommunityName` | Treasurer | `Name` | Rule 1 | Updated settings |
| `RenameAccount` | Treasurer | `AccountID`, `Name` | Rule 5 | Updated account |
| `UpdateOpeningBalance` | Treasurer | `AccountID`, `Amount` | Rule 6; confirmation in UI because all balances change | Updated account |

## Operations

No HTTP API. Operations are methods on `app.SetupService`, called from the TUI.
Operation names match the use cases above.

`SetupInput` carries every field as the typed string, so one validation reports
unreadable and out-of-range values alike. House row `i` is house number `i + 1`.
Account 0 is the `cash` account and account 1 the `bank` account; kinds are fixed
by position. Empty `Credit` and `Arrears` mean zero.

`SetupSummary` (step 7): house count, cuota, opening balance per account and total,
houses with a `CoveredThrough` month, total credit, total opening arrears.

Validation errors are returned as codes per field key (`community_name`,
`houses[3].credit`, …); the TUI maps each code to its Spanish message in
`internal/tui/text`.

Date and month input: dates are typed `DD/MM/AAAA` and months `MM/AAAA`
(`15/05/2026`, `05/2026`); they are stored as `YYYY-MM-DD` and `YYYY-MM`.

Migration `001_setup.sql` adds two constraints beyond the entity table:
`accounts.name` is `UNIQUE COLLATE NOCASE` (rule 5's full case-insensitive check
stays in the application), and the opening credit and arrears columns are
`CHECK (>= 0)`.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Wizard — 1 Coto | Coto name | Enter: next · Esc: cancel wizard (confirm) |
| Wizard — 2 Casas | Count and prefix, live preview `CASA 01 … CASA 18` | Enter: next · Shift+Tab/←: back |
| Wizard — 3 Cuota | Monthly cuota amount | as above |
| Wizard — 4 Cuentas | Names of the cash and bank accounts (defaults `Efectivo`, `Banco (NU)`) | as above |
| Wizard — 5 Apertura | Opening date; opening balance per account | as above |
| Wizard — 6 Casas al abrir | Table, one row per house: `Cubierto hasta` (month or empty), `Crédito`, `Adeudo inicial` | ↑/↓ row · Tab field · Enter: next |
| Wizard — 7 Resumen | Totals: houses, cuota, balances per account and total, houses with coverage, total credit, total opening arrears | Enter: confirm and write · ←: back |
| Ajustes (overlay from main shell) | Edit coto name, account names, opening balances | e: edit field · Esc: close |

The wizard runs full-screen (no main shell) and keeps all input in memory until
step 7 is confirmed. Moving back keeps entered values.

- `←` goes back a step only when the cursor is before the first character of the
  first field; `shift+tab` goes back from the first field. Elsewhere both keys move
  inside the step.
- `esc` + `Sí` quits the application and writes nothing (rule 11).
- The opening date is prefilled with today.
- Changing the house count after step 6 keeps the values of house numbers that
  still exist.
- After step 7 the shell opens without a restart. Its summary panel shows the
  coto name as title, each account with its opening balance, and the total; the
  `Disponible` row stays a `$0.00` placeholder until Phase 3 computes it.

## Business Rules

1. `CommunityName`: required, trimmed, 1–60 characters.
2. `HouseCount`: integer 1–200. Default 18.
3. `HousePrefix`: 0–10 characters, kept exactly as typed (a trailing space or dash is
   significant). Default `CASA `. House code = prefix + number zero-padded to
   `max(2, digits(HouseCount))` → `CASA 01` … `CASA 18`.
4. `FeeAmount`: money > 0. Stored as the `initial` fee rate effective from the opening month.
5. Account names: required, trimmed, 1–30 characters, unique (case-insensitive).
   Exactly one `cash` and one `bank` account.
6. Opening balances: money ≥ 0 per account.
7. `OpeningDate`: required, not after today. Opening month = its month.
   Not editable after setup.
8. Per house: `CoveredThrough` empty (nothing covered) or a month ≥ opening month and
   ≤ opening month + 36 months. Months from the opening month through `CoveredThrough`
   are covered without a payment; their value counts as prepaid money already inside
   the opening balances. `Credit` is 0 ≤ credit < `FeeAmount` and applies to the month
   after `CoveredThrough` (or to the opening month when empty). `Arrears` ≥ 0 is debt
   from before the opening month.
9. A house with `Arrears > 0` must have empty `CoveredThrough` and `Credit = 0`.
10. Startup: if the database file exists but cannot be opened or migrated, show the
    path and error and exit — never start the wizard over it. The wizard starts only
    when migrations succeed and no settings row exists.
11. Cancelling the wizard writes nothing; the next launch starts it again.

## Acceptance Criteria

- [ ] A fresh database launches the wizard; a completed one launches the main shell.
- [ ] Count 18 with prefix `CASA ` creates `CASA 01` … `CASA 18`; prefix `C-` creates `C-01` … `C-18`; count 120 pads to 3 digits.
- [ ] Every invalid field on a wizard step is reported at once and blocks moving forward.
- [ ] Confirming writes settings, 2 accounts, N houses, and 1 initial fee rate in one transaction; a forced failure mid-write leaves no rows.
- [ ] `CompleteSetup` on a completed database is rejected.
- [ ] Opening date in the future is rejected; `CoveredThrough` before the opening month is rejected.
- [ ] A house with arrears and coverage is rejected.
- [ ] An unreadable database file shows an error and does not start the wizard.
- [ ] Renaming an account to an existing name (any case) is rejected.
- [ ] Changing an opening balance changes every balance derived from it (verified once the ledger exists).
