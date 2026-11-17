---
type: spec
module: directory
status: draft
entities: Vendor
operations: ListVendors, GetVendor, CreateVendor, UpdateVendor, DeleteVendor, RestoreVendor
depends-on: setup
---

# Directory — Specification

## Purpose

The directorio: the people, companies, and services the coto pays or works with —
the gardener, the container-cleaning lady, the gate technician and his company, the
garbage service that has no name or phone. Expenses, reminders, and obras pick
their payee (`De/Para`) from this list instead of free text, so spending per
vendor can be filtered and totalled.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `Vendor` (`vendors`) | `id`, `kind TEXT CHECK (kind IN ('person','company','service'))`, `name TEXT NOT NULL`, `phone TEXT NULL`, `email TEXT NULL`, `company_id INTEGER NULL REFERENCES vendors(id)`, `notes TEXT NULL`, `created_at`, `updated_at`, `deleted_at` | UI kinds: `Persona`, `Empresa`, `Servicio`. `company_id` only on persons. Soft-deleted. |

Migration: `internal/store/migrations/002_directory.sql`.
Index: `vendors(kind, name)`.

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `ListVendors` | Treasurer, pickers | `Kind` (optional), `Search` (optional), `IncludeDeleted` | Live only unless `IncludeDeleted`; ordered by name; search matches name, phone, email, company name (case- and accent-insensitive) | `[]VendorRow{ID, Kind, Name, CompanyName, Phone, Email, Deleted}` |
| `GetVendor` | Treasurer | `ID` | Also returns live persons belonging to a company, and a person's company; works for deleted vendors and for a person whose company is deleted | `VendorDetail{Vendor, Company, Members}` |
| `CreateVendor` | Treasurer, inline from a picker | `Kind`, `Name`, `Phone`, `Email`, `CompanyID`, `Notes` | Rules 1–5 | Created vendor |
| `UpdateVendor` | Treasurer | `ID` + fields | Rules 1–5; kind may change only if rule 4 still holds | Updated vendor |
| `DeleteVendor` | Treasurer | `ID` | Rule 6 | — |
| `RestoreVendor` | Treasurer | `ID` | Rule 2 checked again; rule 8 | — |

## Operations

Methods on `app.DirectoryService`, named as the use cases.

- Validation failures return `app.ValidationError` with every invalid field (keys `kind`,
  `name`, `phone`, `email`, `company_id`, `notes`); a rule-2 collision on restore is a
  `name` field error.
- Rules 6 and 8 return `app.VendorConflict{Reason, Names}` (matches `app.ErrConflict`),
  reasons `company_has_members` (rule 6), `company_deleted` and `company_not_company`
  (rule 8); `Names` are display names shown
  to the user, never logged.
- `UpdateVendor` and `DeleteVendor` on an unknown or deleted vendor, and `RestoreVendor`
  on an unknown or live vendor, return `app.ErrNotFound`.
- Lengths count characters, after trimming. Empty phone, email, and notes are stored as
  NULL. Name matching and search use `domain.FoldKey` (lower case, accents dropped),
  computed in Go; no folded column is stored.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Directorio (main panel) | List: Nombre, Tipo, Empresa, Teléfono | `n` nuevo · `e` editar · `d` eliminar (confirm) · `/` buscar · `f` filtrar tipo · `.` mostrar eliminados · `r` restaurar |
| Detalle (detail panel) | All fields; for a company, its people; for a person, its company | — (ledger adds `m` ver movimientos) |
| Formulario de directorio (overlay) | Create / edit | Tab between fields; company field is a picker shown only for `Persona` |
| Picker de directorio (component) | Used by expense, reminder, and obra forms | Type to filter · Enter select · `+` crear nuevo inline · Esc no vendor |

- `/` filters the list as the user types; `f` cycles Todos → Persona → Empresa →
  Servicio; `.` shows deleted vendors struck through; `r` restores the selected row
  only when it is deleted.
- Delete asks for confirmation first; a rule-6 refusal naming the members shows in
  the overlay and any key closes it.
- The form sends no `CompanyID` unless the kind is `Persona`. The company field's
  picker `+` opens the vendor form with the kind fixed to `Empresa`; the new company
  comes back selected. `supr` clears the company field. After a save the vendor is
  selected in the list.

Display name everywhere: a person with a company shows as `Juan Gómez (Portones Acme)`.
A deleted vendor still shows on old movements with ` (eliminado)` appended.

## Business Rules

1. `Name` required, trimmed, 1–80 characters. `Phone` ≤ 30 characters, free text.
   `Email` optional; when present must contain one `@` with text on both sides.
   `Notes` ≤ 500 characters.
2. Names are unique among live vendors of the same kind, case- and accent-insensitive.
3. A `service` may have only a name (e.g. `Basura`).
4. `CompanyID` is allowed only when `Kind = person` and must reference a live vendor of
   kind `company`. A vendor cannot be its own company.
5. Changing a company's kind away from `company` is rejected while live persons
   reference it.
6. Deleting a company is rejected while live persons reference it (the message names
   them). Deleting any other vendor sets `deleted_at`; existing movements, reminders,
   and obras keep the reference.
7. Deleted vendors are not offered in pickers.
8. Restoring a person is rejected unless their `CompanyID` (when set) references a live
   vendor of kind `company` — e.g. the company is deleted, or was changed to another
   kind while the person was deleted. The message names that vendor and the link is
   kept; restore the company (or fix its kind) first.

## Acceptance Criteria

- [ ] A service can be saved with only a name.
- [ ] A person can be linked to a company; the person displays as `Name (Company)`.
- [ ] Linking a person to a service or another person is rejected.
- [ ] Creating `jose` when a live person `José` exists is rejected; a company `José` is allowed.
- [ ] Deleting a company with live members is rejected and names them.
- [ ] A deleted vendor disappears from pickers but still shows on its old movements with `(eliminado)`.
- [ ] Restoring a vendor whose name now collides with a live vendor is rejected.
- [ ] Search finds a person by their company's name.
- [ ] A vendor created inline from a picker is selected in the form that opened it.
- [ ] Restoring a person whose company is deleted, or is no longer a company, is rejected and names it.
