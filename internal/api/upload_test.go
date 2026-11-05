package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestUsedUploadsSingleUse(t *testing.T) {
	var u usedUploads
	exp := time.Now().Add(time.Minute)
	if !u.claim("a", exp) {
		t.Fatal("first claim refused")
	}
	if u.claim("a", exp) {
		t.Fatal("second claim allowed")
	}
	u.release("a")
	if !u.claim("a", exp) {
		t.Fatal("claim after release refused")
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		img.Set(x, x, color.RGBA{200, 50, 50, 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// An upload link minted for a recipe accepts one image, once.
func TestUploadLinkFlow(t *testing.T) {
	e := newOAuthEnv(t)
	created := e.do("POST", "/api/v1/recipes", "application/json", `{"title":"Upload Test"}`, e.withSession)
	var rec struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &rec); err != nil || rec.ID == "" {
		t.Fatalf("create recipe = %d %s", created.Code, created.Body.String())
	}

	link, _, err := e.srv.uploadLink(e.userID.String(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := strings.TrimPrefix(link, testBase)
	if _, err := url.Parse(path); err != nil {
		t.Fatal(err)
	}

	// A garbage token is refused.
	if r := e.do("PUT", "/api/v1/upload/nope", "image/png", "x", nil); r.Code != http.StatusUnauthorized {
		t.Errorf("bad token = %d, want 401", r.Code)
	}
	// A failed upload (not an image) doesn't burn the link.
	if r := e.do("PUT", path, "image/png", "not an image", nil); r.Code != http.StatusUnprocessableEntity {
		t.Errorf("non-image = %d, want 422: %s", r.Code, r.Body.String())
	}
	// A real image works...
	if r := e.do("PUT", path, "image/png", string(testPNG(t)), nil); r.Code != http.StatusOK {
		t.Fatalf("upload = %d: %s", r.Code, r.Body.String())
	}
	if r := e.do("GET", "/api/v1/recipes/"+rec.ID+"/image", "", "", e.withSession); r.Code != http.StatusOK {
		t.Errorf("image after upload = %d", r.Code)
	}
	// ...but only once.
	if r := e.do("PUT", path, "image/png", string(testPNG(t)), nil); r.Code != http.StatusConflict {
		t.Errorf("reuse = %d, want 409", r.Code)
	}
}
