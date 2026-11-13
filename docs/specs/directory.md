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
| `GetVendor` | Treasurer | `ID` | Also returns live persons belonging to a company | `VendorDetail` |
| `CreateVendor` | Treasurer, inline from a picker | `Kind`, `Name`, `Phone`, `Email`, `CompanyID`, `Notes` | Rules 1–5 | Created vendor |
| `UpdateVendor` | Treasurer | `ID` + fields | Rules 1–5; kind may change only if rule 4 still holds | Updated vendor |
| `DeleteVendor` | Treasurer | `ID` | Rule 6 | — |
| `RestoreVendor` | Treasurer | `ID` | Rule 2 checked again | — |

## Operations

Methods on `app.DirectoryService`, named as the use cases.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Directorio (main panel) | List: Nombre, Tipo, Empresa, Teléfono | `n` nuevo · `e` editar · `d` eliminar (confirm) · `/` buscar · `f` filtrar tipo · `.` mostrar eliminados · `r` restaurar |
| Detalle (detail panel) | All fields; for a company, its people; for a person, its company | — (ledger adds `m` ver movimientos) |
| Formulario de directorio (overlay) | Create / edit | Tab between fields; company field is a picker shown only for `Persona` |
| Picker de directorio (component) | Used by expense, reminder, and obra forms | Type to filter · Enter select · `+` crear nuevo inline · Esc no vendor |

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
