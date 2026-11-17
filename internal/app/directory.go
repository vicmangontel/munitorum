package app

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/vicmangontel/munitorum/internal/domain"
)

// Validation codes of the directory beyond the domain's own (domain.VendorIssues).
const (
	CodeVendorNameDuplicate  = "vendor_name_duplicate"  // rule 2
	CodeVendorCompanyInvalid = "vendor_company_invalid" // rule 4: not a live company
	CodeVendorKindInUse      = "vendor_kind_in_use"     // rule 5
)

// Reasons of a VendorConflict.
const (
	ConflictCompanyHasMembers = "company_has_members" // rule 6; Names are the members
	ConflictCompanyDeleted    = "company_deleted"     // restoring a person; Names is the company
	ConflictCompanyNotCompany = "company_not_company" // restoring a person whose company changed kind; Names is that vendor
)

// VendorConflict is returned when a delete or restore is refused because of
// other vendors. Names are display names, to be shown to the treasurer. They
// are never logged. It matches ErrConflict.
type VendorConflict struct {
	Reason string
	Names  []string
}

func (e VendorConflict) Error() string {
	return fmt.Sprintf("vendor conflict: %s: %s", e.Reason, strings.Join(e.Names, ", "))
}

func (e VendorConflict) Is(target error) bool { return target == ErrConflict }

// Vendor is a directory entry. Phone, Email and Notes are empty when not given.
type Vendor struct {
	ID        int64
	Kind      domain.VendorKind
	Name      string
	Phone     string
	Email     string
	Notes     string
	CompanyID *int64
	Deleted   bool
}

// VendorRow is a vendor as the list and the pickers show it.
type VendorRow struct {
	ID          int64
	Kind        domain.VendorKind
	Name        string
	CompanyName string
	Phone       string
	Email       string
	Deleted     bool
}

// DisplayName is "Name (Company)" for a person with a company, else the name.
func (r VendorRow) DisplayName() string { return domain.VendorDisplayName(r.Name, r.CompanyName) }

// VendorDetail is a vendor with its company (for a person; deleted or not, the
// link is kept) and its live members (for a company).
type VendorDetail struct {
	Vendor  Vendor
	Company *Vendor
	Members []Vendor
}

// ListVendorsInput filters the list. Kind and Search are optional.
type ListVendorsInput struct {
	Kind           domain.VendorKind
	Search         string
	IncludeDeleted bool
}

// VendorInput is what the treasurer typed for a vendor.
type VendorInput struct {
	Kind      domain.VendorKind
	Name      string
	Phone     string
	Email     string
	CompanyID *int64
	Notes     string
}

// UpdateVendorInput is the vendor to change and its new content.
type UpdateVendorInput struct {
	ID int64
	VendorInput
}

// VendorStore is what DirectoryService needs from storage inside one unit of work.
type VendorStore interface {
	// Vendors returns every vendor, deleted ones included, in no set order.
	Vendors(ctx context.Context) ([]Vendor, error)
	// InsertVendor writes a new vendor, ignoring v.ID, and returns its id.
	InsertVendor(ctx context.Context, v Vendor, now time.Time) (int64, error)
	// UpdateVendor writes every field of v except Deleted. It returns ErrNotFound
	// when the vendor does not exist.
	UpdateVendor(ctx context.Context, v Vendor, now time.Time) error
	// SetVendorDeleted sets or clears deleted_at. It returns ErrNotFound when the
	// vendor does not exist.
	SetVendorDeleted(ctx context.Context, id int64, deleted bool, now time.Time) error
}

// VendorRepository is a VendorStore that can also group operations in one
// transaction. Writes that read first (uniqueness, members) run inside InTx.
type VendorRepository interface {
	VendorStore
	// InTx runs fn in one transaction: committed when fn returns nil, rolled back
	// otherwise. fn must use the store it is given, not the repository.
	InTx(ctx context.Context, fn func(VendorStore) error) error
}

// DirectoryService manages the directory of people, companies and services.
type DirectoryService struct {
	repo  VendorRepository
	clock Clock
	log   *slog.Logger
}

// NewDirectoryService returns a DirectoryService.
func NewDirectoryService(repo VendorRepository, clock Clock, log *slog.Logger) *DirectoryService {
	return &DirectoryService{repo: repo, clock: clock, log: log}
}

// ListVendors returns the live vendors (every vendor with IncludeDeleted) of the
// kind that match Search, ordered by name. Search is case- and accent-insensitive
// and looks in the name, phone, email and company name.
//
// debt: filters in Go over every vendor; fine for a coto's directory of a few
// hundred, move to SQL if it passes a few thousand.
func (s *DirectoryService) ListVendors(ctx context.Context, in ListVendorsInput) ([]VendorRow, error) {
	all, err := s.repo.Vendors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vendors: %w", err)
	}
	byID := vendorsByID(all)
	needle := domain.FoldKey(strings.TrimSpace(in.Search))

	var rows []VendorRow
	for _, v := range all {
		if (v.Deleted && !in.IncludeDeleted) || (in.Kind != "" && v.Kind != in.Kind) {
			continue
		}
		row := VendorRow{ID: v.ID, Kind: v.Kind, Name: v.Name, Phone: v.Phone, Email: v.Email, Deleted: v.Deleted}
		if v.CompanyID != nil {
			row.CompanyName = byID[*v.CompanyID].Name
		}
		if needle == "" || row.matches(needle) {
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b VendorRow) int {
		return strings.Compare(sortKey(a.Name, a.ID), sortKey(b.Name, b.ID))
	})
	return rows, nil
}

func (r VendorRow) matches(needle string) bool {
	for _, field := range []string{r.Name, r.Phone, r.Email, r.CompanyName} {
		if strings.Contains(domain.FoldKey(field), needle) {
			return true
		}
	}
	return false
}

// sortKey orders by folded name, then by name, then by id, so that the order is
// stable and "Álvaro" sorts with the As.
func sortKey(name string, id int64) string {
	return fmt.Sprintf("%s\x00%s\x00%020d", domain.FoldKey(name), name, id)
}

// GetVendor returns a vendor, deleted or not, with its company and members.
// It returns ErrNotFound for an unknown id.
func (s *DirectoryService) GetVendor(ctx context.Context, id int64) (VendorDetail, error) {
	all, err := s.repo.Vendors(ctx)
	if err != nil {
		return VendorDetail{}, fmt.Errorf("get vendor %d: %w", id, err)
	}
	byID := vendorsByID(all)
	v, ok := byID[id]
	if !ok {
		return VendorDetail{}, fmt.Errorf("get vendor %d: %w", id, ErrNotFound)
	}
	detail := VendorDetail{Vendor: v}
	if v.CompanyID != nil {
		c := byID[*v.CompanyID]
		detail.Company = &c
	}
	if v.Kind == domain.VendorCompany {
		detail.Members = liveMembers(all, id)
	}
	return detail, nil
}

// CreateVendor applies directory rules 1 to 5 and writes the vendor.
func (s *DirectoryService) CreateVendor(ctx context.Context, in VendorInput) (created Vendor, err error) {
	started := time.Now()
	var fields []any
	defer func() { s.emit(ctx, "create_vendor", started, err, fields) }()

	err = s.repo.InTx(ctx, func(tx VendorStore) error {
		v, err := checkVendor(ctx, tx, 0, in)
		if err != nil {
			return err
		}
		if v.ID, err = tx.InsertVendor(ctx, v, s.clock.Now()); err != nil {
			return fmt.Errorf("create vendor: %w", err)
		}
		created = v
		fields = vendorFields(v)
		return nil
	})
	return created, err
}

// UpdateVendor applies directory rules 1 to 5 and rewrites the vendor. It
// returns ErrNotFound for an unknown or deleted vendor.
func (s *DirectoryService) UpdateVendor(ctx context.Context, in UpdateVendorInput) (updated Vendor, err error) {
	started := time.Now()
	fields := []any{"vendor_id", in.ID}
	defer func() { s.emit(ctx, "update_vendor", started, err, fields) }()

	err = s.repo.InTx(ctx, func(tx VendorStore) error {
		v, err := checkVendor(ctx, tx, in.ID, in.VendorInput)
		if err != nil {
			return err
		}
		if err := tx.UpdateVendor(ctx, v, s.clock.Now()); err != nil {
			return fmt.Errorf("update vendor %d: %w", in.ID, err)
		}
		updated = v
		fields = vendorFields(v)
		return nil
	})
	return updated, err
}

// DeleteVendor soft-deletes a vendor. A company with live members is refused
// with a VendorConflict naming them (rule 6). It returns ErrNotFound for an
// unknown or already deleted vendor.
func (s *DirectoryService) DeleteVendor(ctx context.Context, id int64) (err error) {
	started := time.Now()
	fields := []any{"vendor_id", id}
	defer func() { s.emit(ctx, "delete_vendor", started, err, fields) }()

	return s.repo.InTx(ctx, func(tx VendorStore) error {
		all, err := tx.Vendors(ctx)
		if err != nil {
			return fmt.Errorf("delete vendor %d: %w", id, err)
		}
		v, ok := vendorsByID(all)[id]
		if !ok || v.Deleted {
			return fmt.Errorf("delete vendor %d: %w", id, ErrNotFound)
		}
		fields = vendorFields(v)
		if members := liveMembers(all, id); len(members) > 0 {
			names := make([]string, len(members))
			for i, m := range members {
				names[i] = domain.VendorDisplayName(m.Name, v.Name)
			}
			return VendorConflict{Reason: ConflictCompanyHasMembers, Names: names}
		}
		if err := tx.SetVendorDeleted(ctx, id, true, s.clock.Now()); err != nil {
			return fmt.Errorf("delete vendor %d: %w", id, err)
		}
		return nil
	})
}

// RestoreVendor brings a deleted vendor back. Rule 2 is checked again: a name
// that now collides with a live vendor of the same kind is a ValidationError on
// the name. A person whose company is still deleted is refused with a
// VendorConflict naming the company; the link is kept. It returns ErrNotFound
// for an unknown or live vendor.
func (s *DirectoryService) RestoreVendor(ctx context.Context, id int64) (err error) {
	started := time.Now()
	fields := []any{"vendor_id", id}
	defer func() { s.emit(ctx, "restore_vendor", started, err, fields) }()

	return s.repo.InTx(ctx, func(tx VendorStore) error {
		all, err := tx.Vendors(ctx)
		if err != nil {
			return fmt.Errorf("restore vendor %d: %w", id, err)
		}
		byID := vendorsByID(all)
		v, ok := byID[id]
		if !ok || !v.Deleted {
			return fmt.Errorf("restore vendor %d: %w", id, ErrNotFound)
		}
		fields = vendorFields(v)
		if nameTaken(all, v) {
			return ValidationError{Fields: map[string]string{domain.VendorFieldName: CodeVendorNameDuplicate}}
		}
		if v.CompanyID != nil {
			c := byID[*v.CompanyID]
			switch {
			case c.Deleted:
				return VendorConflict{Reason: ConflictCompanyDeleted, Names: []string{c.Name}}
			case c.Kind != domain.VendorCompany:
				return VendorConflict{Reason: ConflictCompanyNotCompany, Names: []string{c.Name}}
			}
		}
		if err := tx.SetVendorDeleted(ctx, id, false, s.clock.Now()); err != nil {
			return fmt.Errorf("restore vendor %d: %w", id, err)
		}
		return nil
	})
}

// emit writes the wide event of one operation; see logEvent.
func (s *DirectoryService) emit(ctx context.Context, action string, started time.Time, err error, fields []any) {
	logEvent(ctx, s.log, action, started, err, fields)
}

func vendorFields(v Vendor) []any { return []any{"vendor_id", v.ID, "kind", string(v.Kind)} }

// checkVendor applies rules 1 to 5 to input for vendor id (0 when new) against
// the vendors the transaction sees, and returns the vendor to write. Every
// invalid field is reported at once.
func checkVendor(ctx context.Context, tx VendorStore, id int64, in VendorInput) (Vendor, error) {
	all, err := tx.Vendors(ctx)
	if err != nil {
		return Vendor{}, fmt.Errorf("check vendor: %w", err)
	}
	byID := vendorsByID(all)
	old, exists := byID[id]
	if id != 0 && (!exists || old.Deleted) {
		return Vendor{}, fmt.Errorf("check vendor %d: %w", id, ErrNotFound)
	}

	draft := domain.VendorDraft{
		ID: id, Kind: in.Kind, Name: in.Name, Phone: in.Phone, Email: in.Email,
		Notes: in.Notes, CompanyID: in.CompanyID,
	}.Trimmed()
	var ve ValidationError
	for _, issue := range draft.Check() {
		ve.add(issue.Field, issue.Code)
	}
	v := Vendor{
		ID: id, Kind: draft.Kind, Name: draft.Name, Phone: draft.Phone, Email: draft.Email,
		Notes: draft.Notes, CompanyID: draft.CompanyID,
	}

	_, nameBad := ve.Fields[domain.VendorFieldName]
	_, kindBad := ve.Fields[domain.VendorFieldKind]
	if !nameBad && !kindBad && nameTaken(all, v) {
		ve.add(domain.VendorFieldName, CodeVendorNameDuplicate)
	}
	if _, bad := ve.Fields[domain.VendorFieldCompanyID]; !bad && v.CompanyID != nil {
		if c, ok := byID[*v.CompanyID]; !ok || c.Deleted || c.Kind != domain.VendorCompany {
			ve.add(domain.VendorFieldCompanyID, CodeVendorCompanyInvalid)
		}
	}
	if id != 0 && old.Kind == domain.VendorCompany && v.Kind != domain.VendorCompany && len(liveMembers(all, id)) > 0 {
		ve.add(domain.VendorFieldKind, CodeVendorKindInUse)
	}
	return v, ve.err()
}

// nameTaken reports whether another live vendor of the same kind has v's name,
// ignoring case and accents (rule 2).
func nameTaken(all []Vendor, v Vendor) bool {
	key := domain.FoldKey(v.Name)
	return slices.ContainsFunc(all, func(o Vendor) bool {
		return o.ID != v.ID && !o.Deleted && o.Kind == v.Kind && domain.FoldKey(o.Name) == key
	})
}

// liveMembers returns the live persons whose company is id, ordered by name.
func liveMembers(all []Vendor, id int64) []Vendor {
	var members []Vendor
	for _, v := range all {
		if !v.Deleted && v.Kind == domain.VendorPerson && v.CompanyID != nil && *v.CompanyID == id {
			members = append(members, v)
		}
	}
	slices.SortFunc(members, func(a, b Vendor) int {
		return strings.Compare(sortKey(a.Name, a.ID), sortKey(b.Name, b.ID))
	})
	return members
}

func vendorsByID(all []Vendor) map[int64]Vendor {
	m := make(map[int64]Vendor, len(all))
	for _, v := range all {
		m[v.ID] = v
	}
	return m
}
