---
type: spec
module: reports
status: draft
entities: MonthlyReport
operations: BuildMonthlyReport, ExportMonthlyReport
depends-on: ledger, houses, assessments
---

# Reports — Specification

## Purpose

Produces the monthly Excel workbook the treasurer shares with the coto: what came
in, what went out, who has paid, and how much money is left in each account. All
figures come from the same derived values the screens use, so the report and the
app always agree. Amounts are real numeric cells so residents can add them up.

## Entities

| Entity | Key fields | Notes |
|---|---|---|
| `MonthlyReport` | `Month`, `AsOf` (last day of the month, or today for the current month), `Ledger`, `FeeGrid`, `CategoryTotals`, `AccountBalances`, `Assessments[]` | Built in `internal/app`, written by `internal/export`. Not stored. |
| Settings extension | `settings.export_dir TEXT NULL` | Last folder used. |

Migration: `internal/store/migrations/008_reports.sql` — `ALTER TABLE settings ADD COLUMN export_dir TEXT NULL`.

## Workbook

File name: `{coto-slug}-reporte-{YYYY-MM}.xlsx` (e.g. `coto-lagos-reporte-2026-11.xlsx`).
Every sheet starts with three header rows: coto name; `Reporte mensual {mes año}`
(with `— parcial al {fecha}` for the current month); `Generado el {fecha hora}`.
Then a blank row and the table. Header row bold with a filter; panes frozen below it.
Money cells numeric, format `"$"#,##0.00`; date cells real dates, format `dd/mm/yyyy`.

| # | Sheet | Content |
|---|---|---|
| 1 | `Libro de caja` | First row `Saldo inicial`. One row per live movement of the month: Fecha, Categoría, De/Para, Descripción, Cuenta, Ingreso, Egreso, Saldo, Notas. Transfers: Cuenta `Efectivo → Banco (NU)`, no Ingreso/Egreso, Saldo unchanged. Adjustments: Categoría `Ajuste (arqueo)`, amount in Ingreso or Egreso by sign. Last row `Saldo final` and column totals. |
| 2 | `Cuotas` | One row per house: Casa, Propietario, one column per month from the later of January of the report year and the opening month through the report month, then `Cubierto hasta`, `Adeudo`, `Adelanto`. Cell = amount applied to that month (blank when nothing). Fills: covered Positive, partial Warning, owed Negative (light tints). Totals row per month. Computed as of `AsOf`. |
| 3 | `Por categoría` | Income by category, total; expenses by category, total; adjustments; net. Only categories with movements in the month. |
| 4 | `Saldos` | Per account: Saldo inicial, Ingresos, Egresos, Transferencias recibidas, Transferencias enviadas, Ajustes, Saldo final; total row. Below: Total, Adelantos, Fondos de cooperación, Disponible, as of `AsOf`. |
| 5 | `Cooperaciones` | Included only when an assessment is open, or had payments or draws in the month. Per assessment: Monto por casa, Cobrado, Gastado, Restante, Estado; then per house: Casa, Pagado, Debe. |

## Use Cases

| Use case | Actor | Input | Business rules | Output |
|---|---|---|---|---|
| `BuildMonthlyReport` | Export, tests | `Month` | Rules 1–3 | `MonthlyReport` |
| `ExportMonthlyReport` | Treasurer | `Month`, `Folder`, `Overwrite` | Rules 1, 4–6 | `{Path}` or `FileExists` (when not `Overwrite`) |

## Operations

`app.ReportService.BuildMonthlyReport`, `app.ReportService.ExportMonthlyReport`;
workbook writing in `internal/export` (excelize), which receives a `MonthlyReport`
and knows nothing about the database.

## UI Views

| View | Purpose | Key interactions |
|---|---|---|
| Exportar reporte (overlay, `x` from any view) | Month picker (default: previous month during the first 10 days of a month, otherwise the current month), folder (default: last used, else the user's Documents folder), resulting file name | `←/→` mes · `tab` carpeta · `enter` exportar · confirmation when the file exists · on success shows the full path |

## Business Rules

1. `Month` between the opening month and the current month.
2. `AsOf` = last day of `Month`, or today when `Month` is the current month. Every figure
   uses live rows with `date ≤ AsOf` (houses and assessments models run as of `AsOf`).
3. `Saldo inicial` = balance at the end of the day before the month's first day (the
   opening balance for the opening month). `Saldo final` = Saldo inicial + month effects.
4. The file is written to a temporary file in the target folder and renamed into place;
   on any failure the temporary file is removed and no partial report remains.
5. Target file exists and `Overwrite` is false → `FileExists`; the UI asks. A target
   locked by Excel fails with the message `Cierra el archivo en Excel e intenta de nuevo`.
6. On success, `settings.export_dir` = folder.

## Acceptance Criteria

- [ ] Exporting November 2026 creates `coto-lagos-reporte-2026-11.xlsx` with sheets 1–4, plus 5 when an assessment is open.
- [ ] `Libro de caja` Saldo final equals the app's total balance at the end of the month; transfers do not change Saldo.
- [ ] `Saldos` per-account Saldo final equals `AccountBalances(AsOf)` for each account.
- [ ] `Cuotas` shows 150 in covered months, 60 in a partial month, blank in owed months, and Adeudo/Adelanto equal to the houses view as of `AsOf`.
- [ ] Money cells are numeric (a SUM in Excel works) and dates are real dates.
- [ ] Exporting a month with no movements produces the sheets with Saldo inicial = Saldo final.
- [ ] An existing file asks before overwriting; a file open in Excel produces the Spanish message and leaves the old file intact.
- [ ] The current month's report is labelled as partial.
- [ ] Reopened with excelize in tests: sheet names, header rows, and key totals match `BuildMonthlyReport`.
