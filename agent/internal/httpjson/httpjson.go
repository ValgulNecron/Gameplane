// Package httpjson holds the agent's shared JSON response helpers.
//
// These functions encapsulate the common pattern of setting JSON
// content-type, writing an HTTP status code, and encoding a response
// body. They are extracted from duplication across the agent's handler
// packages to ensure consistency.
package httpjson

import (
	"encoding/json"
	"net/http"
)

// Write sets Content-Type application/json, writes the status code,
// and JSON-encodes the body. Encode errors are silently ignored,
// consistent with the existing handler implementations.
func Write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Error writes a JSON error response with the given status code and
// message, using the shape {"error": msg}.
func Error(w http.ResponseWriter, status int, msg string) {
	Write(w, status, map[string]string{"error": msg})
}
