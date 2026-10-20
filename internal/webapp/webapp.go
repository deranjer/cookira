// Package webapp embeds the built frontend (web/, built via `pnpm build`
// with its Vite output directory pointed at internal/webapp/dist) so the
// server binary can serve it directly with no separate static file
// container.
package webapp

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded frontend build. Until `pnpm build` has run at
// least once, dist/ only contains a placeholder index.html.
//
// react-router-dom handles routing client-side, so a path like
// /projects/{id} only exists once index.html's JS bundle has loaded and
// taken over — there's no such file in the embedded build. A plain
// http.FileServer 404s a fresh request to that path (deep link, hard
// refresh); rewrite any path that isn't a real embedded file to "/" so
// index.html is served instead, letting the client-side router take it
// from there.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(sub, path); err != nil {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				r = r2
			}
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
