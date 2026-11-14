# Munitorum — Implementation Roadmap

> Written by /noos-new-project. The sequence and dependency record. A dash in
> Plan means that the detailed plan has not been written yet. Plan status is
> recorded only in the plan file and docs/index.md.

Intent: B

| Phase | Module | Outcome | Depends on | Spec | Plan |
|---|---|---|---|---|---|
| 0 | Setup (project) | Shell runs: panels, view list, key bar, help; database created and migrated; file logging; money and month types tested | — | — | `docs/plans/phase-0-setup.md` |
| 1 | Setup (wizard) | Wizard creates the coto on a fresh database; Ajustes edits name, account names, opening balances; reusable form component | 0 | `docs/specs/setup.md` | `docs/plans/phase-1-setup.md` |
| 2 | Directory | People, companies, services; search picker with inline create | 1 | `docs/specs/directory.md` | `docs/plans/phase-2-directory.md` |
| 3 | Ledger | Income, expenses, transfers, arqueo, categories, running balance, filters, edit/delete/restore, manual receipts | 1, 2 | `docs/specs/ledger.md` | `docs/plans/phase-3-ledger.md` |
| 4 | Houses | Contacts, cuota payments with month preview, estado de cuenta, rate changes | 3 | `docs/specs/houses.md` | `docs/plans/phase-4-houses.md` |
| 5 | Assessments | Cooperaciones and funds, fund draws on expenses, obras | 4, 2 | `docs/specs/assessments.md` | — |
| 6 | Reminders | Due and overdue reminders, pay from pre-filled form, skip, undo | 3, 2 | `docs/specs/reminders.md` | — |
| 7 | Dashboard | Resumen, Disponible, pendientes | 4, 5, 6 | `docs/specs/dashboard.md` | — |
| 8 | Reports | Monthly Excel export | 3, 4, 5 | `docs/specs/reports.md` | — |

Phase 6 can run in parallel with 4–5; phase 8 in parallel with 7.
Phases 3, 4, and 5 are sized L and are expected to divide into steps when planned.
