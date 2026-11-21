package app

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// Category classifies the income or the expenses of the ledger. A category with
// a SystemKey is recorded by another module and can never be deactivated.
type Category struct {
	ID        int64
	Name      string
	Direction domain.MovementKind // MovementIncome or MovementExpense
	SystemKey string              // "fee", "assessment", or empty
	Active    bool
	SortOrder int
}

// ListCategoriesInput filters the list. Direction is optional; inactive
// categories are listed only with IncludeInactive.
type ListCategoriesInput struct {
	Direction       domain.MovementKind
	IncludeInactive bool
}

// CategoryRepository is what CategoryService needs from storage.
type CategoryRepository interface {
	// Categories returns every category, inactive ones included, in no set order.
	Categories(ctx context.Context) ([]Category, error)
}

// CategoryService reads the categories of the ledger.
type CategoryService struct {
	repo CategoryRepository
}

// NewCategoryService returns a CategoryService.
func NewCategoryService(repo CategoryRepository) *CategoryService {
	return &CategoryService{repo: repo}
}

// ListCategories returns the categories of the direction (every direction when
// empty), active ones unless IncludeInactive, in sort order.
func (s *CategoryService) ListCategories(ctx context.Context, in ListCategoriesInput) ([]Category, error) {
	all, err := s.repo.Categories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	var rows []Category
	for _, c := range all {
		if (in.Direction == "" || c.Direction == in.Direction) && (c.Active || in.IncludeInactive) {
			rows = append(rows, c)
		}
	}
	slices.SortFunc(rows, func(a, b Category) int {
		return cmp.Or(cmp.Compare(a.SortOrder, b.SortOrder), cmp.Compare(a.ID, b.ID))
	})
	return rows, nil
}
