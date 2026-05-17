package store

import (
	"github.com/fisherevans/meatbag/internal/schema"
	v1 "github.com/fisherevans/meatbag/internal/schema/v1"
)

// Re-export the current schema's types under stable names. Bump these
// aliases (along with `schema.Latest`) when a new schema version lands so
// the rest of the codebase keeps compiling without touching call sites.
type (
	List       = schema.Latest
	Item       = v1.Item
	Input      = v1.Input
	InputValue = v1.InputValue
	State      = v1.State
	Owner      = v1.Owner
	ListStatus = v1.ListStatus
)

// Re-export typed constants. Constants can't be aliased, but assigning the
// already-typed value through `const` preserves the type via the alias.
const (
	StateTodo       = v1.StateTodo
	StateInProgress = v1.StateInProgress
	StateBlocked    = v1.StateBlocked
	StateDone       = v1.StateDone
	StateSkipped    = v1.StateSkipped

	OwnerHuman = v1.OwnerHuman
	OwnerAgent = v1.OwnerAgent

	ListActive   = v1.ListActive
	ListArchived = v1.ListArchived
)

// Validators come along for the ride.
var (
	ValidState = v1.ValidState
	ValidOwner = v1.ValidOwner
)
