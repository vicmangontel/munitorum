---
type: plan
phase: 3
module: ledger
status: in-progress
spec: docs/specs/ledger.md
depends-on: phase-1, phase-2
steps: 4
current-step: 3
---

# Phase 3 — Ledger

## Goal

The libro de caja works: the treasurer records other income, expenses, transfers,
and cash counts; sees each month's movements with a running balance and totals;
corrects, deletes, and restores movements; filters and searches; manages
categories; and tracks manual receipts. Account balances in the summary panel are
derived from movements plus opening balances and update after every save.

**Extension points.** This phase builds only what Houses (Phase 4) needs: rule 12
routing — `GetMovement` reports the owning module and the TUI opens the form
registered for that owner — and a `ListMovements` filter that later modules
extend with their own fields. The validation hooks and expense-form sections
described under Operations → Extension points are built with their first users,
Assessments (Phase 5) and Reminders (Phase 6).

## Prerequisites

- Phase 1 (settings, accounts, form, confirmation) and Phase 2 (vendor picker,
  `FoldKey`, view overlays and detail panel).
- Approval of migration `003_ledger.sql` before it is written.

## Steps

| Step | Slice | Spec | Status |
|---|---|---|---|
| 1 | Income and expenses — carries the spine | `docs/specs/ledger.md` | awaiting manual verification |
| 2 | Transfers and arqueo | `docs/specs/ledger.md` | done |
| 3 | Correcting movements | `docs/specs/ledger.md` | not-started |
| 4 | Finding and classifying | `docs/specs/ledger.md` | not-started |

## Step 1 — Income and expenses

Spec sections: Entities, Effect on balances, Use Cases (`ListMovements` by month and
account, `RecordIncome`, `RecordExpense`, `AccountBalances`, `ListCategories`),
UI Views (Movimientos, income and expense forms), Business Rules 1–4, 7, 9, 10.

### Domain and application tasks
- [x] Migration `internal/store/migrations/003_ledger.sql`: `categories` with the seven seed rows, `movements` with the table-level checks and indexes (ask first)
- [x] `internal/domain`: effect of a movement by kind; account balance as of a date; running balance (rule 9) for one account or the total; month totals (income, expense, adjustments, net)
- [x] `app.LedgerService` and `app.CategoryService` with repository interfaces; `ListCategories` (direction, include inactive)
- [x] `RecordIncome`: rules 1–4, 10; rejects categories with a `system_key`
- [x] `RecordExpense`: rules 1–4, 7; returns `NegativeBalanceWarning{Account, Resulting}` and writes nothing unless `ConfirmNegative`
- [x] `ListMovements` with `Filter{Month, AccountID}`: rows with display fields (category, vendor display name with ` (eliminado)`, account), running balance, totals, opening and closing balance
- [x] `AccountBalances(AsOf)`: per account and total
- [x] `internal/store`: SQLite implementation
- [x] Wide events `record_income`, `record_expense`: `movement_id`, `account_id`, `category_id`, `vendor_id`, `amount_cents`; a declined negative-balance warning logs `outcome=validation`, `warning=negative_balance`
- [x] `cmd/devseed`: six months of income and expenses across both accounts

### TUI tasks
- [x] `internal/tui/ledger` replaces the placeholder: columns Fecha, Categoría, De/Para, Descripción, Cuenta, Monto, Saldo, R; current month by default; cursor on the last row; `[`/`]` month; footer Ingresos, Gastos, Ajustes, Neto
- [x] Amount styles: income `+$150.00` Positive, expense `−$350.00` Negative
- [x] Income form (`i`) and expense form (`g`, vendor picker from Phase 2): every error at once; negative-balance warning shows the resulting balance and asks `Sí`/`No`
- [x] Summary panel shows `AccountBalances(today)`; any save sends a change message that reloads the summary and the open view

### Test tasks
- [x] `internal/domain` table tests: balances, running balance across a month boundary, totals
- [x] `internal/domain` property test: for any movements, Σ account balances = opening total + income − expense + adjustments
- [x] `internal/app` tests: 7,500.00 cash − 178.50 → 7,321.50; date before opening or after today rejected; every invalid field together; `RecordIncome` with `Cuota` rejected; declined warning writes nothing; month totals equal the sum of rows; changing an opening balance moves every derived balance (setup acceptance)
- [x] TUI tests: forms show every error; warning `No` writes nothing; `[`/`]` changes the month

### Done when
- [x] `go build ./...`, `go vet ./...`, `just test` pass
- [x] An expense of 178.50 from cash shows cash at 7,321.50 in the list and in the summary panel
- [ ] Deferred Phase 0 manual checks: `just run` creates `munitorum.db` and `logs/` beside the binary and `q` exits cleanly; the shell renders correctly in Windows Terminal dark and light profiles

## Step 2 — Transfers and arqueo

Spec sections: Use Cases (`RecordTransfer`, `RecordCashCount`), UI Views (transfer form,
Arqueo), Business Rules 3, 7, 8, 11.

### Domain and application tasks
- [x] `internal/domain`: cash-count difference (counted − system balance as of the date, all movements of that date included)
- [x] `RecordTransfer`: rules 1–3, 7, 8; generated description `Transferencia {from} → {to}` (text supplied by the form from `internal/tui/text`)
- [x] `RecordCashCount`: rule 11; difference 0 writes nothing; otherwise one `adjustment` with description `Arqueo: contado $X, sistema $Y`
- [x] Wide events `record_transfer` (`account_id`, `to_account_id`, `amount_cents`), `record_cash_count` (`account_id`, `difference_cents`, `movement_id` when written)
- [x] `cmd/devseed`: two transfers and one arqueo adjustment

### TUI tasks
- [x] Transfer form (`t`) with the negative-balance warning
- [x] Arqueo overlay (`a`): account, date, counted amount; system balance and difference update live; `Cuadra` when zero
- [x] Display: transfer as one row `$600.00 Efectivo → Banco (NU)` in Muted; adjustment signed with label `Ajuste`

### Test tasks
- [x] `internal/app` tests: 600.00 cash → bank changes both balances and not the total and is one row; same account rejected; transfer leaving cash negative warns; count 4,300.00 vs system 4,323.50 → −23.50 adjustment; matching count writes nothing; negative counted rejected
- [x] TUI tests: arqueo difference updates as the amount is typed

### Done when
- [x] `go build ./...`, `go vet ./...`, `just test` pass
- [x] A transfer and an arqueo adjustment appear in Movimientos and the summary panel matches them

## Step 3 — Correcting movements

Spec sections: Use Cases (`GetMovement`, `Update*`, `DeleteMovement`, `RestoreMovement`),
UI Views (Detalle), Business Rules 7, 12, 13.

### Domain and application tasks
- [ ] `GetMovement` → `MovementDetail` with every field and `Owner` (`ledger`, `houses`, `assessments`), derived from the category's `system_key`
- [ ] `UpdateIncome`, `UpdateExpense`, `UpdateTransfer`, `UpdateAdjustment`: same rules as recording; kind never changes; rejected when `Owner` is not `ledger`; rule 7 warning
- [ ] `DeleteMovement` (soft) and `RestoreMovement` (rule 7 warning on restore)
- [ ] `ListMovements` `IncludeDeleted`: deleted rows flagged, excluded from totals and the running balance (rule 13)
- [ ] Wide events `update_movement` (`movement_id`, `kind`, old and new `amount_cents`), `delete_movement`, `restore_movement`

### TUI tasks
- [ ] Detail panel: every field of the selected movement, including notes and receipt link
- [ ] `e` opens the form registered for the movement's owner (ledger forms registered now; houses registers its form in Phase 4) pre-filled; `d` with confirmation; `.` shows deleted rows struck through (style in `theme`); `r` restores

### Test tasks
- [ ] `internal/app` tests: running balance correct after editing and after deleting a movement in the middle of the month; restoring brings its effect back; a restore that leaves an account negative warns; updating a `Cuota` movement (inserted directly in the test) through `UpdateIncome` is rejected
- [ ] TUI tests: `e` pre-fills the form; declining the delete confirmation writes nothing; `.` toggles deleted rows

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] Editing, deleting, and restoring a mid-month movement keeps every later running balance correct on screen

## Step 4 — Finding and classifying

Spec sections: Use Cases (`ListMovements` full filter, `ListPendingReceipts`,
`MarkReceiptDone`, category operations), UI Views (filters, Recibos pendientes,
Categorías), Business Rules 5, 6, 10.

### Domain and application tasks
- [ ] `ListMovements` filter completed: all months, `CategoryID`, `VendorID`, folded `Text` over description and vendor name; the filter struct is the one later modules extend
- [ ] `CreateCategory`, `RenameCategory` (rule 5, uniqueness per direction with `FoldKey`), `SetCategoryActive` (rule 6: system categories cannot be deactivated)
- [ ] `ListPendingReceipts` and `MarkReceiptDone` (pending ↔ done, income only)
- [ ] Wide events `create_category`, `rename_category`, `set_category_active` (`category_id`, `active`), `mark_receipt_done` (`movement_id`, `status`)
- [ ] `cmd/devseed`: two incomes with pending manual receipts, one inactive category with old movements

### TUI tasks
- [ ] `f` filter overlay (month or all, account, category, vendor picker); `/` text search; Recibos pendientes preset; `m` in the detail panel marks the receipt
- [ ] Categorías overlay (`c`): direction and active flag; `n`, `e` rename, `space` activate/deactivate; inactive categories not offered in forms
- [ ] Directorio detail `m` ver movimientos: switches to Movimientos filtered by that vendor

### Test tasks
- [ ] `internal/app` tests: inactive category absent from `ListCategories` for forms and still shown on old movements; `Cuota` cannot be deactivated; folded name collision per direction rejected and allowed across directions; pending receipt listed until marked done; the manual-receipt flag cannot be stored on an expense; filtered totals equal the sum of filtered rows
- [ ] TUI tests: filter overlay narrows the list; categories overlay toggles active
- [ ] Mutation: `just test-mutation ./internal/domain` and the ledger operations in `./internal/app`; investigate survivors; record the result under `docs/architecture.md` → Mutation testing (ask first)

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] Filtering by vendor from Directorio shows only that vendor's movements with matching totals

## Manual Test Scenarios

1. Opening 7,500.00 cash: record an expense of 178.50 with a vendor, an income of 200.00 with manual receipt pending, a transfer of 600.00 to the bank; the summary shows cash 6,921.50 and the total unchanged by the transfer.
2. Record an expense larger than the bank balance: the warning shows the resulting balance; `No` writes nothing; `Sí` records it.
3. Cash count 4,300.00 against a system balance of 4,323.50: a −23.50 `Ajuste` row appears.
4. Delete a mid-month expense, check the running balances, show deleted with `.`, restore with `r`.

## Definition of Done
- [ ] Every step is done
- [ ] `go build ./...` and `go vet ./...` clean
- [ ] All new tests pass (`just test`)
- [ ] Works end-to-end in Windows Terminal
- [ ] `docs/specs/ledger.md` up to date, including where the extension hooks are built (ask first)
- [ ] `docs/architecture.md` Test Coverage Map row for the ledger (ask first)
- [ ] `CLAUDE.md` Current State updated
