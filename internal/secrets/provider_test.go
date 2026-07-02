package secrets

import (
	"errors"
	"testing"
)

func TestMemoryProviderRoundTrip(t *testing.T) {
	p := NewMemory()

	if _, err := p.Get("acme", "GITHUB_TOKEN"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get on empty = %v, want ErrNotFound", err)
	}

	if err := p.Set("acme", "GITHUB_TOKEN", "ghp_123"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got, err := p.Get("acme", "GITHUB_TOKEN")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != "ghp_123" {
		t.Fatalf("Get = %q, want ghp_123", got)
	}

	if _, err := p.Get("personal", "GITHUB_TOKEN"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-profile Get = %v, want ErrNotFound", err)
	}

	if err := p.Delete("acme", "GITHUB_TOKEN"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := p.Get("acme", "GITHUB_TOKEN"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete = %v, want ErrNotFound", err)
	}
}
