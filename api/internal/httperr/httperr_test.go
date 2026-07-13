package httperr

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

func newReq() *http.Request {
	return httptest.NewRequest("GET", "/x", nil)
}

// TestWriteCode_5xxNeverEchoesErr is the B1 defence-in-depth regression
// test: a >=500 WriteCode call must never put the supplied err's text in
// the response body — several callers pass an unsanitized upstream error
// (e.g. a mod-registry GET failure that could carry an API key in a
// *url.Error's URL) straight through. The real error is still logged in
// full server-side (not asserted here); the HTTP response only gets the
// generic status text.
func TestWriteCode_5xxNeverEchoesErr(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteCode(rr, newReq(), http.StatusInternalServerError, errors.New("boom"))
	if rr.Code != 500 {
		t.Fatalf("code=%d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("body leaks the raw error: %q", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Internal Server Error") {
		t.Fatalf("body = %q, want the generic status text", rr.Body.String())
	}
}

// TestWriteCode_502NeverEchoesErr mirrors the real B1 shape: registry.go's
// handlers wrap upstream mod-registry errors as 502s.
func TestWriteCode_502NeverEchoesErr(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteCode(rr, newReq(), http.StatusBadGateway,
		errors.New(`registry GET: Get "https://api.steampowered.com/x?key=s3cret-steam-key": dial tcp: connection refused`))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("code=%d", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "s3cret-steam-key") {
		t.Fatalf("body leaks the API key: %q", rr.Body.String())
	}
}

func TestWriteCode_4xx(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteCode(rr, newReq(), http.StatusBadRequest, errors.New("bad"))
	if rr.Code != 400 {
		t.Fatalf("code=%d", rr.Code)
	}
}

func TestWrite_Classification(t *testing.T) {
	gr := schema.GroupResource{Resource: "x"}
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"forbidden namespace", scope.ErrForbiddenNamespace, http.StatusForbidden},
		{"forbidden cluster", scope.ErrForbiddenCluster, http.StatusBadRequest},
		{"not found", apierrors.NewNotFound(gr, "n"), http.StatusNotFound},
		{"already exists", apierrors.NewAlreadyExists(gr, "n"), http.StatusConflict},
		{"forbidden", apierrors.NewForbidden(gr, "n", errors.New("rbac")), http.StatusForbidden},
		{"unauthorized", apierrors.NewUnauthorized("nope"), http.StatusUnauthorized},
		{"invalid", apierrors.NewInvalid(schema.GroupKind{Group: "gameplane.local", Kind: "X"}, "n", nil), http.StatusUnprocessableEntity},
		{"bad request", apierrors.NewBadRequest("nope"), http.StatusBadRequest},
		{"empty body (io.EOF)", io.EOF, http.StatusBadRequest},
		{"unknown", errors.New("mystery"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			Write(rr, newReq(), tc.err)
			if rr.Code != tc.want {
				t.Fatalf("got %d, want %d", rr.Code, tc.want)
			}
		})
	}
}

func TestWrite_InvalidEchoesAPIErrMessage(t *testing.T) {
	se := &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Reason:  metav1.StatusReasonInvalid,
			Message: "name must be lowercase",
			Code:    http.StatusUnprocessableEntity,
		},
	}
	rr := httptest.NewRecorder()
	Write(rr, newReq(), se)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "name must be lowercase") {
		t.Fatalf("body=%q", rr.Body.String())
	}
}

func TestApiErrMessage_FallbackForNonStatusError(t *testing.T) {
	if got := apiErrMessage(errors.New("plain")); got != "invalid" {
		t.Fatalf("got %q", got)
	}
}
