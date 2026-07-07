package scope

import (
	"errors"
	"net/http/httptest"
	"testing"
)

type fakeLister []string

func (f fakeLister) IDs() []string { return f }

func TestResolveClusterNoParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/servers", nil)
	reg := fakeLister{"local", "prod"}

	id, err := ResolveCluster(req, reg)
	if err != nil {
		t.Errorf("ResolveCluster() returned err = %v, want nil", err)
	}
	if id != DefaultCluster {
		t.Errorf("ResolveCluster() returned %q, want %q", id, DefaultCluster)
	}
}

func TestResolveClusterLocalParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/servers?cluster=local", nil)
	reg := fakeLister{"local", "prod"}

	id, err := ResolveCluster(req, reg)
	if err != nil {
		t.Errorf("ResolveCluster() returned err = %v, want nil", err)
	}
	if id != "local" {
		t.Errorf("ResolveCluster() returned %q, want %q", id, "local")
	}
}

func TestResolveClusterProdParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/servers?cluster=prod", nil)
	reg := fakeLister{"local", "prod"}

	id, err := ResolveCluster(req, reg)
	if err != nil {
		t.Errorf("ResolveCluster() returned err = %v, want nil", err)
	}
	if id != "prod" {
		t.Errorf("ResolveCluster() returned %q, want %q", id, "prod")
	}
}

func TestResolveClusterNotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/servers?cluster=ghost", nil)
	reg := fakeLister{"local", "prod"}

	id, err := ResolveCluster(req, reg)
	if !errors.Is(err, ErrForbiddenCluster) {
		t.Errorf("ResolveCluster() returned err = %v, want ErrForbiddenCluster", err)
	}
	if id != "" {
		t.Errorf("ResolveCluster() returned %q, want empty", id)
	}
}

func TestResolveClusterWhitespaceParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/servers?cluster=%20", nil)
	reg := fakeLister{"local", "prod"}

	id, err := ResolveCluster(req, reg)
	if err != nil {
		t.Errorf("ResolveCluster() returned err = %v, want nil", err)
	}
	if id != DefaultCluster {
		t.Errorf("ResolveCluster() returned %q (whitespace treated as empty), want %q", id, DefaultCluster)
	}
}
