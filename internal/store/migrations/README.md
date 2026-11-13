# Migrations

SQL files named `NNN_description.sql` (e.g. `001_setup.sql`), applied in filename
order at startup by `store.Migrate`, each in its own transaction, and recorded in
`schema_migrations`.

An applied file is never edited. A schema change is a new file.
Conventions: `docs/architecture.md` → Active Conventions → Data layer.
