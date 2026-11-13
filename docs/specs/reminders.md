---
type: spec
module: reminders
status: draft
entities: Reminder, ReminderOccurrence
operations: ListReminders, CreateReminder, UpdateReminder, SetReminderActive, DeleteReminder, ListDueReminders, PayReminder, MarkReminderDone, SkipReminder, UndoReminderOccurrence
depends-on: ledger, directory
---

# Reminders — Specification

## Purpose

A calendar of recurring payments — Telcel on the 7th, gardening, garbage,
container cleaning, CFE every two months — without an automatic scheduler.
When a reminder is due, the treasurer selects it and gets an expense form
already filled with payee, category, account, amount, and description, changes
whatever differs this time, and saves. Nothing is ever posted automatically.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Reminder` (`reminders`) | `id`, `name TEXT NOT NULL`, `interval_months INTEGER CHECK (BETWEEN 1 AND 12)`, `start_month TEXT NOT NULL` (`YYYY-MM`), `day_of_month INTEGER CHECK (BETWEEN 1 AND 31)`, `vendor_id NULL REFERENCES vendors(id)`, `category_id NOT NULL REFERENCES categories(id)`, `account_id NULL REFERENCES accounts(id)`, `amount_cents INTEGER NULL`, `description TEXT NOT NULL`, `active INTEGER NOT NULL DEFAULT 1`, `created_at`, `updated_at`, `deleted_at` | Soft-deleted. `amount_cents` null when the amount varies (CFE). |
| `ReminderOccurrence` (`reminder_occurrences`) | `id`, `reminder_id NOT NULL`, `month TEXT NOT NULL`, `status TEXT CHECK (status IN ('done','skipped'))`, `movement_id INTEGER NULL REFERENCES movements(id)`, `created_at` | `UNIQUE (reminder_id, month)`. Hard-deleted on undo (state, not business record). |

Migration: `internal/store/migrations/006_reminders.sql`.

### Derived (domain)

- **Applicable months:** `start_month + k × interval_months`, k ≥ 0.
- **Due date** of an applicable month: `day_of_month`, clamped to the month's last day.
- **Occurrence state** for an applicable month: `done` or `skipped` if an occurrence row
  exists — except a `done` row whose linked movement is deleted counts as pending
  again; otherwise `pending`.
- **Due list (as of today):** active, live reminders; every applicable month from
  `max(start_month, opening month)` through the current month whose state is
  `pending` and whose due date ≤ today. Past months stay listed (`Vencido`) until done
  or skipped.
- **Upcoming list:** pending occurrences with due date in the next 7 days.

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `ListReminders` | Treasurer | `IncludeInactive` | — | rows with next due date and last done month |
| `CreateReminder` / `UpdateReminder` | Treasurer | `Name, IntervalMonths, StartMonth, DayOfMonth, VendorID?, CategoryID, AccountID?, Amount?, Description` | Rules 1–2 | reminder |
| `SetReminderActive` | Treasurer | `ID, Active` | Inactive reminders are never due | — |
| `DeleteReminder` | Treasurer | `ID` | Soft delete; occurrences kept | — |
| `ListDueReminders` | Dashboard, Recordatorios view | `AsOf` | Derived | `Due[]`, `Upcoming[]` with reminder, month, due date |
| `PayReminder` | Treasurer | `ReminderID, Month` + expense fields (pre-filled, editable) | Rule 3; ledger expense rules | movement + `done` occurrence, one transaction |
| `MarkReminderDone` | Treasurer | `ReminderID, Month` | For payments already recorded by hand; no movement | occurrence |
| `SkipReminder` | Treasurer | `ReminderID, Month` | — | occurrence |
| `UndoReminderOccurrence` | Treasurer | `ReminderID, Month` | Deletes the occurrence; a linked movement is not deleted | — |

## Operations

Methods on `app.ReminderService`, named as the use cases. `PayReminder` calls the
ledger's expense recording inside the same transaction.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Recordatorios (main panel) | Two sections: `Por pagar` (due and overdue, oldest first, overdue in Warning colour) and `Todos` (Nombre, Cada, Día, Proveedor, Monto, Próximo) | `enter` pagar (opens pre-filled expense form) · `h` marcar hecho sin registrar · `s` omitir este mes · `u` deshacer · `n` nuevo · `e` editar · `space` activar/desactivar · `d` eliminar |
| Formulario de recordatorio (overlay) | Fields above; `Cada` shown as `1 mes`, `2 meses`… | all errors at once |
| Expense form, pre-filled | Date = today; payee, category, account, amount, description `{description} {mes}` (e.g. `Recarga interfón nov 2026`) | Saving marks the occurrence done |

The view-list entry shows the due count: `Recordatorios (2)`.

## Business Rules

1. `Name` 1–60 characters, unique among live reminders. `Description` 1–100.
   `IntervalMonths` 1–12. `DayOfMonth` 1–31. `StartMonth` ≥ opening month.
2. Category must be an active expense category; vendor and account, when set, must be
   live. `Amount` > 0 when set.
3. `PayReminder` requires the month to be applicable for the reminder and not already
   `done` or `skipped`. Movement and occurrence are written together or not at all.
4. Changing `IntervalMonths` or `StartMonth` does not delete existing occurrences; months
   that are no longer applicable are simply no longer listed.

## Acceptance Criteria

- [ ] A monthly reminder for day 7 is not due on the 6th and is due on the 7th.
- [ ] Day 31 in a 30-day month is due on the 30th; in February on its last day.
- [ ] A bimonthly CFE reminder starting aug 2026 is due in aug, oct, dec — not sep or nov.
- [ ] Paying from a reminder creates one expense with the edited values and marks the month done; the reminder leaves the due list.
- [ ] A reminder left unpaid in october is still listed as overdue in november, alongside november.
- [ ] Skipping removes the month from the due list; undo brings it back.
- [ ] Deleting the movement created by a reminder makes that month due again.
- [ ] A reminder with no default amount opens the form with the amount empty and requires it.
- [ ] An inactive reminder is never due.
- [ ] A reminder whose start month is before today's month lists every missed month (backfilling missing Telcel recharges).
