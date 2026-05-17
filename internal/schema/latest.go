package schema

import v1 "github.com/fisherevans/meatbag/internal/schema/v1"

// Latest is an alias for the current schema's root type. Bump this (and the
// dispatch in LoadBytes) when a new schema version lands.
//
// Type aliases preserve struct identity, so `*v1.List` and `*Latest` are the
// exact same type at every call site - no conversions needed.
type Latest = v1.List
