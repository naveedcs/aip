package secrets

import (
	"errors"

	"github.com/zalando/go-keyring"
)

type keychain struct{}

// NewKeychain returns a Provider backed by the OS keychain.
func NewKeychain() Provider {
	return keychain{}
}

func keychainService(profile string) string {
	return "aip." + profile
}

func (keychain) Get(profile, name string) (string, error) {
	value, err := keyring.Get(keychainService(profile), name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (keychain) Set(profile, name, value string) error {
	return keyring.Set(keychainService(profile), name, value)
}

func (keychain) Delete(profile, name string) error {
	err := keyring.Delete(keychainService(profile), name)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
