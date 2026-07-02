package secrets

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeychainProviderRoundTrip(t *testing.T) {
	keyring.MockInit()

	p := NewKeychain()
	if err := p.Set("acme", "GITHUB_TOKEN", "ghp_abc"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got, err := p.Get("acme", "GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != "ghp_abc" {
		t.Fatalf("Get = %q, want ghp_abc", got)
	}
	if _, err := p.Get("acme", "MISSING"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing = %v, want ErrNotFound", err)
	}
	if err := p.Delete("acme", "GITHUB_TOKEN"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := p.Get("acme", "GITHUB_TOKEN"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
	}
}
