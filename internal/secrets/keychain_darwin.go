//go:build darwin

// macOS backend: one generic-password Keychain item per secret, under service
// "meatbag" with account "<list-id>:<item-id>:<field>".
package secrets

import (
	"errors"
	"fmt"
	"strings"

	keychain "github.com/keybase/go-keychain"
)

// Set writes (or replaces) a Keychain entry. Returns the canonical secret_ref.
func Set(listID, itemID, field, value string) (string, error) {
	ref := Ref{ListID: listID, ItemID: itemID, Field: field}
	// Try update first to support overwrite without flapping.
	item := newItem(ref)
	item.SetData([]byte(value))
	err := keychain.AddItem(item)
	if errors.Is(err, keychain.ErrorDuplicateItem) {
		// Delete + re-add. The go-keychain Update path is fiddly across
		// versions; remove + insert is portable.
		if derr := keychain.DeleteItem(newQuery(ref)); derr != nil {
			return "", fmt.Errorf("replace (delete): %w", derr)
		}
		if aerr := keychain.AddItem(item); aerr != nil {
			return "", fmt.Errorf("replace (add): %w", aerr)
		}
	} else if err != nil {
		return "", err
	}
	return ref.String(), nil
}

// Get fetches the secret value.
func Get(ref Ref) (string, error) {
	q := newQuery(ref)
	q.SetReturnData(true)
	results, err := keychain.QueryItem(q)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", ErrNotFound
	}
	return string(results[0].Data), nil
}

// Has reports whether a secret exists without retrieving its value.
func Has(ref Ref) (bool, error) {
	q := newQuery(ref)
	q.SetReturnAttributes(true)
	results, err := keychain.QueryItem(q)
	if err != nil {
		return false, err
	}
	return len(results) > 0, nil
}

// Delete removes the secret. Returns nil if it was already gone.
func Delete(ref Ref) error {
	err := keychain.DeleteItem(newQuery(ref))
	if errors.Is(err, keychain.ErrorItemNotFound) {
		return nil
	}
	return err
}

// ListAll enumerates every meatbag-owned Keychain entry. Used by `meatbag gc`.
func ListAll() ([]Ref, error) {
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(Service)
	q.SetMatchLimit(keychain.MatchLimitAll)
	q.SetReturnAttributes(true)
	results, err := keychain.QueryItem(q)
	if err != nil {
		return nil, err
	}
	out := make([]Ref, 0, len(results))
	for _, r := range results {
		acct := r.Account
		parts := strings.SplitN(acct, ":", 3)
		if len(parts) != 3 {
			continue
		}
		out = append(out, Ref{ListID: parts[0], ItemID: parts[1], Field: parts[2]})
	}
	return out, nil
}

func newItem(ref Ref) keychain.Item {
	it := keychain.NewItem()
	it.SetSecClass(keychain.SecClassGenericPassword)
	it.SetService(Service)
	it.SetAccount(ref.Account())
	it.SetLabel("meatbag: " + ref.Field + " (" + ref.ItemID + ")")
	it.SetAccessible(keychain.AccessibleWhenUnlocked)
	it.SetSynchronizable(keychain.SynchronizableNo)
	return it
}

func newQuery(ref Ref) keychain.Item {
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(Service)
	q.SetAccount(ref.Account())
	q.SetMatchLimit(keychain.MatchLimitOne)
	return q
}
