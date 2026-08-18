// Package api holds the OpenAPI description of this service.
//
// The specification lives here rather than beside the handlers so it is
// discoverable at the conventional path, and is embedded rather than read from
// disk so the distroless image needs no extra files.
package api

import _ "embed"

//go:embed openapi.yaml
var spec []byte

// Spec returns the OpenAPI document. It is copied on each call so a handler
// cannot accidentally mutate the embedded bytes.
func Spec() []byte {
	out := make([]byte, len(spec))
	copy(out, spec)
	return out
}
