package tracking

import (
	"testing"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

func ids(fences []domain.FenceRef) []string {
	out := make([]string, len(fences))
	for i, f := range fences {
		out[i] = f.ID
	}
	return out
}

func refs(idList ...string) []domain.FenceRef {
	out := make([]domain.FenceRef, len(idList))
	for i, id := range idList {
		out[i] = domain.FenceRef{ID: id, Name: "fence " + id}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		previous    []domain.FenceRef
		current     []domain.FenceRef
		wantEntered []string
		wantExited  []string
	}{
		{"outside to outside", nil, nil, []string{}, []string{}},
		{"first entry", nil, refs("a"), []string{"a"}, []string{}},
		{"stays inside", refs("a"), refs("a"), []string{}, []string{}},
		{"leaves", refs("a"), nil, []string{}, []string{"a"}},
		{"moves between fences", refs("a"), refs("b"), []string{"b"}, []string{"a"}},
		{"enters overlapping fence", refs("a"), refs("a", "b"), []string{"b"}, []string{}},
		{"leaves one of two overlapping", refs("a", "b"), refs("b"), []string{}, []string{"a"}},
		{"keeps input order", nil, refs("c", "a", "b"), []string{"c", "a", "b"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := Diff(tt.previous, tt.current)
			if got := ids(entered); !equal(got, tt.wantEntered) {
				t.Errorf("entered = %v, want %v", got, tt.wantEntered)
			}
			if got := ids(exited); !equal(got, tt.wantExited) {
				t.Errorf("exited = %v, want %v", got, tt.wantExited)
			}
		})
	}
}
