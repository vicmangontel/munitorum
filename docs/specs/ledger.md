---
type: spec
module: ledger
status: draft
entities: Movement, Category
operations: ListMovements, GetMovement, RecordIncome, RecordExpense, RecordTransfer, RecordCashCount, UpdateIncome, UpdateExpense, UpdateTransfer, UpdateAdjustment, DeleteMovement, RestoreMovement, AccountBalances, ListPendingReceipts, MarkReceiptDone, ListCategories, CreateCategory, RenameCategory, SetCategoryActive
depends-on: setup, directory
---

# Ledger — Specification

## Purpose

The libro de caja: every peso that enters, leaves, or moves between the two
accounts is one movement. Account balances, running balances, and month totals
are derived from movements plus the opening balances, never typed in, so the
row-skipping balance errors of the spreadsheet cannot happen. This module owns
the movement table and the generic flows (other income, expense, transfer, cash
count); the houses, assessments, and reminders modules record their own
movements through the same table and extend the expense and income forms.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Movement` (`movements`) | `id`, `date TEXT NOT NULL` (`YYYY-MM-DD`), `kind TEXT CHECK (kind IN ('income','expense','transfer','adjustment'))`, `account_id INTEGER NOT NULL REFERENCES accounts(id)`, `to_account_id INTEGER NULL REFERENCES accounts(id)`, `amount_cents INTEGER NOT NULL`, `category_id INTEGER NULL REFERENCES categories(id)`, `vendor_id INTEGER NULL REFERENCES vendors(id)`, `description TEXT NOT NULL`, `notes TEXT NULL`, `receipt_link TEXT NULL`, `manual_receipt TEXT NOT NULL DEFAULT 'none' CHECK (manual_receipt IN ('none','pending','done'))`, `created_at`, `updated_at`, `deleted_at` | Soft-deleted. Later modules add columns by migration: `house_id` (houses), `assessment_id`, `project_id`, `fund_assessment_id`, `fund_amount_cents` (assessments). |
| `Category` (`categories`) | `id`, `name TEXT NOT NULL`, `direction TEXT CHECK (direction IN ('income','expense'))`, `system_key TEXT NULL UNIQUE CHECK (system_key IN ('fee','assessment'))`, `active INTEGER NOT NULL DEFAULT 1`, `sort_order INTEGER NOT NULL` | Not soft-deleted; deactivated instead. Seeded by the migration. |

Table-level checks on `movements`:
- `amount_cents > 0`, except `kind = 'adjustment'` where `amount_cents <> 0` (signed).
- `to_account_id IS NOT NULL` if and only if `kind = 'transfer'`, and `to_account_id <> account_id`.
- `category_id IS NOT NULL` if and only if `kind IN ('income','expense')`.
- `manual_receipt = 'none'` unless `kind = 'income'`.

Indexes: `movements(date, id)`, `movements(account_id)`, `movements(category_id)`, `movements(vendor_id)`.

Migration: `internal/store/migrations/003_ledger.sql`. Seed rows:

| name | direction | system_key | sort_order |
|---|---|---|---|
| Cuota | income | fee | 1 |
| Extraordinaria | income | assessment | 2 |
| Otros ingresos | income | — | 3 |
| Insumos | expense | — | 10 |
| Servicios | expense | — | 11 |
| Mtto preventivo | expense | — | 12 |
| Reparaciones | expense | — | 13 |

### Effect on balances

| Kind | `account_id` | `to_account_id` | Available funds / category totals |
|---|---|---|---|
| income | + amount | — | income category total + |
| expense | − amount | — | expense category total + |
| transfer | − amount | + amount | no change |
| adjustment | + signed amount | — | listed separately as `Ajustes` |

Account balance as of a date = `accounts.opening_balance_cents` + effects of live
movements with `date ≤` that date. Computed in `internal/domain`.

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `ListMovements` | Treasurer, reports | `Filter{Month or All, AccountID, CategoryID, VendorID, Text, IncludeDeleted}` | Rule 9; extended later with `HouseID`, `AssessmentID`, `ProjectID` | `Ledger{Rows[]{Movement, display fields, RunningBalance}, Totals{Income, Expense, Adjustments, Net}, OpeningBalance, ClosingBalance}` |
| `GetMovement` | Treasurer | `ID` | Includes `Owner` — which form edits it (rule 12) | `MovementDetail` |
| `RecordIncome` | Treasurer | `Date, AccountID, CategoryID, Amount, Description, Notes, ReceiptLink, ManualReceiptPending` | Rules 1–4, 10; category must be an active income category with no `system_key` | Created movement |
| `RecordExpense` | Treasurer | `Date, AccountID, CategoryID, VendorID?, Amount, Description, Notes, ReceiptLink, ConfirmNegative` | Rules 1–4, 7 | Created movement, or `NegativeBalanceWarning{Account, Resulting}` when not confirmed |
| `RecordTransfer` | Treasurer | `Date, FromAccountID, ToAccountID, Amount, Description?, Notes, ConfirmNegative` | Rules 1–3, 7, 8 | Created movement or warning |
| `RecordCashCount` | Treasurer | `Date, AccountID, Counted, Notes` | Rule 11 | `{Difference, Movement?}` |
| `UpdateIncome` / `UpdateExpense` / `UpdateTransfer` / `UpdateAdjustment` | Treasurer | `ID` + same fields as record | Same rules; kind cannot change; rule 12 | Updated movement or warning |
| `DeleteMovement` | Treasurer | `ID` | Soft delete; confirmation in UI | — |
| `RestoreMovement` | Treasurer | `ID` | Rule 7 warning applies | — |
| `AccountBalances` | Dashboard, summary panel, reports | `AsOf date` | Derived | `[]{Account, Balance}` + total |
| `ListPendingReceipts` | Treasurer, dashboard | — | `manual_receipt = 'pending'`, live | rows |
| `MarkReceiptDone` | Treasurer | `ID` | Only pending → done (and done → pending to undo) | — |
| `ListCategories` | Forms, settings | `Direction?`, `IncludeInactive` | — | rows |
| `CreateCategory` | Treasurer | `Name, Direction` | Rule 5 | created |
| `RenameCategory` | Treasurer | `ID, Name` | Rule 5 | updated |
| `SetCategoryActive` | Treasurer | `ID, Active` | Rule 6 | updated |

## Operations

Methods on `app.LedgerService` and `app.CategoryService`, named as the use cases.

**Extension points used by later modules.** The expense form exposes optional
sections that other modules contribute: obra and fund portion (assessments),
pre-filled values and "mark reminder done" (reminders). `ListMovements.Filter`
gains fields owned by those modules. The ledger does not import those modules;
they register form sections and filter fields with the TUI root, and register
validation hooks with `app.LedgerService` that run inside the same transaction
on record, update, delete, and restore (e.g. assessments rule 3: a fund may never
go negative). A hook failure rejects the whole operation. No hooks are built yet;
they arrive with their first users.

**Contract details.**
- Inputs `Date` (`DD/MM/AAAA`) and `Amount` are the text as typed, so one
  validation reports unreadable and out-of-range values together.
- `RecordExpense` returns the warning as an error value (`app.NegativeBalanceWarning`,
  read with `errors.As`), like `app.VendorConflict`.
- `RecordExpense.VendorID`, when given, must name a live vendor (`vendor_invalid`).
- `ListMovements` rows carry the vendor display name and a `VendorDeleted` flag; the
  TUI adds the ` (eliminado)` suffix, since `internal/app` holds no Spanish text.
- Validation codes: `date_invalid`, `date_future`, `date_before_opening`,
  `amount_invalid`, `amount_not_positive`, `account_invalid`, `description_length`,
  `notes_length`, `receipt_link_length`, `category_invalid`, `category_reserved`,
  `vendor_invalid`.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Movimientos (main panel) | Columns: Fecha, Categoría, De/Para, Descripción, Cuenta, Monto, Saldo, R (receipt mark). Default filter: current month; chronological; cursor on last row. Footer: Ingresos, Gastos, Ajustes, Neto for the filter | `i` ingreso · `g` gasto · `t` transferencia · `a` arqueo · `e` editar · `d` eliminar · `[`/`]` mes anterior/siguiente · `f` filtros · `/` buscar · `.` eliminados · `r` restaurar · `c` categorías |
| Detalle | All fields of the selected movement, including notes and receipt link | `m` marcar recibo hecho |
| Formulario ingreso / gasto / transferencia (overlay) | Create / edit | All field errors shown at once; negative-balance warning shows resulting balance and asks Sí/No |
| Arqueo (overlay) | Account, date, counted amount; shows system balance and difference live | Enter: record adjustment · Esc |
| Recibos pendientes (filter preset) | Income with pending manual receipt | `m` marcar hecho |
| Categorías (overlay) | List with direction and active flag | `n` · `e` renombrar · `space` activar/desactivar |

Amount display: income `+$150.00` Positive; expense `−$350.00` Negative; transfer
`$600.00 Efectivo → Banco (NU)` Muted; adjustment signed with label `Ajuste`.
`Saldo` is the account's running balance when filtered by one account, otherwise the
total across accounts.

## Business Rules

1. `Date` required, `≥ settings.opening_date`, `≤` today.
2. `Amount` money > 0 (adjustments: computed, non-zero).
3. Account(s) must exist. `Description` required except for transfers (generated:
   `Transferencia {from} → {to}`), 1–120 characters. `Notes` ≤ 500. `ReceiptLink` ≤ 500,
   plain text, not validated as a URL.
4. Category must be active and match the movement's direction. Income with a
   `system_key` category is recorded only by the houses and assessments modules.
5. Category names: 1–40 characters, unique per direction (case- and accent-insensitive).
6. System categories can be renamed but not deactivated. Inactive categories are not
   offered in forms; movements that use them keep and display them.
7. When an expense, transfer, edit, or restore leaves the current balance (all live
   movements, all dates) of an account the operation touches below zero, the operation
   returns a warning and writes nothing unless `ConfirmNegative` is set. Other accounts
   are not checked. Exactly zero does not warn; an account already below zero warns
   again. Income never warns.
8. Transfer accounts must differ.
9. Ordering: `date` ascending, then `id` ascending. Running balance starts from the
   balance at the end of the day before the first row in the filter.
10. Manual receipt applies to income only: `pending` when the flag is set on create;
    `done` when marked.
11. Cash count: system balance = account balance as of `Date` including all movements
    of that date. Difference = counted − system. Difference 0 → nothing written,
    message `Cuadra`. Otherwise one `adjustment` movement for the difference, description
    `Arqueo: contado $X, sistema $Y`. Counted money ≥ 0.
12. A movement is edited by the form of the module that owns it: fee payments
    (houses), assessment payments (assessments); everything else by the ledger forms.
    Kind never changes; to change kind, delete and record again.
13. Deleted movements are excluded from every balance, total, and list unless
    `IncludeDeleted`; when shown, they are struck through and excluded from totals.

## Acceptance Criteria

- [ ] With opening balances 7,500.00 (cash) and 0 (bank), an expense of 178.50 from cash shows a cash balance of 7,321.50.
- [ ] A transfer of 600.00 cash → bank changes both balances and not the total, and appears as one row.
- [ ] Running balance stays correct after editing and after deleting a movement in the middle of the month.
- [ ] An expense that would make cash negative asks for confirmation; declining writes nothing.
- [ ] A movement dated before the opening date or after today is rejected.
- [ ] All invalid fields of a form are reported together.
- [ ] A cash count of 4,300.00 against a system balance of 4,323.50 records a −23.50 adjustment; a matching count records nothing.
- [ ] Income with manual receipt pending appears in Recibos pendientes until marked done; the flag cannot be set on an expense.
- [ ] A deactivated category is not offered in forms but still shows on old movements; `Cuota` cannot be deactivated.
- [ ] `RecordIncome` with the `Cuota` category is rejected.
- [ ] A deleted movement can be shown and restored; restoring brings its effect back into balances.
- [ ] Month totals (income, expense, adjustments, net) match the sum of the rows in the filter.
- [ ] Property: for any sequence of movements, the sum of account balances equals the opening total + income − expense + adjustments.
