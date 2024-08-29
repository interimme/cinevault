package data

import (
	"cinevault.interimme.net/internal/validator"
	"math"
	"strings"
)

// Filters represents pagination and sorting options for database queries.
type Filters struct {
	Page         int      // Current page number.
	PageSize     int      // Number of items per page.
	Sort         string   // Field to sort by, possibly prefixed with '-' for descending order.
	SortSafelist []string // List of allowed fields that can be used for sorting.
}

// Metadata contains pagination metadata for a list of resources.
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`  // The current page number.
	PageSize     int `json:"page_size,omitempty"`     // The size of each page.
	FirstPage    int `json:"first_page,omitempty"`    // The first page number (typically 1).
	LastPage     int `json:"last_page,omitempty"`     // The last page number, calculated from total records.
	TotalRecords int `json:"total_records,omitempty"` // The total number of records across all pages.
}

// calculateMetadata calculates pagination metadata based on the total number of records, current page, and page size.
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		// Return an empty Metadata struct if there are no records.
		return Metadata{}
	}
	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))), // Calculate the last page based on total records and page size.
		TotalRecords: totalRecords,
	}
}

// sortColumn returns the column to sort by, after verifying it's in the safelist.
// If the sort value is not in the safelist, it panics.
func (f Filters) sortColumn() string {
	for _, safeValue := range f.SortSafelist {
		if f.Sort == safeValue {
			return strings.TrimPrefix(f.Sort, "-") // Remove '-' prefix if present.
		}
	}
	panic("unsafe sort parameter: " + f.Sort) // Panic if sort parameter is not in the safelist.
}

// sortDirection returns the sorting direction ("ASC" or "DESC") based on the prefix of the sort parameter.
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// ValidateFilters validates the Filters struct to ensure pagination and sorting parameters are valid.
func ValidateFilters(v *validator.Validator, f Filters) {
	// Check that the page parameter is greater than zero and not unreasonably large.
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")

	// Check that the page_size parameter is greater than zero and does not exceed a reasonable limit.
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	// Ensure that the sort parameter matches a value in the safelist.
	v.Check(validator.In(f.Sort, f.SortSafelist...), "sort", "invalid sort value")
}

// limit returns the page size, which is the number of items per page.
func (f Filters) limit() int {
	return f.PageSize
}

// offset calculates the starting point for the records to be retrieved based on the current page and page size.
func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}
