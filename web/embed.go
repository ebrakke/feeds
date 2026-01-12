package web

import "embed"

//go:embed templates/*.html
var Templates embed.FS

//go:embed packs/*.json
var Packs embed.FS
