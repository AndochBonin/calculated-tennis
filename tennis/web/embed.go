// Package web holds the Svelte front end. `npm run build` writes the SPA to
// dist/, which cmd/web embeds and serves.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
