package i18n

import "testing"

// TestLocaleParity guards the project rule that English and Russian ship in
// sync: every message key must exist in both locales.
func TestLocaleParity(t *testing.T) {
	svc, err := NewService()
	if err != nil {
		t.Fatalf("create i18n service: %v", err)
	}

	english := svc.locales["en"]
	russian := svc.locales["ru"]

	for key := range english {
		if _, ok := russian[key]; !ok {
			t.Errorf("key %q present in en but missing in ru", key)
		}
	}
	for key := range russian {
		if _, ok := english[key]; !ok {
			t.Errorf("key %q present in ru but missing in en", key)
		}
	}
}
