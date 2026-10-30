// Package schema embeds the JSON Schema for wsup.json, which is also served
// from the repository at the URL generated files name in $schema.
package schema

import _ "embed"

// Workspace is schema/wsup-3.json.
//
//go:embed wsup-3.json
var Workspace []byte
