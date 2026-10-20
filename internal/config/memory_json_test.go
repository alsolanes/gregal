package config

import "testing"

// Recall i RecallProject han de tornar sempre slices no-nil: els endpoints
// que les serialitzen a JSON han d'emetre [] i no null (contracte de la UI).
func TestRecallEmptyIsNotNull(t *testing.T) {
	if got := filterEntries(nil, ""); got == nil {
		t.Fatal("filterEntries amb entrada buida ha de tornar slice no-nil, no nil")
	}
}
