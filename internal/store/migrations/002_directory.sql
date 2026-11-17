CREATE TABLE vendors (
  id INTEGER PRIMARY KEY,
  kind TEXT NOT NULL CHECK (kind IN
    ('person','company','service')),
  name TEXT NOT NULL,
  phone TEXT NULL,
  email TEXT NULL,
  company_id INTEGER NULL REFERENCES vendors(id),
  notes TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT NULL
);
CREATE INDEX idx_vendors_kind_name
  ON vendors(kind, name);
