// Package templates owns the embedded module catalogue: module manifests and
// their files under modules/, and the shared skill library under skills/.
package templates

import "embed"

// Files includes hidden assets such as .gitignore and .gitkeep.
//
//go:embed all:modules all:skills
var Files embed.FS
