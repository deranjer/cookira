package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"slices"
)

const apiKeyPrefix = "ck_"

// GenerateAPIKey returns a new plaintext API key and its stored hash.
// Keys are high-entropy random tokens, not user-chosen passwords, so a
// fast hash (sha256) is appropriate — unlike bcrypt for login passwords,
// there's no brute-forceable low-entropy input to slow down.
func GenerateAPIKey() (plaintext, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = apiKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashAPIKey(plaintext)
}

func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ScopesForWrite returns the scopes array to store for a new key.
func ScopesForWrite(write bool) []string {
	if write {
		return []string{"read", "write"}
	}
	return []string{"read"}
}

func ScopesCanWrite(scopes []string) bool {
	return slices.Contains(scopes, "write")
}
