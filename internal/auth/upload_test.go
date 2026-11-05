package auth

import (
	"testing"
	"time"
)

var uploadSecret = []byte("test-secret-test-secret-test-secret!")

func TestUploadTokenRoundTrip(t *testing.T) {
	tok, issued, err := SignUploadToken(uploadSecret, "user-1", "recipe-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyUploadToken(uploadSecret, tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "user-1" || got.RecipeID != "recipe-1" || got.TokenID != issued.TokenID || got.TokenID == "" {
		t.Errorf("claims = %+v", got)
	}
}

func TestUploadTokenRejected(t *testing.T) {
	expired, _, _ := SignUploadToken(uploadSecret, "u", "r", time.Now().Add(-2*UploadTokenTTL))
	good, _, _ := SignUploadToken(uploadSecret, "u", "r", time.Now())
	access, _, _ := SignAccessToken(uploadSecret, "u", time.Now())

	cases := map[string]struct {
		secret []byte
		token  string
	}{
		"expired":      {uploadSecret, expired},
		"wrong secret": {[]byte("another-secret-another-secret-12345"), good},
		"garbage":      {uploadSecret, "nope"},
		"access token": {uploadSecret, access},
	}
	for name, c := range cases {
		if _, err := VerifyUploadToken(c.secret, c.token); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// And the reverse: an upload token is not a login token.
	if _, err := VerifyAccessToken(uploadSecret, good); err == nil {
		t.Error("upload token accepted as an access token")
	}
}
