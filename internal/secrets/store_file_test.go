//go:build !darwin

package secrets

import (
	"testing"
)

func TestFileBackendRoundTrip(t *testing.T) {
	t.Setenv("MEATBAG_HOME", t.TempDir())

	ref := Ref{ListID: "list1", ItemID: "item1", Field: "api_key"}

	// Absent to start.
	if ok, err := Has(ref); err != nil || ok {
		t.Fatalf("Has on empty store = (%v, %v), want (false, nil)", ok, err)
	}
	if _, err := Get(ref); err != ErrNotFound {
		t.Fatalf("Get on empty store err = %v, want ErrNotFound", err)
	}

	// Set then read back.
	gotRef, err := Set(ref.ListID, ref.ItemID, ref.Field, "s3cr3t")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if gotRef != ref.String() {
		t.Fatalf("Set ref = %q, want %q", gotRef, ref.String())
	}
	if v, err := Get(ref); err != nil || v != "s3cr3t" {
		t.Fatalf("Get = (%q, %v), want (s3cr3t, nil)", v, err)
	}
	if ok, err := Has(ref); err != nil || !ok {
		t.Fatalf("Has after Set = (%v, %v), want (true, nil)", ok, err)
	}

	// Overwrite replaces in place.
	if _, err := Set(ref.ListID, ref.ItemID, ref.Field, "rotated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	if v, _ := Get(ref); v != "rotated" {
		t.Fatalf("Get after overwrite = %q, want rotated", v)
	}

	// A field with awkward characters must survive the filename round-trip.
	weird := Ref{ListID: "list1", ItemID: "item1", Field: "weird/field:name with spaces"}
	if _, err := Set(weird.ListID, weird.ItemID, weird.Field, "v"); err != nil {
		t.Fatalf("Set weird field: %v", err)
	}

	// ListAll recovers both refs from filenames.
	all, err := ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	found := map[string]bool{}
	for _, r := range all {
		found[r.String()] = true
	}
	if !found[ref.String()] || !found[weird.String()] {
		t.Fatalf("ListAll = %v, missing one of the set refs", all)
	}

	// Delete is idempotent.
	if err := Delete(ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := Delete(ref); err != nil {
		t.Fatalf("Delete (already gone): %v, want nil", err)
	}
	if ok, _ := Has(ref); ok {
		t.Fatalf("Has after Delete = true, want false")
	}
}
