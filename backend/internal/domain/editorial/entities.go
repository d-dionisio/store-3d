package editorial

import (
	"errors"
	"regexp"
	"strings"
	"time"

	catalog "example.com/loja-3d/backend/internal/domain/catalog"
)

var (
	ErrNotFound        = errors.New("editorial catalog not found")
	ErrInvalid         = errors.New("invalid editorial catalog")
	ErrConflict        = errors.New("editorial catalog conflict")
	ErrEmpty           = errors.New("editorial catalog is empty")
	ErrInactiveProduct = errors.New("product is not active and commercial")
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Catalog struct {
	ID, Slug, Title, Description, Eyebrow, EditionLabel, Status string
	CoverImageID                                                *string
	CoverImageURL                                               string
	Version                                                     int
	PublishedAt                                                 *time.Time
	CreatedAt, UpdatedAt                                        time.Time
	ProductIDs                                                  []string
	Products                                                    []catalog.Product
}

type Input struct {
	Slug, Title, Description, Eyebrow, EditionLabel string
	CoverImageID                                    *string
	ExpectedVersion                                 int
}

func (input Input) Validate() error {
	if !slugPattern.MatchString(input.Slug) || len(input.Slug) > 120 || strings.TrimSpace(input.Title) == "" || len(input.Title) > 160 || strings.TrimSpace(input.Description) == "" || len(input.Description) > 2000 || strings.TrimSpace(input.Eyebrow) == "" || len(input.Eyebrow) > 80 || len(input.EditionLabel) > 80 {
		return ErrInvalid
	}
	return nil
}

type Page struct {
	Items       []Catalog
	Page, Limit int
	Total       int64
}
