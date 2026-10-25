// Package templates owns the complete native workspace, project and collection starters.
package templates

import "embed"

// Files includes hidden assets such as .gitignore, .gitkeep and .agents.
//
//go:embed all:workspace all:project all:collections
var Files embed.FS
