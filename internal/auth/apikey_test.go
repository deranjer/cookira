package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	plaintext, hash := GenerateAPIKey()

	if !strings.HasPrefix(plaintext, "ck_") {
		t.Errorf("plaintext = %q, want ck_ prefix", plaintext)
	}
	if len(plaintext) < 20 {
		t.Errorf("plaintext = %q, suspiciously short", plaintext)
	}

	sum := sha256.Sum256([]byte(plaintext))
	want := hex.EncodeToString(sum[:])
	if hash != want {
		t.Errorf("hash = %q, want %q (sha256 of plaintext)", hash, want)
	}
	if hash != HashAPIKey(plaintext) {
		t.Errorf("HashAPIKey(plaintext) = %q, want %q", HashAPIKey(plaintext), hash)
	}

	// Two calls must never collide.
	plaintext2, hash2 := GenerateAPIKey()
	if plaintext == plaintext2 || hash == hash2 {
		t.Error("GenerateAPIKey produced identical output twice")
	}
}

func TestScopesForWrite(t *testing.T) {
	tests := []struct {
		write bool
		want  []string
	}{
		{write: true, want: []string{"read", "write"}},
		{write: false, want: []string{"read"}},
	}
	for _, tt := range tests {
		got := ScopesForWrite(tt.write)
		if len(got) != len(tt.want) {
			t.Fatalf("ScopesForWrite(%v) = %v, want %v", tt.write, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ScopesForWrite(%v) = %v, want %v", tt.write, got, tt.want)
			}
		}
	}
}

func TestScopesCanWrite(t *testing.T) {
	tests := []struct {
		scopes []string
		want   bool
	}{
		{scopes: []string{"read", "write"}, want: true},
		{scopes: []string{"write"}, want: true},
		{scopes: []string{"read"}, want: false},
		{scopes: nil, want: false},
		{scopes: []string{}, want: false},
	}
	for _, tt := range tests {
		if got := ScopesCanWrite(tt.scopes); got != tt.want {
			t.Errorf("ScopesCanWrite(%v) = %v, want %v", tt.scopes, got, tt.want)
		}
	}
}
