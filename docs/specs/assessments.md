---
type: spec
module: assessments
status: draft
entities: Assessment, AssessmentPayment (movement), FundDraw (movement fields), Project
operations: ListAssessments, GetAssessment, CreateAssessment, UpdateAssessment, CloseAssessment, ReopenAssessment, DeleteAssessment, RecordAssessmentPayment, UpdateAssessmentPayment, FundBalances, ListProjects, GetProject, CreateProject, UpdateProject, DeleteProject
depends-on: houses, ledger, directory
---

# Assessments — Specification

## Purpose

Two related things that the spreadsheet kept in a separate "Extraordinarias" sheet,
outside the balances:

- **Cooperaciones extraordinarias** — money in. A one-time charge to every casa
  for a purpose (e.g. $717 for repairs), paid in instalments. Its money sits in the
  same accounts but forms a fund that is not available for ordinary spending until
  it is spent or released.
- **Obras** — money out. A job with a vendor and an agreed cost, paid in parts
  (anticipos, then liquidación). An obra may be paid from regular money, from a
  fund, or from both.

They meet only in an expense: a payment to an obra (or any expense) can take part
of its amount from a cooperación fund, and the rest from regular funds.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Assessment` (`assessments`) | `id`, `name TEXT NOT NULL`, `amount_per_house_cents INTEGER NOT NULL CHECK (> 0)`, `charge_date TEXT NOT NULL`, `purpose TEXT NULL`, `status TEXT CHECK (status IN ('open','closed'))`, `closed_at TEXT NULL`, `created_at`, `updated_at`, `deleted_at` | Charged to every house. Soft-deleted. |
| Assessment payment | `movements` row: `kind = 'income'`, category with `system_key = 'assessment'`, `house_id`, `assessment_id` | Instalments allowed. |
| Fund draw | On an expense `movements` row: `fund_assessment_id`, `fund_amount_cents` | Portion of the expense paid from the fund. |
| `Project` (`projects`) | `id`, `name TEXT NOT NULL`, `vendor_id INTEGER NULL REFERENCES vendors(id)`, `agreed_cost_cents INTEGER NULL`, `status TEXT CHECK (status IN ('open','finished','cancelled'))`, `description TEXT NULL`, `created_at`, `updated_at`, `deleted_at` | UI statuses: `Abierta`, `Terminada`, `Cancelada`. Agreed cost may be unknown at first. Soft-deleted. |
| Project payment | Expense `movements` row with `project_id` | Shown as `Anticipo` while the obra is open. |

Migration: `internal/store/migrations/005_assessments.sql` — `assessments`, `projects`;
`ALTER TABLE movements ADD COLUMN` `assessment_id`, `project_id`, `fund_assessment_id`
(all `INTEGER NULL REFERENCES`), `fund_amount_cents INTEGER NULL`; indexes on the three
id columns. Cross-column rules are enforced in `internal/app` (SQLite cannot add
table checks by `ALTER`).

### Derived values (domain)

- Per assessment: `Charged` = amount × house count; `Collected` = Σ live payments;
  `Spent` = Σ `fund_amount_cents` of live expenses drawing from it;
  `Remaining` = Collected − Spent; `Owed` = Charged − Collected.
- Per house and assessment: `Paid`, `Owed` = amount − Paid.
- `Reserved` (used by dashboard rule *Available*) = Σ `Remaining` of **open** assessments.
  Closing an assessment releases its remaining money into available funds.
- Per project: `Paid` = Σ live linked expenses; `Remaining` = agreed − Paid (when agreed
  cost is set); `FromFunds` = Σ fund portions of those expenses.

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `ListAssessments` | Treasurer | `IncludeClosed` | — | rows with Charged, Collected, Spent, Remaining, houses paid in full / total |
| `GetAssessment` | Treasurer | `ID` | — | per-house status, payments, expenses drawing from it |
| `CreateAssessment` | Treasurer | `Name, AmountPerHouse, ChargeDate, Purpose` | Rule 1 | created (`open`) |
| `UpdateAssessment` | Treasurer | `ID` + fields | Rule 1; rule 3 | updated |
| `CloseAssessment` | Treasurer | `ID` | Rule 4 | closed |
| `ReopenAssessment` | Treasurer | `ID` | — | open |
| `DeleteAssessment` | Treasurer | `ID` | Only with no live payments or draws | — |
| `RecordAssessmentPayment` | Treasurer | `AssessmentID, HouseID, Date, AccountID, Amount, Notes, ReceiptLink, ManualReceiptPending` | Ledger rules 1–3, 10; rules 2, 5 | movement |
| `UpdateAssessmentPayment` | Treasurer | `ID` + fields | Rules 2, 3, 5 | movement |
| `FundBalances` | Dashboard, expense form, reports | `AsOf` | Derived | per assessment + `Reserved` |
| `ListProjects` / `GetProject` | Treasurer | status filter / `ID` | — | rows / detail with payments |
| `CreateProject` / `UpdateProject` | Treasurer | `Name, VendorID?, AgreedCost?, Status, Description` | Rule 6 | project |
| `DeleteProject` | Treasurer | `ID` | Only with no live linked expenses | — |

**Expense form section** (contributed to the ledger's expense form; validated by
`RecordExpense` / `UpdateExpense` through an `app` hook):
`Obra` (picker of open projects, optional) and `Pagar de cooperación` (picker of open
assessments with Remaining > 0, optional) with `Monto de cooperación`
(default: the lower of the expense amount and the fund's remaining). Rules 3, 7.

## Operations

Methods on `app.AssessmentService` and `app.ProjectService`, named as the use cases.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Cooperaciones (main panel) | Columns: Nombre, Monto por casa, Cobrado, Gastado, Disponible del fondo, Casas pagadas (e.g. `15/18`), Estado | `n` nueva · `e` editar · `p` registrar pago · `enter` detalle · `x` cerrar/reabrir · `.` mostrar cerradas |
| Cooperación — detalle | Per-house table: Casa, Pagado, Debe; payments; expenses drawn from the fund | `p` pago para la casa seleccionada |
| Pago de cooperación (overlay) | Cooperación, casa, fecha, cuenta, monto (default: what the house still owes), notas, enlace, recibo manual pendiente | all errors at once |
| Obras (main panel) | Columns: Obra, Proveedor, Costo acordado, Pagado, Restante, De fondos, Estado | `n` nueva · `e` editar · `g` registrar pago (opens expense form with the obra pre-selected) · `enter` detalle |
| Obra — detalle | Payments in date order with fund portions; remaining | — |
| Estado de cuenta extension | Section `Cooperaciones` in the houses statement: per assessment, paid and owed | — |

Closing an assessment with remaining money asks for confirmation showing the amount
that becomes available.

## Business Rules

1. Assessment: `Name` 1–60 characters, unique among live assessments; `AmountPerHouse`
   > 0; `ChargeDate ≥ settings.opening_date`. `Purpose` ≤ 500.
2. A payment requires an `open` assessment and a house. `Amount` ≤ what that house
   still owes on that assessment (no overpayment, no credit carry-over).
3. No operation may leave any assessment with `Remaining < 0` or a house with
   `Paid > AmountPerHouse` — this applies to creating, editing, deleting, and restoring
   payments and expenses, and to lowering `AmountPerHouse`. The operation is rejected
   with a message naming the assessment.
4. Closing sets `status = 'closed'`. A closed assessment accepts no payments and no
   new draws; its remaining money stops being reserved. Houses still owing are shown
   but not counted in dashboard arrears.
5. Assessment payments are income in the `Extraordinaria` category; they count toward
   account balances and are reserved until spent or released.
6. Project: `Name` 1–80 characters; `AgreedCost` ≥ 0 when present; vendor must be a live
   directory entry. `Paid > AgreedCost` is allowed and shown with the Warning colour
   (`Excedido`).
7. Expense fund portion: 0 < `fund_amount_cents` ≤ expense amount, and ≤ the
   assessment's Remaining (plus this expense's own current draw when editing).
   `fund_assessment_id` and `fund_amount_cents` are both set or both null. Only
   expenses can draw; only open projects can be linked on create.

## Acceptance Criteria

- [ ] Creating a 717.00 assessment with 18 houses shows Charged 12,906.00 and every house owing 717.00.
- [ ] A house pays 300 then 417 → paid in full; a third payment is rejected.
- [ ] Paying an assessment raises the account balance but not Available; Reserved rises by the same amount.
- [ ] An expense of 5,200 for the obra "Portón" drawing 3,000 from the fund leaves fund Remaining reduced by 3,000 and Available reduced by 2,200.
- [ ] A draw larger than the fund's remaining is rejected.
- [ ] Deleting an assessment payment whose money was already spent (Remaining would go negative) is rejected.
- [ ] Closing an assessment with 500 remaining moves 500 into Available after confirmation.
- [ ] An obra with agreed cost 5,200 and payments 1,000 + 4,200 shows Restante 0; a further 500 shows Excedido.
- [ ] An obra can be created with no agreed cost and no vendor.
- [ ] The houses estado de cuenta shows each assessment's paid and owed amount for that house.
- [ ] Property: for any sequence of payments and draws accepted by the rules, every assessment's Remaining ≥ 0 and Σ Remaining (open) = Reserved.
