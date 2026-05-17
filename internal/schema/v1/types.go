// Package v1 defines the meatbag list schema version 1. Types here are the
// canonical shape of a list file at this version. Newer schema versions live
// in sibling packages (v2, v3, ...) with a migration function that converts
// from the previous version. Code outside the schema tree should reference
// these types via `internal/store`'s type aliases, which always point at the
// latest version.
package v1

import "time"

// SchemaVersion is the on-disk version this package handles. Stamped on every
// save and consulted on load by `internal/schema`.
const SchemaVersion = 1

type State string

const (
	StateTodo       State = "todo"
	StateInProgress State = "in_progress"
	StateBlocked    State = "blocked"
	StateDone       State = "done"
	StateSkipped    State = "skipped"
)

func ValidState(s State) bool {
	switch s {
	case StateTodo, StateInProgress, StateBlocked, StateDone, StateSkipped:
		return true
	}
	return false
}

type Owner string

const (
	OwnerHuman Owner = "human"
	OwnerAgent Owner = "agent"
)

func ValidOwner(o Owner) bool {
	return o == OwnerHuman || o == OwnerAgent
}

type ListStatus string

const (
	ListActive   ListStatus = "active"
	ListArchived ListStatus = "archived"
)

// Input describes a single structured input field on an item.
type Input struct {
	Name        string   `yaml:"name" json:"name"`
	Type        string   `yaml:"type" json:"type"`
	Label       string   `yaml:"label,omitempty" json:"label,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool     `yaml:"required,omitempty" json:"required,omitempty"`
	Options     []string `yaml:"options,omitempty" json:"options,omitempty"`
	Accept      []string `yaml:"accept,omitempty" json:"accept,omitempty"`
	Default     any      `yaml:"default,omitempty" json:"default,omitempty"`
}

// InputValue is the persisted answer to an Input. For plain types, Value
// holds the data inline. For password types, SecretRef points at the Keychain
// entry. For file types, BlobRef points at ~/.meatbag/blobs/<sha256>.
type InputValue struct {
	Value     any    `yaml:"value,omitempty" json:"value,omitempty"`
	SecretRef string `yaml:"secret_ref,omitempty" json:"secret_ref,omitempty"`
	BlobRef   string `yaml:"blob_ref,omitempty" json:"blob_ref,omitempty"`
	Filename  string `yaml:"filename,omitempty" json:"filename,omitempty"`
	Size      int64  `yaml:"size,omitempty" json:"size,omitempty"`
	HasValue  bool   `yaml:"has_value" json:"has_value"`
}

type Item struct {
	ID          string                `yaml:"id" json:"id"`
	Title       string                `yaml:"title" json:"title"`
	Owner       Owner                 `yaml:"owner" json:"owner"`
	State       State                 `yaml:"state" json:"state"`
	Content     string                `yaml:"content,omitempty" json:"content,omitempty"`
	Inputs      []Input               `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	InputValues map[string]InputValue `yaml:"input_values,omitempty" json:"input_values,omitempty"`
	Note        string                `yaml:"note,omitempty" json:"note,omitempty"`
	CreatedAt   time.Time             `yaml:"created_at" json:"created_at"`
	UpdatedAt   time.Time             `yaml:"updated_at" json:"updated_at"`
	Children    []*Item               `yaml:"children,omitempty" json:"children,omitempty"`
}

// List is the root document. SchemaVersion is stamped first so the value is
// always at the top of the YAML and easy to spot in a `cat`.
type List struct {
	SchemaVersion int        `yaml:"schema_version" json:"schema_version"`
	ID            string     `yaml:"id" json:"id"`
	Slug          string     `yaml:"slug" json:"slug"`
	Title         string     `yaml:"title" json:"title"`
	Description   string     `yaml:"description,omitempty" json:"description,omitempty"`
	ProjectPath   string     `yaml:"project_path,omitempty" json:"project_path,omitempty"`
	Status        ListStatus `yaml:"status" json:"status"`
	CreatedAt     time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `yaml:"updated_at" json:"updated_at"`
	Items         []*Item    `yaml:"items,omitempty" json:"items,omitempty"`
}
