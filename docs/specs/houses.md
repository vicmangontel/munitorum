---
type: spec
module: houses
status: draft
entities: House, HouseContact, FeeRate, FeePayment (movement), MonthStatus, HouseStatement
operations: ListHouses, GetHouseStatement, PreviewFeePayment, RecordFeePayment, UpdateFeePayment, UpdateHouseOpening, AddHouseContact, UpdateHouseContact, DeleteHouseContact, ListFeeRates, CreateFeeRateChange, DeleteFeeRateChange, FeeSummary
depends-on: setup, ledger
---

# Houses — Specification

## Purpose

Tracks what every casa owes and has paid in cuotas. A payment is an amount of
money; the app decides which months it covers, carries any surplus into future
months as adelanto, and shows each casa's estado de cuenta so the treasurer can
answer "what do I owe?" in seconds. It also owns house contacts and cuota rate
changes. Month coverage is always recomputed from payments — never stored — so
editing or deleting a payment, or adding a rate change, can never leave a casa in
an inconsistent state.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `House` (`houses`, from setup) | `number`, `code`, `opening_covered_through`, `opening_credit_cents`, `opening_arrears_cents` | Opening fields editable here. |
| `HouseContact` (`house_contacts`) | `id`, `house_id NOT NULL REFERENCES houses(id)`, `position INTEGER CHECK (position IN (1,2))`, `name TEXT NOT NULL`, `phone`, `email`, `created_at`, `updated_at`, `deleted_at` | Position 1 is the principal contact, shown as `Propietario`. Soft-deleted. |
| `FeeRate` (`fee_rates`, from setup) | `effective_month`, `amount_cents`, `policy`, `recorded_at` | Rate changes add rows with policy `honor_prepaid` (`respetar adelantos`) or `charge_difference` (`cobrar diferencia`). Not soft-deleted. |
| Fee payment | A `movements` row: `kind = 'income'`, category with `system_key = 'fee'`, `house_id` set | `house_id` column added to `movements` by this module. |
| `MonthStatus` | `Month`, `Due`, `Applied`, `State` (`covered` / `partial` / `owed` / `future`) | Derived, `internal/domain`. |
| `HouseStatement` | house, contacts, months, payments with the months each covered, arrears, prepaid | Derived. |

Migration: `internal/store/migrations/004_houses.sql` — `house_contacts`;
`ALTER TABLE movements ADD COLUMN house_id INTEGER NULL REFERENCES houses(id)`;
index `movements(house_id)`; unique index `house_contacts(house_id, position) WHERE deleted_at IS NULL`.

## Allocation model (domain)

Computed per house, as of a date `D` (default today); current month `C = month(D)`;
opening month `O`.

1. **Due per month.** For each month `m ≥ O`, `Due(m)` = amount of the latest rate with
   `effective_month ≤ m`, except months frozen by an `honor_prepaid` change (step 4).
2. **Credit sources, in order:**
   a. Opening value: months `O … opening_covered_through` valued at the initial rate,
      plus `opening_credit_cents` — treated as one credit dated the opening date.
   b. Live fee payments with `date ≤ D`, ordered by `date`, then `id`.
3. **Filling.** Each credit first pays any remaining `opening_arrears_cents`, then fills
   months in order from `O`: a month takes up to `Due(m) − Applied(m)`; the rest flows
   to the next month, without limit into the future. Each payment therefore covers a
   contiguous range of months, possibly starting and ending with a partial month.
4. **Frozen months.** For each `honor_prepaid` change `R` (in `recorded_at` order),
   run steps 1–3 with the rates known before `R` and only the credits dated before
   `R.recorded_at`. Months `≥ R.effective_month` fully covered in that run keep their
   previous `Due`. With `charge_difference`, nothing is frozen: the new `Due` applies
   and the same credits now cover fewer months (the difference is owed).
5. **State of month `m`:** `covered` when `Applied = Due`; `partial` when
   `0 < Applied < Due`; `owed` when `Applied = 0` and `m ≤ C`; `future` when
   `Applied = 0` and `m > C`.
6. **Arrears(D)** = remaining opening arrears + Σ over `m ≤ C` of `Due(m) − Applied(m)`.
   A month is owed from its 1st day (no grace period).
7. **Prepaid(D)** = Σ over `m > C` of `Applied(m)`.
8. **Covered through** = the last month of the unbroken run of `covered` months starting
   at `O` (none if `O` is not covered).

`FeeSummary(D)` returns, across all houses: total arrears, total prepaid, houses with
arrears, and the month grid for a year — used by the dashboard and reports.

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `ListHouses` | Treasurer | `OnlyWithArrears` | Model as of today | `[]{Code, Principal, CoveredThrough, State, MonthsOwed, Arrears, Prepaid}` |
| `GetHouseStatement` | Treasurer | `HouseID` | Months from `O` to the later of `C` and the last month with `Applied > 0` | `HouseStatement` |
| `PreviewFeePayment` | Fee payment form | `HouseID, Date, Amount, EditingID?` | Runs the model with the new or changed payment | `{Months[]{Month, Applied, Due, State}, ResultingArrears, ResultingPrepaid}` |
| `RecordFeePayment` | Treasurer | `HouseID, Date, AccountID, Amount, Description?, Notes, ReceiptLink, ManualReceiptPending` | Ledger rules 1–3, 10; rule 1 | Created movement |
| `UpdateFeePayment` | Treasurer | `ID` + same fields | As record; house may change | Updated movement |
| `UpdateHouseOpening` | Treasurer | `HouseID, CoveredThrough, Credit, Arrears` | Setup rules 8–9 | Updated house |
| `AddHouseContact` | Treasurer | `HouseID, Name, Phone, Email` | Rules 3–4 | Contact |
| `UpdateHouseContact` | Treasurer | `ID` + fields | Rule 3 | Contact |
| `DeleteHouseContact` | Treasurer | `ID` | Rule 5 | — |
| `ListFeeRates` | Treasurer | — | — | rates, newest first |
| `CreateFeeRateChange` | Treasurer | `EffectiveMonth, Amount, Policy` | Rule 6 | Rate |
| `DeleteFeeRateChange` | Treasurer | `ID` | Rule 7 | — |
| `FeeSummary` | Dashboard, reports | `AsOf` | Model | totals + grid |

Fee payments are deleted and restored through the ledger (`DeleteMovement`,
`RestoreMovement`); the ledger routes their edit to this module's form.

## Operations

Methods on `app.HouseService` and `app.FeeService`, named as the use cases.
The allocation model is a pure function in `internal/domain/fees`.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Casas (main panel) | Columns: Casa, Propietario, Cubierto hasta, Estado (`Al corriente`, `Debe 2 meses`, `Parcial`), Adeudo, Adelanto | `p` registrar pago · `enter` estado de cuenta · `f` solo con adeudo · `c` contactos · `o` datos de apertura · `t` tarifas de cuota |
| Estado de cuenta (detail panel; full view on `enter`) | Month strip by year (`may ✓ jun ✓ … nov ✗`, partial `◐` with amount), owed months listed with total, adelanto with month it reaches, payments with months each covered, opening data. Cooperaciones section added by assessments | `p` pago · `e` editar pago seleccionado |
| Pago de cuota (overlay) | Casa (picker; pre-selected from list), fecha, cuenta, monto (default: arrears, or one month's due if none), descripción (default generated), notas, enlace recibo, recibo manual pendiente | Live preview of months covered; all errors at once |
| Contactos (overlay) | Up to 2 contacts | `n` · `e` · `d` |
| Tarifas de cuota (overlay) | Rate history with policy | `n` nueva tarifa (asks month, amount, policy with explanation) · `d` eliminar última |

Default description: `Pago cuota {range}`, e.g. `Pago cuota sep–nov 2026`,
`Pago cuota nov 2026 (parcial)`, `Pago cuota adelanto dic 2026–may 2027`.
The statement always shows the derived months; the stored description is not
rewritten when a later edit changes coverage.

## Business Rules

1. A fee payment requires a house; `house_id` is required when the category is the
   `fee` system category, and the fee category is required when `house_id` is set
   without `assessment_id` (enforced in `internal/app`).
2. Allocation follows the model above; no other code decides coverage.
3. Contact: `Name` required 1–80 characters; `Phone` ≤ 30; `Email` optional, one `@`
   with text on both sides.
4. At most 2 live contacts per house. A new contact takes the first free position.
5. Deleting the position-1 contact while position 2 exists moves position 2 to 1.
6. Rate change: `Amount` > 0; `EffectiveMonth ≥ C` and after every existing
   `effective_month`; `Policy` required (`honor_prepaid` or `charge_difference`);
   `recorded_at` = now.
7. Only the most recent rate change can be deleted, and only while its
   `effective_month > C`. The `initial` rate cannot be deleted.
8. Editing opening fields, editing or deleting a payment, and adding or deleting a rate
   change all take effect through recomputation; nothing else is updated.

## Acceptance Criteria

- [ ] Rate 150; a house owing sep and oct pays 450 on 12 nov → sep, oct, nov covered; arrears 0; prepaid 0.
- [ ] Opening may 2026 with opening arrears 150 (april); the house pays 360 → arrears cleared, may covered, jun partial 60; a later 90 completes jun (spreadsheet Casa 03 case).
- [ ] Opening covered through jan 2027: in nov 2026 the house shows `Al corriente`, prepaid 300 (dec, jan), with no payment recorded.
- [ ] Opening arrears 600 and a payment of 750 → arrears cleared, opening month covered.
- [ ] A house with no payments and no opening coverage owes every month from the opening month through the current month (Casa 08).
- [ ] Rate changes to 170 from jan 2027 with `honor_prepaid`: a house that had already paid through apr 2027 stays covered through apr 2027. With `charge_difference`: the same 1,800 paid in may 2026 covers may–dec 2026 at 150 and jan–mar 2027 at 170, leaving apr 2027 partial at 90 (spreadsheet Casa 04 case).
- [ ] Deleting a payment in the middle of a sequence re-flows the later payments into earlier months.
- [ ] Payment preview matches the coverage after saving.
- [ ] A third contact is rejected; deleting contact 1 promotes contact 2.
- [ ] A rate change with an effective month in the past is rejected.
- [ ] Property: for any opening values, rates, and payments, Σ Applied + arrears paid + unapplied = Σ credits; months are filled in order with at most one partial month per house at any time.
- [ ] Property: total of `Prepaid` across houses never exceeds total fee credits.
