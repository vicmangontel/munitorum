# Munitorum — Docs Index

> Read this file first. It tells you exactly which documents to open for any task.
> Update it whenever a spec or plan file is added, renamed, or its status changes.

## Agent context loading protocol

1. Read `CLAUDE.md` when it exists, then this file — together under 5 minutes of context.
2. Then open only the files listed in the task routing table below that match your task.
3. Do not read docs not listed for your task — unnecessary context wastes tokens.

## Task routing

| If you need to... | Read |
|---|---|
| Understand any architecture or convention | `docs/architecture.md` |
| Understand the full product scope | `docs/prd.md` |
| Work on a specific module | `docs/specs/{module}.md` + its plan if listed below; otherwise plan it from `docs/roadmap.md` first |
| Find the next phase to plan | `docs/roadmap.md` → first row with Plan `—`; run `/noos-new-project` Phase 5 |
| Add a DB migration | `docs/architecture.md` → Active Conventions → Data layer |
| Add a new use case to an existing module | `docs/specs/{module}.md` for the contract |
| Map a Spanish UI term to a code name | `docs/architecture.md` → Domain Glossary |
| Check what is already built vs. planned | `CLAUDE.md` → Current State when present; otherwise `docs/roadmap.md` |
| Understand the implementation sequence | `docs/roadmap.md`; this file → Implementation plans |
| Change an architecture decision | `docs/architecture.md` → update it → update `CLAUDE.md` |

## Core documents

| File | Purpose |
|---|---|
| `CLAUDE.md` | Project rules, naming conventions, commands, current state |
| `docs/architecture.md` | All decisions, stack and layout, glossary, conventions, design system, accepted risks |
| `docs/prd.md` | Full product requirements — features, business rules, workflows, non-functional requirements |
| `docs/roadmap.md` | Agreed phase sequence, dependencies, and plan creation progress |
| `docs/index.md` | This file — document navigator |

## Module specs

| File | Module | Entities | Status |
|---|---|---|---|
| `docs/specs/setup.md` | Setup | Settings, Account, House, FeeRate | draft |
| `docs/specs/directory.md` | Directory | Vendor | draft |
| `docs/specs/ledger.md` | Ledger | Movement, Category | draft |
| `docs/specs/houses.md` | Houses | House, HouseContact, FeeRate, FeePayment, MonthStatus, HouseStatement | draft |
| `docs/specs/assessments.md` | Assessments | Assessment, AssessmentPayment, FundDraw, Project | draft |
| `docs/specs/reminders.md` | Reminders | Reminder, ReminderOccurrence | draft |
| `docs/specs/dashboard.md` | Dashboard | Todo, DashboardSummary | draft |
| `docs/specs/reports.md` | Reports | MonthlyReport | draft |

## Implementation plans

| File | Phase | Module | Depends on | Status |
|---|---|---|---|---|
| `docs/plans/phase-0-setup.md` | 0 | Setup (project) | — | not-started |

## Change plans

| File | Type | Module | Status | Date |
|---|---|---|---|---|

## Source material

| File | Purpose |
|---|---|
| `docs/Coto Lagos 2025-2026.xlsx` | The spreadsheet Munitorum replaces; history from May 2026 is entered by hand from it |
