package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}
	if hash == "correct horse battery staple" {
		t.Fatal("HashPassword did not hash the input")
	}

	if !CheckPassword(hash, "correct horse battery staple") {
		t.Error("CheckPassword rejected the correct password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Error("CheckPassword accepted an incorrect password")
	}
}

func TestHashPasswordUnique(t *testing.T) {
	// bcrypt salts each hash, so hashing the same password twice must not
	// produce the same output.
	hash1, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	hash2, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash1 == hash2 {
		t.Error("HashPassword produced identical hashes for two calls with the same input")
	}
}
