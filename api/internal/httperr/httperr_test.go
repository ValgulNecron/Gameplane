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

func TestWriteCode_5xxLogs(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteCode(rr, newReq(), http.StatusInternalServerError, errors.New("boom"))
	if rr.Code != 500 {
		t.Fatalf("code=%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "boom") {
		t.Fatalf("body=%q", rr.Body.String())
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
