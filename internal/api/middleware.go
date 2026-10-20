package api

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"

	"github.com/deranjer/cookira/internal/auth"
)

// requireAuth is huma operation-level middleware (as opposed to
// auth.RequireAuth, which is chi/net-http middleware used for the
// non-huma /ws and /mcp routes): it validates the session cookie or
// Bearer API key and injects the resolved Identity into the request
// context, or short-circuits with 401.
func requireAuth(humaAPI huma.API, mgr *auth.Manager) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		r, _ := humachi.Unwrap(ctx)
		id, err := mgr.IdentityFromRequest(ctx.Context(), r)
		if err != nil {
			_ = huma.WriteErr(humaAPI, ctx, http.StatusUnauthorized, "login required")
			return
		}
		next(huma.WithContext(ctx, auth.WithIdentity(ctx.Context(), id)))
	}
}
