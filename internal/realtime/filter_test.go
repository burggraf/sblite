// internal/realtime/filter_test.go
package realtime

import (
	"testing"
)

func TestMatchesFilterEq(t *testing.T) {
	row := map[string]any{"status": "active", "count": 5}

	if !matchesFilter("status=eq.active", row, nil) {
		t.Error("should match status=eq.active")
	}
	if matchesFilter("status=eq.inactive", row, nil) {
		t.Error("should not match status=eq.inactive")
	}
}

func TestMatchesFilterNeq(t *testing.T) {
	row := map[string]any{"status": "active"}

	if !matchesFilter("status=neq.inactive", row, nil) {
		t.Error("should match status=neq.inactive")
	}
	if matchesFilter("status=neq.active", row, nil) {
		t.Error("should not match status=neq.active")
	}
}

func TestMatchesFilterNumeric(t *testing.T) {
	row := map[string]any{"count": float64(5)}

	if !matchesFilter("count=gt.3", row, nil) {
		t.Error("should match count=gt.3")
	}
	if !matchesFilter("count=gte.5", row, nil) {
		t.Error("should match count=gte.5")
	}
	if !matchesFilter("count=lt.10", row, nil) {
		t.Error("should match count=lt.10")
	}
	if !matchesFilter("count=lte.5", row, nil) {
		t.Error("should match count=lte.5")
	}
}

func TestMatchesFilterIn(t *testing.T) {
	row := map[string]any{"status": "active"}

	if !matchesFilter("status=in.(active,pending)", row, nil) {
		t.Error("should match status in (active,pending)")
	}
	if matchesFilter("status=in.(inactive,deleted)", row, nil) {
		t.Error("should not match status in (inactive,deleted)")
	}
}

func TestMatchesFilterInvalidFormat(t *testing.T) {
	row := map[string]any{"status": "active"}

	// Invalid filter formats should not match
	if matchesFilter("invalid", row, nil) {
		t.Error("invalid filter should not match")
	}
	if matchesFilter("status=invalid.value", row, nil) {
		t.Error("invalid operator should not match")
	}
}
