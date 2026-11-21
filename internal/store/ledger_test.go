package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

// newLedgerDB returns a migrated database holding a cash account (id 1) and a
// bank account (id 2).
func newLedgerDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "l.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := Migrate(db, Migrations()); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"cash", "bank"} {
		if _, err := db.Exec(`INSERT INTO accounts (name, kind, opening_balance_cents, sort_order, created_at, updated_at)
			VALUES (?, ?, 0, ?, '2026-11-13 09:30:00', '2026-11-13 09:30:00')`, kind, kind, i); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestCategoriesTable_IsSeededWithTheSevenDefaultCategories(t *testing.T) {
	// Arrange
	repo := NewLedgerRepository(newLedgerDB(t))

	// Act
	all, err := repo.Categories(context.Background())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := []app.Category{
		{Name: "Cuota", Direction: domain.MovementIncome, SystemKey: "fee", SortOrder: 1},
		{Name: "Extraordinaria", Direction: domain.MovementIncome, SystemKey: "assessment", SortOrder: 2},
		{Name: "Otros ingresos", Direction: domain.MovementIncome, SortOrder: 3},
		{Name: "Insumos", Direction: domain.MovementExpense, SortOrder: 10},
		{Name: "Servicios", Direction: domain.MovementExpense, SortOrder: 11},
		{Name: "Mtto preventivo", Direction: domain.MovementExpense, SortOrder: 12},
		{Name: "Reparaciones", Direction: domain.MovementExpense, SortOrder: 13},
	}
	if len(all) != len(want) {
		t.Fatalf("%d categories, want %d: %+v", len(all), len(want), all)
	}
	for i, w := range want {
		g := all[i]
		if g.Name != w.Name || g.Direction != w.Direction || g.SystemKey != w.SystemKey || g.SortOrder != w.SortOrder || !g.Active {
			t.Errorf("category %d = %+v, want %+v active", i, g, w)
		}
	}
}

func TestCategoriesTable_RejectsADuplicateSystemKeyAnUnknownKeyAndAnUnknownDirection(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"a second category with the fee key", `INSERT INTO categories (name, direction, system_key, sort_order) VALUES ('Otra cuota', 'income', 'fee', 9)`},
		{"an unknown system key", `INSERT INTO categories (name, direction, system_key, sort_order) VALUES ('X', 'income', 'other', 9)`},
		{"an unknown direction", `INSERT INTO categories (name, direction, sort_order) VALUES ('X', 'transfer', 9)`},
		{"an active flag that is not 0 or 1", `INSERT INTO categories (name, direction, active, sort_order) VALUES ('X', 'income', 2, 9)`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			db := newLedgerDB(t)

			// Act
			_, err := db.Exec(c.sql)

			// Assert
			if err == nil {
				t.Fatal("want the database to refuse the row")
			}
		})
	}
}

func TestMovementsTable_EnforcesItsTableLevelChecks(t *testing.T) {
	const insert = `INSERT INTO movements (date, kind, account_id, to_account_id, amount_cents, category_id, manual_receipt, description, created_at, updated_at)
		VALUES ('2026-11-10', ?, ?, ?, ?, ?, ?, 'x', '2026-11-13 09:30:00', '2026-11-13 09:30:00')`
	cases := []struct {
		name string
		args []any // kind, account, to account, amount, category, manual receipt
		ok   bool
	}{
		{"a valid income", []any{"income", 1, nil, 100, 3, "none"}, true},
		{"a valid transfer", []any{"transfer", 1, 2, 100, nil, "none"}, true},
		{"a valid negative adjustment", []any{"adjustment", 1, nil, -100, nil, "none"}, true},
		{"a valid income with a pending receipt", []any{"income", 1, nil, 100, 3, "pending"}, true},
		{"an income of zero", []any{"income", 1, nil, 0, 3, "none"}, false},
		{"an expense of a negative amount", []any{"expense", 1, nil, -100, 4, "none"}, false},
		{"an adjustment of zero", []any{"adjustment", 1, nil, 0, nil, "none"}, false},
		{"a transfer without a destination", []any{"transfer", 1, nil, 100, nil, "none"}, false},
		{"an income with a destination", []any{"income", 1, 2, 100, 3, "none"}, false},
		{"a transfer to the same account", []any{"transfer", 1, 1, 100, nil, "none"}, false},
		{"an income without a category", []any{"income", 1, nil, 100, nil, "none"}, false},
		{"a transfer with a category", []any{"transfer", 1, 2, 100, 3, "none"}, false},
		{"an adjustment with a category", []any{"adjustment", 1, nil, 100, 3, "none"}, false},
		{"an expense with a pending manual receipt", []any{"expense", 1, nil, 100, 4, "pending"}, false},
		{"an unknown kind", []any{"refund", 1, nil, 100, 3, "none"}, false},
		{"an unknown manual receipt status", []any{"income", 1, nil, 100, 3, "later"}, false},
		{"an account that does not exist", []any{"income", 9, nil, 100, 3, "none"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			db := newLedgerDB(t)

			// Act
			_, err := db.Exec(insert, c.args...)

			// Assert
			if c.ok && err != nil {
				t.Fatalf("the database refused a valid row: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("want the database to refuse the row")
			}
		})
	}
}

func TestLedgerRepository_MovementsReturnsLiveRowsInDateThenIdOrderWithTheirFields(t *testing.T) {
	// Arrange
	db := newLedgerDB(t)
	repo := NewLedgerRepository(db)
	ctx := context.Background()
	category := int64(4)
	link := app.Movement{
		Date: now.AddDate(0, 0, -1), Kind: domain.MovementExpense, AccountID: 1, Amount: 500, CategoryID: &category,
		Description: "b", Notes: "nota", ReceiptLink: "http://x", Receipt: app.ReceiptNone,
	}
	earlier := app.Movement{Date: now.AddDate(0, 0, -3), Kind: domain.MovementExpense, AccountID: 2, Amount: 700, CategoryID: &category, Description: "a", Receipt: app.ReceiptNone}
	gone := app.Movement{Date: now, Kind: domain.MovementExpense, AccountID: 1, Amount: 900, CategoryID: &category, Description: "c", Receipt: app.ReceiptNone}
	for _, m := range []app.Movement{link, earlier, gone} {
		if _, err := repo.InsertMovement(ctx, m, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE movements SET deleted_at = '2026-11-13 10:00:00' WHERE description = 'c'`); err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := repo.Movements(ctx)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Description != "a" || got[1].Description != "b" {
		t.Fatalf("movements = %+v, want the two live rows, earliest first", got)
	}
	b := got[1]
	if b.Notes != "nota" || b.ReceiptLink != "http://x" || b.Amount != 500 || b.AccountID != 1 || b.ToAccountID != nil ||
		b.CategoryID == nil || *b.CategoryID != 4 || b.VendorID != nil || b.Date.Format(dateLayout) != "2026-11-12" {
		t.Fatalf("movement = %+v", b)
	}
}

func TestLedgerRepositoryInTx_RollsBackTheInsertWhenTheFunctionFails(t *testing.T) {
	// Arrange
	repo := NewLedgerRepository(newLedgerDB(t))
	ctx := context.Background()
	boom := errors.New("boom")
	category := int64(4)

	// Act
	err := repo.InTx(ctx, func(tx app.LedgerStore) error {
		m := app.Movement{Date: now, Kind: domain.MovementExpense, AccountID: 1, Amount: 500, CategoryID: &category, Description: "x", Receipt: app.ReceiptNone}
		if _, err := tx.InsertMovement(ctx, m, now); err != nil {
			return err
		}
		return boom
	})

	// Assert
	all, listErr := repo.Movements(ctx)
	if !errors.Is(err, boom) || listErr != nil || len(all) != 0 {
		t.Fatalf("error = %v, movements = %v (%v); want the insert rolled back", err, all, listErr)
	}
}
