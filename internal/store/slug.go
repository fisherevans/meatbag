package store

import (
	"fmt"
	"regexp"
	"strings"
)

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// SlugifyTitle produces a base slug from a title. Empty if title has no usable
// characters; callers should fall back to a default.
func SlugifyTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugInvalid.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// ValidSlug checks a user-supplied slug.
func ValidSlug(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' && i > 0 && i < len(s)-1:
		default:
			return false
		}
	}
	return true
}

// AllocateSlug returns a slug not in `taken`, appending -2, -3, ... on collision.
func AllocateSlug(base string, taken map[string]bool) string {
	if base == "" {
		base = "list"
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", base, i)
		if !taken[cand] {
			return cand
		}
	}
}
