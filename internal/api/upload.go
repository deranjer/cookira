package api

import (
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/deranjer/cookira/internal/auth"
	"github.com/deranjer/cookira/internal/recipes"
)

// usedUploads remembers upload-link ids that have been redeemed (or are being
// redeemed) until the links would have expired anyway, which makes each link
// single-use. It is per process; links are only valid for minutes, and a
// restart simply forgets, which at worst lets a link be used again within its
// original window.
type usedUploads struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// claim reserves id until exp. It reports false if id was already claimed.
func (u *usedUploads) claim(id string, exp time.Time) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	for k, t := range u.seen {
		if t.Before(now) {
			delete(u.seen, k)
		}
	}
	if _, dup := u.seen[id]; dup {
		return false
	}
	if u.seen == nil {
		u.seen = map[string]time.Time{}
	}
	u.seen[id] = exp
	return true
}

// release gives a claim back, so a failed upload can be retried.
func (u *usedUploads) release(id string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.seen, id)
}

// uploadLink is the mcpserver.UploadLinker: it signs a token for recipeID and
// returns the absolute URL the agent should PUT the image to.
func (s *Server) uploadLink(userID, recipeID string) (string, time.Time, error) {
	tok, claims, err := s.mgr.UploadToken(userID, recipeID)
	if err != nil {
		return "", time.Time{}, err
	}
	return s.publicURL + "/api/v1/upload/" + url.PathEscape(tok), claims.ExpiresAt, nil
}

// registerUploadRoute accepts the raw image bytes of an upload link minted by
// the MCP create_image_upload_url tool. The link itself is the credential: it
// is signed, scoped to one recipe, short-lived and single-use. This is how an
// agent gets a local photo into a recipe without pushing megabytes of base64
// through the model.
func (s *Server) registerUploadRoute(r *chi.Mux) {
	h := func(w http.ResponseWriter, req *http.Request) {
		claims, err := s.mgr.VerifyUpload(chi.URLParam(req, "token"))
		if err != nil {
			writeProblem(w, http.StatusUnauthorized, "upload link is invalid or has expired")
			return
		}
		if !s.uploads.claim(claims.TokenID, claims.ExpiresAt) {
			writeProblem(w, http.StatusConflict, "upload link was already used")
			return
		}
		ok := false
		defer func() {
			if !ok {
				s.uploads.release(claims.TokenID)
			}
		}()

		data, err := io.ReadAll(http.MaxBytesReader(w, req.Body, recipes.MaxImageBytes+1))
		if err != nil || len(data) == 0 {
			writeProblem(w, http.StatusBadRequest, "expected the raw image bytes as the request body, no larger than 10 MB")
			return
		}
		ctx := auth.WithIdentity(req.Context(), auth.Identity{UserID: claims.UserID, OAuth: true, Write: true, ClientName: "upload link"})
		if err := s.svc.SetImage(ctx, claims.RecipeID, data); err != nil {
			writeServiceError(w, err)
			return
		}
		ok = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}
	r.Put("/api/v1/upload/{token}", h)
	r.Post("/api/v1/upload/{token}", h)
}
