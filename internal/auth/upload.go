package auth

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// UploadTokenTTL is how long an image-upload link stays valid.
	UploadTokenTTL = 10 * time.Minute

	// A distinct issuer means an upload token is never accepted as a login or
	// OAuth access token, and vice versa.
	uploadIssuer = "cookira-upload"
)

// UploadClaims is what an image-upload link authorises: one upload to one
// recipe, on behalf of one user.
type UploadClaims struct {
	UserID    string
	RecipeID  string
	TokenID   string // random; lets the server make the link single-use
	ExpiresAt time.Time
}

type uploadJWT struct {
	jwt.RegisteredClaims
	Recipe string `json:"rid"`
}

// SignUploadToken mints a short-lived, recipe-scoped token that the MCP
// create_image_upload_url tool hands out. Whoever holds it can upload one
// header image to that recipe without further credentials; that is the point,
// since the file is sent by curl rather than through the model.
func SignUploadToken(secret []byte, userID, recipeID string, now time.Time) (string, UploadClaims, error) {
	jti := make([]byte, 12)
	if _, err := rand.Read(jti); err != nil {
		return "", UploadClaims{}, err
	}
	c := UploadClaims{UserID: userID, RecipeID: recipeID, TokenID: hex.EncodeToString(jti), ExpiresAt: now.Add(UploadTokenTTL)}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, uploadJWT{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    uploadIssuer,
			Subject:   userID,
			ID:        c.TokenID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(c.ExpiresAt),
		},
		Recipe: recipeID,
	})
	s, err := tok.SignedString(secret)
	return s, c, err
}

// VerifyUploadToken validates signature, issuer and expiry.
func VerifyUploadToken(secret []byte, token string) (UploadClaims, error) {
	var claims uploadJWT
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(uploadIssuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || claims.Subject == "" || claims.Recipe == "" || claims.ID == "" {
		return UploadClaims{}, ErrInvalidToken
	}
	return UploadClaims{UserID: claims.Subject, RecipeID: claims.Recipe, TokenID: claims.ID, ExpiresAt: claims.ExpiresAt.Time}, nil
}

// UploadToken and VerifyUpload sign and check upload tokens with the
// manager's secret.
func (m *Manager) UploadToken(userID, recipeID string) (string, UploadClaims, error) {
	return SignUploadToken(m.secret, userID, recipeID, time.Now())
}

func (m *Manager) VerifyUpload(token string) (UploadClaims, error) {
	return VerifyUploadToken(m.secret, token)
}
