package schema

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const legacyYAML = `# A pre-versioning meatbag list (no schema_version stamp).
id: 01abcdefghijklmnopqrstuvwx
slug: legacy
title: Legacy list
status: active
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
items:
  - id: it_legacy01
    title: An old item
    owner: human
    state: todo
    created_at: 2026-01-01T00:00:00Z
    updated_at: 2026-01-01T00:00:00Z
`

func TestLoadLegacyUnstamped(t *testing.T) {
	got, err := LoadBytes([]byte(legacyYAML))
	if err != nil {
		t.Fatalf("LoadBytes: %v", err)
	}
	if got.Slug != "legacy" || got.Title != "Legacy list" {
		t.Fatalf("parse: %+v", got)
	}
	if got.SchemaVersion != CurrentVersion {
		t.Fatalf("expected SchemaVersion normalized to %d, got %d", CurrentVersion, got.SchemaVersion)
	}
}

func TestEncodeStampsVersion(t *testing.T) {
	in, err := LoadBytes([]byte(legacyYAML))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, in); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "schema_version: "+itoa(CurrentVersion)) {
		t.Fatalf("missing schema_version in output:\n%s", out)
	}
	// And the in-memory struct should now report the current version.
	if in.SchemaVersion != CurrentVersion {
		t.Fatalf("Encode did not normalize SchemaVersion: %d", in.SchemaVersion)
	}
}

func TestRoundTripPreservesData(t *testing.T) {
	in, err := LoadBytes([]byte(legacyYAML))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID || out.Slug != in.Slug || len(out.Items) != len(in.Items) {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", in, out)
	}
}

func TestUnknownFutureVersionRejected(t *testing.T) {
	future := `schema_version: 99
id: 01future
slug: future
title: From the future
status: active
created_at: 2030-01-01T00:00:00Z
updated_at: 2030-01-01T00:00:00Z
`
	_, err := LoadBytes([]byte(future))
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

// itoa: tiny helper so the test file has no extra deps.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
