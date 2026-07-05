package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestNewOIDC_EmptyIssuerReturnsNil(t *testing.T) {
	o, err := NewOIDC(context.Background(), "", "client", "secret", "http://x/cb")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if o != nil {
		t.Fatalf("expected nil OIDC")
	}
}

func TestNewOIDC_BadIssuer(t *testing.T) {
	// Pointing at a non-OIDC URL fails discovery.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	_, err := NewOIDC(context.Background(), srv.URL, "c", "s", "http://x/cb")
	if err == nil {
		t.Fatal("expected discovery error")
	}
}

// stubOIDC builds an OIDC instance with just enough plumbing to
// exercise HandleStart / HandleCallback's pre-IdP branches without a
// real issuer.
func stubOIDC() *OIDC {
	return &OIDC{
		oauth: &oauth2.Config{
			ClientID:    "client",
			RedirectURL: "https://app.example.com/oidc/cb",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://idp.example.com/authorize",
				TokenURL: "https://idp.example.com/token",
			},
			Scopes: []string{"openid", "profile", "email"},
		},
	}
}

func TestHandleStart_SetsCookiesAndRedirects(t *testing.T) {
	o := stubOIDC()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/auth/oidc/start", nil)
	o.HandleStart()(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("code=%d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://idp.example.com/authorize") {
		t.Fatalf("Location=%q", loc)
	}
	var state, nonce bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == oidcStateCookie {
			state = true
		}
		if c.Name == oidcNonceCookie {
			nonce = true
		}
	}
	if !state || !nonce {
		t.Fatalf("cookies missing: %+v", rr.Result().Cookies())
	}
}

func TestHandleCallback_StateMismatch(t *testing.T) {
	o := stubOIDC()
	store := NewSessionStore(newAuthDB(t))

	t.Run("no cookie", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc", nil)
		o.HandleCallback(store)(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("cookie mismatch", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc", nil)
		req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: "different"})
		o.HandleCallback(store)(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("empty cookie", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/auth/oidc/callback?state=abc", nil)
		req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: ""})
		o.HandleCallback(store)(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", rr.Code)
		}
	})
}

func TestAttachStore(t *testing.T) {
	o := &OIDC{}
	store := newAuthDB(t)
	o.AttachStore(store)
	if o.db != store {
		t.Fatal("AttachStore did not attach")
	}
}

func TestResolveOrLinkUser_NoStoreErrors(t *testing.T) {
	o := &OIDC{}
	_, err := o.resolveOrLinkUser(context.Background(), "https://idp", "sub-1", "u@x", "U", "viewer", false)
	if err == nil || !strings.Contains(err.Error(), "no store attached") {
		t.Fatalf("got %v", err)
	}
}
