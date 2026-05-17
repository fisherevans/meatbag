package blobs

import (
	"bytes"
	"io"
	"testing"
)

func TestWriteAndDedupe(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sha1, n1, err := s.Write(bytes.NewReader([]byte("hello world")))
	if err != nil {
		t.Fatal(err)
	}
	if n1 != 11 {
		t.Fatalf("size: %d", n1)
	}
	if len(sha1) != 64 {
		t.Fatalf("sha length: %d", len(sha1))
	}

	// Second write of same content -> same sha, no error.
	sha2, _, err := s.Write(bytes.NewReader([]byte("hello world")))
	if err != nil {
		t.Fatal(err)
	}
	if sha1 != sha2 {
		t.Fatalf("dedupe failed: %s vs %s", sha1, sha2)
	}

	// Read back.
	rc, err := s.Read(sha1)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if string(got) != "hello world" {
		t.Fatalf("read: %q", got)
	}

	// Listing.
	all, err := s.ListAll()
	if err != nil || len(all) != 1 || all[0] != sha1 {
		t.Fatalf("list: %v %v", all, err)
	}

	// Delete is idempotent.
	if err := s.Delete(sha1); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(sha1); err != nil {
		t.Fatal(err)
	}
}

func TestRefRoundTrip(t *testing.T) {
	sha := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ref := BuildRef(sha)
	got, err := ParseRef(ref)
	if err != nil || got != sha {
		t.Fatalf("roundtrip: %v %v", got, err)
	}
	if _, err := ParseRef("not-a-blob"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseRef("blobs/short"); err == nil {
		t.Fatal("expected error")
	}
}
