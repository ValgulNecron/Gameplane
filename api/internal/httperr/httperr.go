// Package httperr maps internal errors to safe, generic messages for
// the HTTP response while preserving the full error in the server log.
//
// The default path previously echoed err.Error() directly, which leaked
// K8s API paths, database column names, FS paths, and other detail to
// unauthenticated or minimally-authed callers.
package httperr

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// WriteCode writes a specific HTTP status with the supplied (safe)
// message. Use this when the handler has already classified the
// condition itself — for example, surfacing a CRD finalizer's
// in-use blocker as a 409.
//
// For a 4xx, err.Error() is echoed verbatim: by convention every existing
// 4xx caller has already classified the condition and hand-built a message
// safe to show the caller (e.g. "ref is required", "cluster already
// exists"). For a >=500 status that contract does not hold — several
// callers wrap a raw upstream error (e.g. a mod-registry GET failure) that
// was never vetted for caller-safety, and a *url.Error embeds the full
// request URL, query string included, which for a provider like Steam
// carries an admin's API key. So >=500 never echoes err.Error(): the
// generic http.StatusText for the code is written to the client, and the
// real error is still logged in full server-side.
func WriteCode(w http.ResponseWriter, req *http.Request, status int, err error) {
	if status >= 500 {
		slog.Error("handler error",
			"method", req.Method, "path", req.URL.Path, "err", err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	slog.Debug("handler client error",
		"method", req.Method, "path", req.URL.Path, "status", status, "err", err)
	http.Error(w, err.Error(), status)
}

// Write responds with the appropriate HTTP status for the given error
// and a generic message. The actual error is logged at Error level so
// operators can still debug.
func Write(w http.ResponseWriter, req *http.Request, err error) {
	status, msg := classify(err)
	if status >= 500 {
		slog.Error("handler error",
			"method", req.Method, "path", req.URL.Path, "err", err)
	} else {
		slog.Debug("handler client error",
			"method", req.Method, "path", req.URL.Path, "status", status, "err", err)
	}
	http.Error(w, msg, status)
}

// classify returns (status, safeMessage) for a range of common errors.
// The default is 500 "internal error" — never leak unknown errors.
func classify(err error) (int, string) {
	switch {
	case errors.Is(err, scope.ErrForbiddenNamespace):
		return http.StatusForbidden, "namespace not permitted"
	case errors.Is(err, scope.ErrForbiddenCluster):
		// 400, not 403: an unknown ?cluster= is a malformed request (like an
		// unrecognized query param value), not an authorization decision —
		// that mirrors rbac.Middleware's direct 400 for the same error.
		return http.StatusBadRequest, "cluster not permitted"
	case apierrors.IsNotFound(err):
		return http.StatusNotFound, "not found"
	case apierrors.IsAlreadyExists(err):
		return http.StatusConflict, "already exists"
	case apierrors.IsForbidden(err):
		return http.StatusForbidden, "forbidden"
	case apierrors.IsUnauthorized(err):
		return http.StatusUnauthorized, "unauthorized"
	case apierrors.IsInvalid(err):
		// Validation errors are safe to surface — they describe the
		// user's input, not server internals.
		return http.StatusUnprocessableEntity, apiErrMessage(err)
	case apierrors.IsBadRequest(err):
		return http.StatusBadRequest, "bad request"
	case errors.Is(err, io.EOF):
		return http.StatusBadRequest, "empty body"
	}
	return http.StatusInternalServerError, "internal error"
}

func apiErrMessage(err error) string {
	var se *apierrors.StatusError
	if errors.As(err, &se) && se.ErrStatus.Message != "" {
		return se.ErrStatus.Message
	}
	return "invalid"
}
