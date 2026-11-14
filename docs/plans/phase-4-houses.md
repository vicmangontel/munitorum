---
type: plan
phase: 4
module: houses
status: not-started
spec: docs/specs/houses.md
depends-on: phase-3
steps: 3
current-step: 1
---

# Phase 4 — Houses

## Goal

The treasurer records a cuota payment as an amount of money and the app decides
which months it covers, with a live preview before saving. The Casas view shows
every house's state, arrears, and adelanto; the estado de cuenta shows the month
strip, owed months, how far the adelanto reaches, and which months each payment
covered. Payments are edited from the houses form and deleted or restored through
the ledger, and coverage is always recomputed. House contacts, opening data, and
cuota rate changes with either policy are managed here. `FeeSummary` exists as an
operation for the dashboard (Phase 7) and reports (Phase 8); this phase builds no
screen for it.

The allocation model is one pure function in `internal/domain/fees`; no other code
decides coverage (rule 2).

## Prerequisites

- Phase 3: movements, `LedgerService` record rules, rule 12 owner routing, the
  extendable `ListMovements` filter, delete and restore.
- Phase 1: house opening rules in `internal/domain`. Phase 2: picker component.
- Approval of migration `004_houses.sql` before it is written.

## Steps

| Step | Slice | Spec | Status |
|---|---|---|---|
| 1 | Paying a cuota — carries the spine | `docs/specs/houses.md` | not-started |
| 2 | Corrections and contacts | `docs/specs/houses.md` | not-started |
| 3 | Cuota rate changes | `docs/specs/houses.md` | not-started |

## Step 1 — Paying a cuota

Spec sections: Entities, Allocation model steps 1–3 and 5–8, Use Cases (`ListHouses`,
`GetHouseStatement`, `PreviewFeePayment`, `RecordFeePayment`), UI Views (Casas,
Estado de cuenta, Pago de cuota), Business Rules 1, 2.

### Domain and application tasks
- [ ] `internal/domain/fees`: `Due(m)` from the rate history; credit sources (opening value as one credit dated the opening date, then live payments by date and id); filling (opening arrears first, then months in order from the opening month, no limit into the future)
- [ ] `internal/domain/fees`: month states, `Arrears(D)`, `Prepaid(D)`, covered-through, and the months each payment covered
- [ ] Migration `internal/store/migrations/004_houses.sql`: `house_contacts`, `movements.house_id`, index `movements(house_id)`, unique live position index (ask first)
- [ ] `app.HouseService` and `app.FeeService` with repository interfaces; `ListHouses(OnlyWithArrears)` as of today
- [ ] `GetHouseStatement`: months from the opening month to the later of the current month and the last month with an applied amount; payments with their months; opening data
- [ ] `PreviewFeePayment(HouseID, Date, Amount, EditingID?)`: months touched with applied, due, state; resulting arrears and prepaid
- [ ] `RecordFeePayment`: ledger rules 1–3, 10 and rule 1; fee system category; one transaction
- [ ] Ledger integration: `GetMovement` reports `Owner = houses` for fee payments; Movimientos shows the house code in De/Para
- [ ] `internal/store`: SQLite implementation
- [ ] Wide event `record_fee_payment`: `movement_id`, `house_id`, `account_id`, `amount_cents`
- [ ] `cmd/devseed`: fee payments that reproduce the mixed states, including the spreadsheet Casa 03, 04, and 08 cases

### TUI tasks
- [ ] `internal/tui/houses` replaces the placeholder: columns Casa, Propietario (empty until Step 2), Cubierto hasta, Estado (`Al corriente`, `Debe 2 meses`, `Parcial`), Adeudo, Adelanto; `p` registrar pago
- [ ] Estado de cuenta in the detail panel: month strip by year (`✓` Positive, `◐` Warning with amount, `✗` Negative), owed months with total, adelanto with the month it reaches, payments with the months each covered, opening data
- [ ] Pago de cuota overlay: house picker pre-selected from the list; fecha, cuenta, monto (default arrears, else one month's due), descripción (default `Pago cuota {range}` built in `internal/tui/text` from the preview), notas, enlace, recibo manual pendiente; live preview; every error at once

### Test tasks
- [ ] `internal/domain/fees` example tests: sep–oct owed + 450 on 12 nov covers sep–nov; Casa 03 (opening arrears 150, pays 360 then 90); covered through jan 2027 with no payment shows prepaid 300 in nov 2026; arrears 600 + 750 covers the opening month; Casa 08 owes every month
- [ ] `internal/domain/fees` property tests: Σ applied + arrears paid + unapplied = Σ credits; months filled in order with at most one partial month; Σ prepaid ≤ Σ fee credits
- [ ] `internal/app` tests: the preview equals coverage after saving; `ListHouses` states and `OnlyWithArrears`; a fee payment without a house rejected; `RecordIncome` with `Cuota` still rejected
- [ ] TUI tests: changing the amount updates the preview; every form error shown; default description for a range, a partial month, and an adelanto
- [ ] Mutation: `just test-mutation ./internal/domain/fees`; investigate survivors

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] The Casa 03 case from the spreadsheet shows on screen as in the spec, and the preview matches what is saved

## Step 2 — Corrections and contacts

Spec sections: Use Cases (`UpdateFeePayment`, `UpdateHouseOpening`, contact operations),
UI Views (Casas `f` `c` `o`, Estado de cuenta full view and `e`, Contactos),
Business Rules 3–5, 8.

### Domain and application tasks
- [ ] `UpdateFeePayment`: same rules as recording; the house may change
- [ ] `UpdateHouseOpening`: setup rules 8–9 through the Phase 1 domain functions
- [ ] `ListMovements` filter gains `HouseID`
- [ ] `AddHouseContact`, `UpdateHouseContact`, `DeleteHouseContact`: rules 3–5 (at most two live contacts, first free position, position 2 promoted when 1 is deleted)
- [ ] Wide events `update_fee_payment` (`movement_id`, old and new `house_id`, `amount_cents`), `update_house_opening` (`house_id`, centavos), contact events (`house_id`, `contact_id`, `position`); never names, phones, emails
- [ ] `cmd/devseed`: contacts for most houses, two contacts on some

### TUI tasks
- [ ] Ledger `e` on a fee payment opens the Pago de cuota form (registered for owner `houses`), pre-filled, with the preview computed using `EditingID`
- [ ] Estado de cuenta full view on `enter`; `e` edits the selected payment; Casas `f` solo con adeudo
- [ ] Datos de apertura overlay (`o`) and Contactos overlay (`c`: `n`, `e`, `d`); Propietario column from position 1

### Test tasks
- [ ] `internal/app` tests: deleting a payment in the middle of a sequence through `DeleteMovement` re-flows later payments into earlier months, and restoring reverses it; moving a payment to another house moves its coverage; editing opening data recomputes coverage; third contact rejected; deleting contact 1 promotes contact 2
- [ ] TUI tests: editing from the ledger opens the houses form; contact form shows every error

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] Deleting a mid-sequence payment from Movimientos visibly re-flows the estado de cuenta

## Step 3 — Cuota rate changes

Spec sections: Allocation model step 4, Use Cases (`ListFeeRates`, `CreateFeeRateChange`,
`DeleteFeeRateChange`, `FeeSummary`), UI Views (Tarifas de cuota), Business Rules 6–8.

### Domain and application tasks
- [ ] `internal/domain/fees`: frozen months for each `honor_prepaid` change in `recorded_at` order (rerun with rates and credits known before the change); `charge_difference` freezes nothing
- [ ] `ListFeeRates` (newest first), `CreateFeeRateChange` (rule 6), `DeleteFeeRateChange` (rule 7)
- [ ] `FeeSummary(AsOf)`: total arrears, total prepaid, houses with arrears, month grid for a year
- [ ] Wide events `create_fee_rate_change` (`rate_id`, `effective_month`, `amount_cents`, `policy`), `delete_fee_rate_change` (`rate_id`)
- [ ] `cmd/devseed`: one future rate change

### TUI tasks
- [ ] Tarifas de cuota overlay (`t`): history with policy; `n` asks month, amount, and policy, each policy with its explanation; `d` deletes the last change after confirmation

### Test tasks
- [ ] `internal/domain/fees` example tests: rate 170 from jan 2027 — `honor_prepaid` keeps a house paid through apr 2027 covered; `charge_difference` makes 1,800 paid in may 2026 cover may–dec 2026 at 150 and jan–mar 2027 at 170 with apr 2027 partial at 90 (Casa 04)
- [ ] `internal/domain/fees` property tests extended with random rate changes and both policies
- [ ] `internal/app` tests: effective month in the past or not after the last change rejected; only the latest future change deletable; `initial` not deletable; `FeeSummary` totals equal the sum over `ListHouses`
- [ ] TUI tests: the policy explanation shows for the selected policy; delete confirmation declined writes nothing
- [ ] Mutation: `just test-mutation ./internal/domain/fees` and the fee operations in `./internal/app`; record under `docs/architecture.md` → Mutation testing (ask first)

### Done when
- [ ] `go build ./...`, `go vet ./...`, `just test` pass
- [ ] Adding the jan 2027 rate change with each policy shows the Casa 04 result in its estado de cuenta

## Manual Test Scenarios

1. On the devseed database open Casa 03: arrears cleared, may covered, jun partial then completed; the month strip and payments list agree.
2. Record a payment of 450 for a house owing sep and oct: the preview shows sep–nov covered before saving; the list then shows `Al corriente`.
3. Delete a mid-sequence payment from Movimientos; the house's estado de cuenta re-flows; restore it.
4. Add a rate change for a future month with `cobrar diferencia`, check a prepaid house, delete the change, add it with `respetar adelantos`, check again.

## Definition of Done
- [ ] Every step is done
- [ ] `go build ./...` and `go vet ./...` clean
- [ ] All new tests pass (`just test`)
- [ ] Works end-to-end in Windows Terminal
- [ ] `docs/specs/houses.md` up to date
- [ ] `docs/architecture.md` Test Coverage Map row for houses and the fee allocation (ask first)
- [ ] `CLAUDE.md` Current State updated
