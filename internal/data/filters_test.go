package data

import (
	"testing"

	"github.com/skybytescode/greenlight/internal/validator"
)

func TestValidateFilters(t *testing.T) {
	safelist := []string{"id", "year", "-year"}
	tests := []struct {
		name    string
		filters Filters
		wantKey string
	}{
		{"valid", Filters{Page: 1, PageSize: 20, Sort: "-year", SortSafelist: safelist}, ""},
		{"page zero", Filters{Page: 0, PageSize: 20, Sort: "id", SortSafelist: safelist}, "page"},
		{"page size too big", Filters{Page: 1, PageSize: 101, Sort: "id", SortSafelist: safelist}, "page_size"},
		{"sort not in safelist", Filters{Page: 1, PageSize: 20, Sort: "password", SortSafelist: safelist}, "sort"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := validator.New()
			ValidateFilters(v, tt.filters)
			if tt.wantKey == "" && !v.Valid() {
				t.Fatalf("unexpected errors: %v", v.Errors)
			}
			if tt.wantKey != "" {
				if _, ok := v.Errors[tt.wantKey]; !ok {
					t.Fatalf("expected an error for %q, got %v", tt.wantKey, v.Errors)
				}
			}
		})
	}
}

func TestSortColumnAndDirection(t *testing.T) {
	f := Filters{Sort: "-year", SortSafelist: []string{"year", "-year"}}
	if got := f.sortColumn(); got != "year" {
		t.Errorf("sortColumn() = %q, want year", got)
	}
	if got := f.sortDirection(); got != "DESC" {
		t.Errorf("sortDirection() = %q, want DESC", got)
	}
}

// sortColumn is the last guard before the value is put into SQL, so an
// unlisted value must panic rather than reach the query.
func TestSortColumnPanicsOnUnsafeValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a sort value outside the safelist")
		}
	}()
	Filters{Sort: "id; DROP TABLE movies", SortSafelist: []string{"id"}}.sortColumn()
}

func TestOffsetAndMetadata(t *testing.T) {
	f := Filters{Page: 3, PageSize: 20}
	if got := f.offset(); got != 40 {
		t.Errorf("offset() = %d, want 40", got)
	}

	got := calculateMetadata(45, 3, 20)
	want := Metadata{CurrentPage: 3, PageSize: 20, FirstPage: 1, LastPage: 3, TotalRecords: 45}
	if got != want {
		t.Errorf("calculateMetadata = %+v, want %+v", got, want)
	}
	if got := calculateMetadata(0, 1, 20); got != (Metadata{}) {
		t.Errorf("no records should give empty metadata, got %+v", got)
	}
}
