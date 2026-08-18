package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
)

// httpReq builds a request with a literal string body.
func httpReq(method, path, body string) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
}

func newRR() *httptest.ResponseRecorder { return httptest.NewRecorder() }
