// Package httpserver provides the mTLS-authenticated HTTP control plane
// for the capture sidecar: starting/stopping captures, polling status,
// and downloading completed PCAPNG files.
package httpserver

import "net/http"

// HandleStart handles POST /captures/{id}:start requests to begin capturing.
func HandleStart(w http.ResponseWriter, r *http.Request) {
	// TODO: implement capture start handler with mTLS auth, filter validation,
	// AF_PACKET setup, and PCAPNG writer initialization
}

// HandleStop handles POST /captures/{id}:stop requests to end a capture.
func HandleStop(w http.ResponseWriter, r *http.Request) {
	// TODO: implement capture stop handler with cleanup
}

// HandleStatus handles GET /captures/{id}/status requests for capture state.
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: implement status polling handler
}

// HandleDownload handles GET /captures/{id}/file requests to download PCAPNG.
func HandleDownload(w http.ResponseWriter, r *http.Request) {
	// TODO: implement file download handler with proper headers and mTLS auth
}
