// Package skills embeds the Nexrender agent skill distributed with the CLI.
package skills

import "embed"

//go:embed nexrender
var Files embed.FS
