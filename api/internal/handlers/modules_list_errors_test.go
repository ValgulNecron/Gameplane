package handlers

import (
	"errors"
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

// dynamicWithListError builds a fake dynamic client that errors out on
// the given resource's list verb. Used to drive the handler error paths.
func dynamicWithListError(resource string, err error) *dynamicfake.FakeDynamicClient {
	c := fakeKubeClient().Dynamic.(*dynamicfake.FakeDynamicClient)
	c.PrependReactor("list", resource, func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return c
}

func TestListSources_DBError(t *testing.T) {
	k := fakeKubeClient()
	k.Dynamic = dynamicWithListError("modulesources", errors.New("internal"))
	r := mountModulesRouter(k)
	rr := do(t, r, "GET", "/modules/sources", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
}

func TestListInstalled_DBError(t *testing.T) {
	k := fakeKubeClient()
	k.Dynamic = dynamicWithListError("modules", errors.New("internal"))
	r := mountModulesRouter(k)
	rr := do(t, r, "GET", "/modules/", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d %s", rr.Code, rr.Body)
	}
}

// TestCatalog_SourcesListError covers the early-return when listing
// ModuleSources fails inside catalog().
func TestCatalog_SourcesListError(t *testing.T) {
	k := fakeKubeClient()
	k.Dynamic = dynamicWithListError("modulesources", errors.New("internal"))
	r := mountModulesRouter(k)
	rr := do(t, r, "GET", "/modules/catalog", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}

// TestCatalog_ModulesListError covers the second list-error branch.
func TestCatalog_ModulesListError(t *testing.T) {
	k := fakeKubeClient()
	c := k.Dynamic.(*dynamicfake.FakeDynamicClient)
	c.PrependReactor("list", "modules", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("internal")
	})
	r := mountModulesRouter(k)
	rr := do(t, r, "GET", "/modules/catalog", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rr.Code)
	}
}
