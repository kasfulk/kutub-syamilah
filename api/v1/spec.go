// Package spec provides the embedded OpenAPI specification for the Kutub Syamilah API.
package spec

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte
