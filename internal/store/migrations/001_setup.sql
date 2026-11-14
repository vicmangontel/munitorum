CREATE TABLE settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  community_name TEXT NOT NULL,
  house_prefix TEXT NOT NULL,
  house_count INTEGER NOT NULL CHECK (house_count BETWEEN 1 AND 200),
  opening_date TEXT NOT NULL,        -- YYYY-MM-DD
  setup_completed_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE accounts (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL UNIQUE COLLATE NOCASE,
  kind TEXT NOT NULL CHECK (kind IN ('cash','bank')),
  opening_balance_cents INTEGER NOT NULL CHECK (opening_balance_cents >= 0),
  sort_order INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE houses (
  id INTEGER PRIMARY KEY,
  number INTEGER NOT NULL UNIQUE,
  code TEXT NOT NULL UNIQUE,
  opening_covered_through TEXT NULL,  -- YYYY-MM
  opening_credit_cents INTEGER NOT NULL DEFAULT 0
    CHECK (opening_credit_cents >= 0),
  opening_arrears_cents INTEGER NOT NULL DEFAULT 0
    CHECK (opening_arrears_cents >= 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE fee_rates (
  id INTEGER PRIMARY KEY,
  effective_month TEXT NOT NULL UNIQUE,  -- YYYY-MM
  amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
  policy TEXT NOT NULL CHECK (policy IN
    ('initial','honor_prepaid','charge_difference')),
  recorded_at TEXT NOT NULL
);
