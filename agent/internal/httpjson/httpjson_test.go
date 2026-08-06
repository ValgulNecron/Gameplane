package httpjson

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       any
		wantStatus int
		wantBody   string
	}{
		{
			name:       "200 with struct",
			status:     http.StatusOK,
			body:       map[string]string{"id": "test", "name": "example"},
			wantStatus: http.StatusOK,
			wantBody:   `{"id":"test","name":"example"}`,
		},
		{
			name:       "201 with struct",
			status:     http.StatusCreated,
			body:       map[string]interface{}{"created": true, "count": 42},
			wantStatus: http.StatusCreated,
			wantBody:   `{"count":42,"created":true}`,
		},
		{
			name:       "200 with slice",
			status:     http.StatusOK,
			body:       []string{"a", "b", "c"},
			wantStatus: http.StatusOK,
			wantBody:   `["a","b","c"]`,
		},
		{
			name:       "202 with nil",
			status:     http.StatusAccepted,
			body:       nil,
			wantStatus: http.StatusAccepted,
			wantBody:   `null`,
		},
		{
			name:       "400 with error object",
			status:     http.StatusBadRequest,
			body:       map[string]string{"error": "bad input"},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"bad input"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Write(w, tt.status, tt.body)

			if w.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", w.Code, tt.wantStatus)
			}

			if w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", w.Header().Get("Content-Type"))
			}

			// For order-independent comparison, unmarshal both to compare
			var got, want any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
				return
			}
			if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
				t.Errorf("failed to unmarshal expected: %v", err)
				return
			}
			if got != want {
				t.Errorf("body = %v, want %v", got, want)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		msg        string
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "400 bad request",
			status:     http.StatusBadRequest,
			msg:        "invalid input",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "invalid input",
		},
		{
			name:       "404 not found",
			status:     http.StatusNotFound,
			msg:        "resource not found",
			wantStatus: http.StatusNotFound,
			wantMsg:    "resource not found",
		},
		{
			name:       "500 server error",
			status:     http.StatusInternalServerError,
			msg:        "internal error",
			wantStatus: http.StatusInternalServerError,
			wantMsg:    "internal error",
		},
		{
			name:       "empty message",
			status:     http.StatusBadRequest,
			msg:        "",
			wantStatus: http.StatusBadRequest,
			wantMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			Error(w, tt.status, tt.msg)

			if w.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", w.Code, tt.wantStatus)
			}

			if w.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", w.Header().Get("Content-Type"))
			}

			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Errorf("failed to unmarshal response: %v", err)
				return
			}

			if body["error"] != tt.wantMsg {
				t.Errorf("error message = %q, want %q", body["error"], tt.wantMsg)
			}
		})
	}
}
