// Package openapi exposes the specification used to generate the CLI client.
package openapi

import _ "embed"

//go:embed openapi.json
var Document []byte
