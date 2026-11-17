---
type: plan
phase: 2
module: directory
status: done
spec: docs/specs/directory.md
depends-on: phase-1
---

# Phase 2 — Directory

## Goal

The Directorio view lists the people, companies, and services the coto works
with; the treasurer creates, edits, searches, filters, deletes, and restores them,
and links a person to a company. A reusable picker lets later forms (expenses,
reminders, obras, fee payments) choose an item by typing, and create a vendor
inline without leaving the form. Case- and accent-insensitive matching lives in
one `internal/domain` helper that the ledger reuses for category names.

This is the first view that opens its own overlays and fills the detail panel, so
the view contract gains both (recorded in `docs/architecture.md` → TUI layer).

## Prerequisites

- Phase 1 Step 1 done: form and confirmation components, interactive overlay host,
  `app.ValidationError`.
- Approval of migration `002_directory.sql` before it is written.

## Domain and application tasks
- [x] Migration `internal/store/migrations/002_directory.sql`: `vendors` per the spec, index `vendors(kind, name)` (ask first)
- [x] `internal/domain`: `FoldKey` — lower case, NFD, combining marks dropped (`golang.org/x/text/unicode/norm`, already in the module graph); `José` and `jose` fold equal
- [x] `internal/domain`: vendor rules 1, 3, 4 (lengths, email shape, `CompanyID` only on persons, not self); display name `Name (Company)`; deleted suffix applied by callers through `text`
- [x] `app.DirectoryService` with its repository interface; `ListVendors` (kind, folded search over name, phone, email, company name; live only unless `IncludeDeleted`; ordered by name)
- [x] `GetVendor` with live members for a company and the company for a person
- [x] `CreateVendor` and `UpdateVendor`: rules 1–5, every invalid field at once; uniqueness (rule 2) checked inside the write transaction
- [x] `DeleteVendor` (rule 6: a company with live members is rejected with an error that lists them) and `RestoreVendor` (rule 2 checked again)
- [x] `internal/store`: SQLite implementation
- [x] Wide events `create_vendor`, `update_vendor`, `delete_vendor`, `restore_vendor`: `vendor_id`, `kind`; never names, phones, emails, notes
- [x] `cmd/devseed`: gardener and container-cleaning person, company `Portones Acme` with one technician, services `Basura` and `CFE`, one deleted vendor

## TUI tasks
- [x] View contract: an optional detail method rendered in the detail panel, and a message a view sends to open an overlay in the root; update `docs/architecture.md` → TUI layer (ask first)
- [x] `internal/tui/directory` replaces the placeholder: columns Nombre, Tipo, Empresa, Teléfono; `n` `e` `d` (confirm) `/` `f` `.` `r`; key hints
- [x] Detail panel: all fields; a company's people; a person's company
- [x] Vendor form overlay: kind selector (`Persona`, `Empresa`, `Servicio`); company field shown only for `Persona` and chosen with the picker
- [x] Picker component `internal/tui/picker`, generic over the items a caller loads: type to filter (folded), `enter` select, `+` create inline (opens the vendor form, returns the new item selected), `esc` no selection; deleted vendors never offered

## Test tasks
- [x] `internal/domain` table tests: `FoldKey`, vendor rules, display name
- [x] `internal/app` tests for every acceptance criterion: service with name only; person linked to a company and displayed `Name (Company)`; link to a service or person rejected; `jose` vs live person `José` rejected and company `José` allowed; company with members not deletable and members named; restore after a name collision rejected; search by company name; deleted vendor excluded from `ListVendors`
- [x] TUI tests: vendor form shows every error at once; picker filters while typing; `+` inline create returns the new vendor selected in a test host form; delete confirmation declined writes nothing

## Manual Test Scenarios

1. Create company `Portones Acme`, then person `Juan Gómez` linked to it: the list shows `Juan Gómez (Portones Acme)`; `/acme` finds both.
2. Try to delete `Portones Acme`: the message names Juan Gómez. Delete Juan, then the company; `.` shows both struck through; `r` restores them.
3. Create service `Basura` with no phone or email: saved.
4. Create person `jose`, then another person `José`: rejected with the field error.

## Definition of Done
- [x] `go build ./...` and `go vet ./...` clean
- [x] All new tests pass (`just test`)
- [x] Works end-to-end in Windows Terminal
- [x] `docs/specs/directory.md` up to date
- [x] `docs/architecture.md` Test Coverage Map row for directory (ask first)
- [x] `CLAUDE.md` Current State updated
