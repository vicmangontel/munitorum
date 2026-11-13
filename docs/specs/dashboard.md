---
type: spec
module: dashboard
status: draft
entities: Todo, DashboardSummary
operations: GetSummary, ListTodos, CreateTodo, UpdateTodo, SetTodoDone, DeleteTodo
depends-on: houses, assessments, reminders
---

# Dashboard — Specification

## Purpose

The Resumen view and the always-visible summary panel answer "how much money do
we have, and how much of it can we actually spend?" in one glance, and list what
needs attention: houses that owe, reminders that are due, manual receipts still
to write, and the treasurer's own pendientes. The module computes nothing new —
it combines the figures owned by ledger, houses, assessments, and reminders — and
owns the pendientes list.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Todo` (`todos`) | `id`, `text TEXT NOT NULL`, `done_at TEXT NULL`, `created_at`, `updated_at`, `deleted_at` | UI: `Pendiente`. Soft-deleted. |
| `DashboardSummary` | see below | Derived, as of today. |

Migration: `internal/store/migrations/007_todos.sql`.

### `DashboardSummary`

| Field | Source |
|---|---|
| Balance per account, total | `ledger.AccountBalances(today)` |
| Adelantos | `houses.FeeSummary(today).Prepaid` |
| Fondos de cooperación | `assessments.FundBalances(today).Reserved` |
| Disponible | Total − Adelantos − Fondos de cooperación |
| Month income / expense / adjustments / net | `ledger.ListMovements(current month).Totals` |
| Month income by category, month expense by category | same, grouped |
| Houses with arrears, total arrears | `houses.FeeSummary(today)` |
| Cooperación owed (open assessments) | `assessments` per-house owed, summed |
| Due and upcoming reminders | `reminders.ListDueReminders(today)` |
| Pending manual receipts (count, list) | `ledger.ListPendingReceipts` |
| Open pendientes | `ListTodos` |

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `GetSummary` | Resumen view, summary panel | — | Rules 1–2 | `DashboardSummary` |
| `ListTodos` | Pendientes view, dashboard | `IncludeDone` | Open first by `created_at`; done after, newest done first | rows |
| `CreateTodo` | Treasurer | `Text` | Rule 3 | todo |
| `UpdateTodo` | Treasurer | `ID, Text` | Rule 3 | todo |
| `SetTodoDone` | Treasurer | `ID, Done` | Sets or clears `done_at` | todo |
| `DeleteTodo` | Treasurer | `ID` | Soft delete | — |

## Operations

Methods on `app.DashboardService` and `app.TodoService`, named as the use cases.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Summary panel (left column, always visible) | Coto name as title; each account balance; Total; Disponible (Negative colour when below zero) | — |
| Resumen (main panel) | Blocks: **Dinero** (Total, Adelantos, Fondos de cooperación, Disponible); **Este mes** (Ingresos, Gastos, Ajustes, Neto, top categories); **Adeudos** (houses owing, total, cooperación owed); **Recordatorios** (due, overdue, next 7 days); **Recibos manuales pendientes** (count); **Pendientes** (open items) | `enter` on a block jumps to its view (Casas filtered by arrears, Recordatorios, Recibos pendientes, Pendientes) |
| Pendientes (main panel) | Open items; done items hidden unless toggled | `n` nuevo · `e` editar · `space` hecho/no hecho · `d` eliminar · `.` mostrar hechos |

The view list shows counts: `Recordatorios (2)`, `Pendientes (3)`. The summary panel and
Resumen refresh after every successful save in any view.

## Business Rules

1. Disponible = Total − Adelantos − Fondos de cooperación (open assessments only).
   A negative Disponible is shown, not hidden: it means money paid in advance or reserved
   has been spent.
2. All figures are computed as of today from live rows; nothing is cached across saves.
3. Todo `Text` required, trimmed, 1–200 characters.

## Acceptance Criteria

- [ ] With total 6,945.50, prepaid 1,200.00, and an open fund holding 500.00, Disponible shows 5,245.50.
- [ ] Closing that assessment raises Disponible by 500.00.
- [ ] Recording a payment in any view updates the summary panel without restarting.
- [ ] Disponible below zero is shown in the Negative colour.
- [ ] A due reminder appears in Resumen; paying it removes it.
- [ ] `enter` on Adeudos opens Casas filtered to houses with arrears.
- [ ] A pendiente marked done disappears from the open list and from the count; it reappears with `.`.
- [ ] Month totals in Resumen equal the ledger footer for the current month.
