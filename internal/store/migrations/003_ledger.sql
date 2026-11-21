CREATE TABLE categories (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  direction TEXT NOT NULL CHECK (direction IN ('income','expense')),
  system_key TEXT NULL UNIQUE CHECK (system_key IN ('fee','assessment')),
  active INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0,1)),
  sort_order INTEGER NOT NULL
);
INSERT INTO categories (name, direction, system_key, sort_order) VALUES
  ('Cuota',           'income',  'fee',        1),
  ('Extraordinaria',  'income',  'assessment', 2),
  ('Otros ingresos',  'income',  NULL,         3),
  ('Insumos',         'expense', NULL,         10),
  ('Servicios',       'expense', NULL,         11),
  ('Mtto preventivo', 'expense', NULL,         12),
  ('Reparaciones',    'expense', NULL,         13);

CREATE TABLE movements (
  id INTEGER PRIMARY KEY,
  date TEXT NOT NULL,                 -- YYYY-MM-DD
  kind TEXT NOT NULL CHECK (kind IN
    ('income','expense','transfer','adjustment')),
  account_id INTEGER NOT NULL REFERENCES accounts(id),
  to_account_id INTEGER NULL REFERENCES accounts(id),
  amount_cents INTEGER NOT NULL,
  category_id INTEGER NULL REFERENCES categories(id),
  vendor_id INTEGER NULL REFERENCES vendors(id),
  description TEXT NOT NULL,
  notes TEXT NULL,
  receipt_link TEXT NULL,
  manual_receipt TEXT NOT NULL DEFAULT 'none'
    CHECK (manual_receipt IN ('none','pending','done')),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  deleted_at TEXT NULL,
  CHECK (CASE WHEN kind = 'adjustment' THEN amount_cents <> 0
              ELSE amount_cents > 0 END),
  CHECK ((kind = 'transfer') = (to_account_id IS NOT NULL)),
  CHECK (to_account_id IS NULL OR to_account_id <> account_id),
  CHECK ((kind IN ('income','expense')) = (category_id IS NOT NULL)),
  CHECK (manual_receipt = 'none' OR kind = 'income')
);
CREATE INDEX idx_movements_date_id     ON movements(date, id);
CREATE INDEX idx_movements_account_id  ON movements(account_id);
CREATE INDEX idx_movements_category_id ON movements(category_id);
CREATE INDEX idx_movements_vendor_id   ON movements(vendor_id);
