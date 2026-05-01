package cli

import "embed"

//go:embed templates/*.md
var templateFS embed.FS
