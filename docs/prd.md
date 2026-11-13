# Munitorum — Product Requirements Document

## Overview

Munitorum is a local, single-user terminal application (keyboard-driven, in the
style of lazygit) for the treasurer of a Mexican residential community
(*fraccionamiento* / *coto*). It replaces the "Coto Lagos 2025-2026" spreadsheet.

It records monthly household quotas (*cuotas*), one-time special assessments
(*cooperaciones extraordinarias*), expenses, transfers between accounts, and cash
adjustments. It always shows the real balance per account, how much of that
money is available this month versus paid in advance for future months or
reserved in a cooperación fund, which months each casa owes, and which recurring
payments are due. It exports a monthly Excel report.

The spreadsheet it replaces has running-balance formulas that skip rows, a
dashboard offset by one month, and special-assessment money kept outside the
ledger. Munitorum derives every balance from recorded movements, so totals
cannot drift.

The interface is in Spanish. The only currency is MXN.

## Users and Roles

| Role | Responsibilities | Permissions level |
|---|---|---|
| Tesorero (treasurer) | Sets up the coto, records all income, expenses, transfers and adjustments, manages casas, contacts and directory, exports reports | Full access. Sole user. No login in v1. |

No external party uses the application. Residents receive exported Excel reports
outside the app, and the treasurer answers "what do I owe?" questions from the
casa statement screen.

## Domain Vocabulary

| Term | Meaning |
|---|---|
| Casa | A household unit. Identified by a generated code (`CASA 01` … `CASA 18`). |
| Cuota | The recurring monthly charge per casa. Amount has an effective-dated history. |
| Mes cubierto | A month of cuota fully paid by a casa. |
| Adelanto | Cuota money received for months after the current month. |
| Adeudo | Cuota owed for the current or past months that is not fully paid, plus any opening adeudo. |
| Cooperación extraordinaria | A one-time special assessment charged to each casa. Its money forms a fund inside the same accounts. |
| Obra | A piece of work with a vendor and an agreed cost, paid in one or more payments (anticipos and liquidación). |
| Anticipo | An advance payment to a vendor before the work is complete. |
| Apertura | The one-time opening: balances and covered months received from the previous administrator. |
| Cuenta | A money location: `Efectivo` (cash) and `Banco (NU)` (debit). |
| Directorio | Directory of people, companies, and services the coto pays. |
| Recordatorio | A recurring reminder (every N months) for a payment, which pre-fills an expense. |
| Recibo manual | A paper receipt the treasurer writes by hand for income. Tracked as pending/done. |
| Ajuste | A movement that corrects an account balance to the counted amount (arqueo). |
| Pendiente | A free-text to-do note for the treasurer. |
| Estado de cuenta | Per-casa screen: months covered and owed, payments, cooperación status. |

## Features — In Scope (v1)

1. **First-run setup wizard** (clean database only)
   - Coto name, number of casas, code prefix (`CASA `, `CASA-`, `C-`, …); casas are
     generated with zero-padded numbers (`CASA 01` … `CASA 18`).
   - Initial monthly cuota amount.
   - Accounts: `Efectivo` and `Banco (NU)` created by the wizard (names editable).
   - Apertura: opening date (defines the apertura month), opening balance per
     account, and per casa: last month already covered (or none), optional
     partial credit, optional opening adeudo.
   - Summary and confirm; written in one transaction.
2. **Casas and contacts**
   - Casa list: covered through month, months owed, adeudo amount, adelanto amount.
   - Up to 2 contacts per casa; one by default. Fields: name (required), phone, email.
   - Opening adeudo editable after setup (for casas whose prior debt is unknown at setup).
   - No casas added or removed after setup in v1.
3. **Estado de cuenta (casa statement screen)** — month-by-month cuota status
   (covered, partial, owed) from the apertura month to the furthest covered month;
   payment history; cooperación status (owed, paid, remaining). In-app only, no export.
4. **Cuota rate changes** — set a new amount and the month it takes effect. Each
   change records whether months already paid in advance keep the old rate
   (`respetar adelantos`) or owe the difference (`cobrar diferencia`). The
   community votes on this per change.
5. **Cuota payments**
   - Record a payment: casa, date, account, amount, notes, recibo-manual flag,
     optional receipt link.
   - Applied to months oldest-unpaid first; partial months allowed; surplus covers
     future months (adelanto).
   - Preview of months covered before saving. Description generated from the
     months covered ("Pago sep–nov 2026").
6. **Cooperaciones extraordinarias**
   - Create one: name, amount per casa, date, purpose.
   - Record casa payments toward it, in instalments, into either account.
   - Fund view: charged, collected, spent, remaining; casas that have not paid in full.
7. **Expenses**
   - Date, category, payee (from the directory, optional), description, account,
     amount, notes, optional receipt link.
   - Optional link to an obra.
   - Funding split: the portion paid from a cooperación fund (up to its remaining
     balance) and the portion paid from regular funds.
8. **Obras** — obra with vendor (directory entry), agreed cost, status
   (`abierta`, `terminada`, `cancelada`); payments are expenses linked to the obra;
   view of paid vs remaining. An obra may be paid from regular funds, one or more
   cooperación funds, or both.
9. **Transfers** — one action moves money between accounts.
10. **Other income** — income that is not a cuota or cooperación.
11. **Cash count (arqueo) and adjustments** — enter the counted amount for an
    account; the app records an `Ajuste` movement for the difference, with a note.
12. **Directory (Directorio)** — entries of kind person (`persona`), company
    (`empresa`), or service (`servicio`). A person may belong to a company.
    Fields: name (required), phone, email, notes.
13. **Categories** — editable in the app. Defaults:
    income `Cuota`, `Extraordinaria`, `Otros ingresos`;
    expense `Insumos`, `Servicios`, `Mtto preventivo`, `Reparaciones`.
    `Cuota` and `Extraordinaria` are system categories (renamable, not deletable).
    Transfers, apertura, and ajustes are movement types, not categories.
14. **Recordatorios** — name, interval in months (1 = monthly, 2 = bimonthly),
    start month, day of month, default payee, category, account, amount,
    description. Due on or after that day in each applicable month; selecting it
    opens a pre-filled expense form; saving marks it done for that month; it can
    be skipped for the month; it stays due until done or skipped.
15. **Dashboard** — balance per account and total; breakdown of the total into
    available, adelantos, and cooperación funds; current-month income and
    expenses; casas with adeudo and total adeudo; due recordatorios; pending
    manual receipts; open pendientes.
16. **Ledger (Libro de caja)** — chronological list of all movements with running
    balance per account and in total; filter by month, account, category, casa,
    payee; edit and delete any entry at any time, with all derived values recomputed.
    Past-dated entries allowed (used to load May–November 2026 history by hand).
17. **Monthly Excel export** — workbook in Spanish with:
    (a) month ledger, (b) cuota grid — status per casa for each month of the
    year to date, (c) totals by category, (d) opening and closing balance per
    account, (e) cooperación status sheet when a cooperación has activity or
    remaining balance.
18. **Manual receipt tracking** — on income movements only; pending count; mark done.
19. **Pendientes** — free-text to-do notes with done flag.

## Features — Out of Scope (v1)

- Authentication, multiple users, roles.
- Adding accounts after setup; adding or removing casas after setup.
- Receipt images or file attachments (only a plain-text link).
- Automatic posting of recordatorios.
- Vendor payables beyond obra remaining balance.
- Sending messages (email, WhatsApp) from the app.
- Multiple cotos in one database.
- Sync, cloud, or remote access.
- Cuota discounts (annual payment is 12 × monthly rate).
- Per-casa statement export.
- Importing the existing spreadsheet (history is entered by hand).
- Currencies other than MXN; languages other than Spanish.
- In-app backup (the treasurer copies the database file manually).

## Business Rules

1. **Month due.** Every casa owes the cuota rate in effect for each month from the
   apertura month onward. A month is due from its 1st day — no grace period.
2. **Coverage.** A month is covered when the amount applied to it equals its rate.
   Partially paid months are shown as partial.
3. **Allocation.** A cuota payment pays any opening adeudo first, then months
   oldest-unpaid first, including future months. Example: in November a casa
   owing September and October pays $450 — September, October and November
   become covered.
4. **Recalculation.** Editing or deleting a cuota payment, or adding a rate change,
   recomputes that casa's allocation from its payments in date order.
5. **Rate changes and adelantos.** With `respetar adelantos`, months already fully
   covered when the change is recorded keep the old rate. With
   `cobrar diferencia`, those months become partial and owe the difference.
6. **Adelanto.** Cuota money applied to months after the current month.
   It becomes available on the 1st of the month it covers.
7. **Fund.** A cooperación's remaining balance = collected − expense portions drawn
   from it. An expense cannot draw more than the remaining balance.
8. **Available.** Available = total of all account balances − adelantos −
   remaining balance of every open cooperación. Closing a cooperación releases its
   remaining money into available funds.
9. **Balances.** An account balance is the sum of its movements, including apertura
   and ajustes. Balances are derived, never stored as editable values.
10. **Negative balance.** An expense or transfer that makes an account negative
    requires confirmation; it is not blocked.
11. **Transfer.** A transfer is a single movement with source and destination
    accounts; it changes neither available funds nor category totals.
12. **Money.** All amounts are MXN with two decimals.

## Key Workflows

### First-run setup
1. Treasurer launches Munitorum with no database; the wizard starts.
2. Enters coto name, casa count (18), prefix (`CASA `), cuota ($150).
3. Confirms account names (`Efectivo`, `Banco (NU)`).
4. Enters apertura date (2 May 2026) and opening balance per account.
5. For each casa, enters the last covered month, optional partial credit, optional opening adeudo.
6. Reviews a summary and confirms.

### Load history (one-time, by hand)
1. Treasurer enters the May–November 2026 movements from the spreadsheet with
   their original dates, then adds missing recurring payments (Telcel, Basura).
2. Runs an arqueo per account to reconcile with the real cash and bank balance.

### Record a cuota payment
1. From the casas list or ledger, press the payment key; choose casa.
2. Enter date (default today), account, amount.
3. The app previews the months the amount covers and any adelanto.
4. Optionally set recibo manual pending, a receipt link, and a note; confirm.
5. Casa status, balances, and dashboard update.

### Answer "what do I owe?"
1. Open the casa; the estado de cuenta shows owed months, partial months, and
   cooperación remaining, with totals.

### Pay a due recordatorio
1. Dashboard lists "Telcel — día 7".
2. Select it; an expense form opens pre-filled.
3. Adjust any value; save. The recordatorio is marked done for this month.

### Pay an obra partly from a cooperación
1. Record an expense, link the obra, select the cooperación fund.
2. Enter the amount from the fund (up to its remaining balance); the rest comes from regular funds.
3. Fund remaining, obra remaining, and available funds update.

### Monthly close-out
1. Run an arqueo on `Efectivo` if needed.
2. Open the report screen, choose the month, export the Excel file to a chosen folder.

## Error Handling

- Every form validates before saving and shows all field errors at once.
- Negative account balance after an expense or transfer: warning plus confirmation.
- A cooperación portion larger than the fund's remaining balance is rejected.
- Saves are atomic: a payment and its month allocation are written together or not at all.
- Excel export failure (folder not writable, file open in Excel) shows the cause; no partial file is left.
- Wizard aborted midway leaves no rows; it restarts on next launch.
- Database file missing or unreadable at launch: clear message with the path; never silently start a new wizard over an existing file that failed to open.

## Non-Functional Requirements

- **Performance:** single user; ~20 casas; a few hundred movements per year. Every screen renders under 100 ms.
- **Offline:** fully offline. No network access.
- **Devices:** one Windows computer, Windows Terminal. Single executable.
- **Integrations:** none. Excel (.xlsx) export only.
- **Compliance:** none formal. Personal data (names, phones, emails) stays on the local disk.
- **Backup:** manual copy of the database file.
- **Data retention:** indefinite. Deleted entries are hidden, not erased.
- **Language and currency:** Spanish UI; MXN only.

## Future Scope (v2+)

- Add and manage additional accounts.
- In-app backup to a chosen folder.
- Open-source release (configurable language, generic setup).
