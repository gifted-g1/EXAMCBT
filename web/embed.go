// Package web embeds the built React app (web/dist) into the Go binary so
// the exam hall needs no separate web server or internet access.
package web

import "embed"

// Dist holds the Vite build output. It contains only .gitkeep until
// `npm run build` has run; the server then shows a "frontend not built" page.
//
//go:embed all:dist
var Dist embed.FS
