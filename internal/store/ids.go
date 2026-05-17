package store

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewListID returns a new ULID, lowercase.
func NewListID() string {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(rand.Reader, 0))
	return strings.ToLower(id.String())
}

// IsListID reports whether s looks like a ULID we generated (26 chars, base32).
func IsListID(s string) bool {
	if len(s) != 26 {
		return false
	}
	_, err := ulid.Parse(strings.ToUpper(s))
	return err == nil
}

var itemIDEncoding = base32.NewEncoding("abcdefghijkmnpqrstuvwxyz23456789").WithPadding(base32.NoPadding)

// NewItemID returns a new short item ID like "it_abcd1234".
func NewItemID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return "it_" + itemIDEncoding.EncodeToString(b[:])
}
