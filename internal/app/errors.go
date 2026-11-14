package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Sentinel errors for cases the UI must tell apart from a failure.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

// ValidationError lists every invalid field of an operation's input, so a form
// can show all errors at once. Fields maps a field key to a code from
// validation codes (see Codes); the TUI turns codes into Spanish messages.
type ValidationError struct {
	Fields map[string]string
}

func (e ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s: %s", k, e.Fields[k])
	}
	return "invalid input: " + strings.Join(parts, "; ")
}

func (e *ValidationError) add(field, code string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[field] = code
}

// err returns the error to hand back: nil when no field is invalid.
func (e ValidationError) err() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}
