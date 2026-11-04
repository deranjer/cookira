package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// AccessTokenTTL is deliberately short: access tokens are stateless, so
	// this bounds how long a deleted user or stolen token stays usable.
	// Clients stay signed in by redeeming their refresh token.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is how long a refresh token lives from issue (each
	// redemption rotates it), so a client used at least this often never has
	// to sign in again.
	RefreshTokenTTL = 90 * 24 * time.Hour

	jwtIssuer = "cookira"

	// loginRefreshPrefix marks login refresh tokens; RefreshTokenPrefix is
	// the separate OAuth (MCP connector) refresh token.
	loginRefreshPrefix = "cwr_"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// SignAccessToken returns an HS256 JWT whose subject is the user ID. The
// role is intentionally not a claim: it is read from the database when it
// matters, so a demotion takes effect immediately.
func SignAccessToken(secret []byte, userID string, now time.Time) (string, time.Time, error) {
	exp := now.Add(AccessTokenTTL)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    jwtIssuer,
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	})
	s, err := tok.SignedString(secret)
	return s, exp, err
}

// VerifyAccessToken validates signature, issuer and expiry and returns the
// user ID.
func VerifyAccessToken(secret []byte, token string) (string, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// GenerateRefreshToken returns a new random opaque token and its stored hash.
func GenerateRefreshToken() (plaintext, hash string) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = loginRefreshPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashAPIKey(plaintext)
}

// GenerateInviteToken returns a new random invite token and its stored hash.
func GenerateInviteToken() (plaintext, hash string) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	return plaintext, HashAPIKey(plaintext)
}
