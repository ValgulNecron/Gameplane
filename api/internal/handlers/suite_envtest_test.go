//go:build envtest

package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/ValgulNecron/gameplane/api/internal/kube"
	"github.com/ValgulNecron/gameplane/api/internal/scope"
)

// envtest fires up a real kube-apiserver+etcd in-process. We then build
// the same kube.Client + chi router the production main wires, minus
// the auth/RBAC/audit middleware (those live in their own packages and
// have their own unit tests). Tests assert that handlers correctly
// shape requests at the apiserver.
//
// Per-test isolation: the suite uses a single shared namespace
// (scope.DefaultNamespace) because that namespace is hard-coded into
// the scope package at init time and can't be changed at runtime
// without touching the package. Tests therefore use unique resource
// names per test (uniqueResourceName) to avoid collisions.

var (
	testEnv  *envtest.Environment
	cfg      *rest.Config
	kubeC    *kube.Client
	apiSrv   *httptest.Server
	apiBase  string
	mountedR *chi.Mux
)

func TestMain(m *testing.M) {
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "operator", "config", "crd")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic("envtest start: " + err.Error())
	}

	kubeC, err = kube.New(cfg)
	if err != nil {
		_ = testEnv.Stop()
		panic("kube client: " + err.Error())
	}

	if _, err := kubeC.Typed.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: scope.DefaultNamespace}},
		metav1.CreateOptions{},
	); err != nil {
		_ = testEnv.Stop()
		panic("create games namespace: " + err.Error())
	}

	// reg dispatches MountResources' cluster-aware routes: kubeC as the
	// default "local" cluster (the real envtest apiserver), plus a second,
	// empty "other" cluster backed by a fake dynamic client so
	// TestResources_ClusterDispatch_Isolation can prove that a `?cluster=`
	// selector never leaks objects across clusters. Every other Mount*
	// call below still takes kubeC directly (Phase 2 migrates them).
	reg := kube.NewRegistry(scope.DefaultCluster)
	reg.Set(scope.DefaultCluster, kubeC)
	fakeScheme := runtime.NewScheme()
	fakeDyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(fakeScheme, map[schema.GroupVersionResource]string{
		kube.GVRs["servers"]:   "GameServerList",
		kube.GVRs["templates"]: "GameTemplateList",
		kube.GVRs["backups"]:   "BackupList",
		kube.GVRs["schedules"]: "BackupScheduleList",
		kube.GVRs["restores"]:  "RestoreList",
	})
	reg.Set("other", &kube.Client{Dynamic: fakeDyn, Typed: k8sfake.NewSimpleClientset()})

	mountedR = chi.NewRouter()
	MountResources(mountedR, reg)
	MountLifecycle(mountedR, reg)
	MountDestinations(mountedR, reg)
	MountModules(mountedR, kubeC, "default")

	apiSrv = httptest.NewServer(mountedR)
	apiBase = apiSrv.URL

	code := m.Run()

	apiSrv.Close()
	_ = testEnv.Stop()
	os.Exit(code)
}

// uniqueResourceName generates a short, lowercase, K8s-DNS-friendly
// suffix on the given prefix. Tests share a namespace so names must
// not collide across tests.
func uniqueResourceName(prefix string) string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return prefix + "-" + hex.EncodeToString(buf)
}
