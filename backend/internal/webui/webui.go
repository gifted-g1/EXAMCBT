// Package webui embeds the built React frontend (frontend/dist, copied
// into this package's dist/ folder at build time — see the top-level
// Dockerfile) directly into the Go backend binary.
//
// This exists specifically to make offline/LAN-mode exams actually
// work end to end: a genuinely offline exam hall has no route to a
// separately-hosted frontend server, so the same Go process that
// answers the REST/WebSocket API also serves the UI, on the one port
// the "Exam Server" panel advertises. Visiting http://<lan-ip>:<port>
// in a browser gets the full app, not a bare JSON 404.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/*
var embedded embed.FS

// Handler returns an http.Handler that serves the embedded frontend
// build, with SPA-style fallback: any path that isn't a real static
// asset (and isn't under an excluded prefix, e.g. /api or /ws) is
// served index.html so React Router can handle client-side routes
// like /student/exams/:id/write directly on refresh/deep-link.
func Handler(excludePrefixes ...string) http.Handler {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		// Embedding failed at compile time in a way we can't recover
		// from at runtime — serve a clear message instead of a panic
		// on every request.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend build not embedded — run the frontend build step before compiling the backend", http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range excludePrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				http.NotFound(w, r)
				return
			}
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}
		if _, err := fs.Stat(sub, cleanPath); err != nil {
			// Not a real file on disk (e.g. "/student/exams/abc/write")
			// — hand it to index.html and let React Router take over.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
