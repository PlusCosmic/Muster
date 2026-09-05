// Package models holds the structs that cross the Go/frontend boundary for
// the app itself (not for a particular game — those live in each game's own
// models package, e.g. internal/rimworld/models), plus the helpers every
// boundary package shares.
//
// Field names are camelCased in JSON, matching the TypeScript mirrors that
// `wails3 generate bindings` writes to frontend/bindings. Optional values are
// pointers (JSON null); list fields must never be nil, because the frontend
// calls array methods on them without checking — use NonNil when building.
package models

// AppInfo describes the running app.
type AppInfo struct {
	Version  string `json:"version"`
	DataRoot string `json:"dataRoot"`
}

// Str returns a pointer to s, or nil when s is empty.
func Str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Deref returns *p, or "" when p is nil.
func Deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// Int64 returns a pointer to v.
func Int64(v int64) *int64 { return &v }

// NonNil turns a nil slice into an empty one so it serialises as [] not null.
func NonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
