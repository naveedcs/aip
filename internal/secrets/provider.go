package secrets

import "errors"

// ErrNotFound is returned when a secret does not exist.
var ErrNotFound = errors.New("secret not found")

// Provider stores secret values keyed by profile and name.
type Provider interface {
	Get(profile, name string) (string, error)
	Set(profile, name, value string) error
	Delete(profile, name string) error
}
