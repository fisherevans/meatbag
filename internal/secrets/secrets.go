// Package secrets stores password-typed input values outside the plaintext
// list YAML. Each secret is scoped by an account string "<list-id>:<item-id>:<field>"
// and referenced from YAML by a secret_ref of the form:
//
//	keychain:meatbag:<list-id>:<item-id>:<field>
//
// The backend is platform-specific:
//
//   - macOS (keychain_darwin.go): the login Keychain, one generic-password
//     item per secret under service "meatbag".
//   - other OSes (store_file.go): a per-$MEATBAG_HOME directory of 0600 files
//     under <home>/secrets/. Headless Linux boxes (e.g. the devbox) have no
//     login keychain and no Secret Service daemon, so a file store keyed the
//     same way is the portable equivalent.
//
// The "keychain:" ref prefix is retained on every platform so a ref written on
// one machine still parses on another - it's a namespace tag, not a claim that
// the value lives in an OS keychain. Use BuildRef / ParseRef to construct or
// decode refs; Set / Get / Has / Delete / ListAll are the backend-specific
// storage operations.
package secrets

import (
	"errors"
	"fmt"
	"strings"
)

const (
	Service   = "meatbag"
	RefPrefix = "keychain:meatbag:"
)

// ErrNotFound is returned when a secret ref has no stored value.
var ErrNotFound = errors.New("secret not found")

// Ref is a parsed secret reference.
type Ref struct {
	ListID string
	ItemID string
	Field  string
}

func (r Ref) String() string {
	return BuildRef(r.ListID, r.ItemID, r.Field)
}

func (r Ref) Account() string {
	return r.ListID + ":" + r.ItemID + ":" + r.Field
}

// BuildRef returns the canonical secret_ref string for a (list, item, field).
func BuildRef(listID, itemID, field string) string {
	return RefPrefix + listID + ":" + itemID + ":" + field
}

// ParseRef decodes a secret_ref string. Returns an error if it isn't the
// expected keychain:meatbag:... shape.
func ParseRef(s string) (Ref, error) {
	rest, ok := strings.CutPrefix(s, RefPrefix)
	if !ok {
		return Ref{}, fmt.Errorf("not a meatbag keychain ref: %q", s)
	}
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return Ref{}, fmt.Errorf("malformed ref: %q", s)
	}
	return Ref{ListID: parts[0], ItemID: parts[1], Field: parts[2]}, nil
}
