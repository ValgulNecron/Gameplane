//go:build envtest

package controller

import (
	"context"
	"fmt"
	"sync"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
	"github.com/kestrel-gg/kestrel/operator/internal/oci"
)

// fakeOCI is an in-process replacement for the OCI client used by the
// ModuleSource and Module reconcilers. Tests pre-populate Tags +
// Bundles by reference, and inspect call counts to verify the
// reconciler's caching/back-off behavior.
type fakeOCI struct {
	mu      sync.Mutex
	tags    map[string][]string
	bundles map[string]map[string]*oci.Bundle // ref → version → bundle
	pulls   int
	errOn   map[string]error
}

func newFakeOCI() *fakeOCI {
	return &fakeOCI{
		tags:    map[string][]string{},
		bundles: map[string]map[string]*oci.Bundle{},
		errOn:   map[string]error{},
	}
}

func (f *fakeOCI) ListTags(_ context.Context, ref string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.errOn["tags:"+ref]; ok {
		return nil, err
	}
	// Mirror Client.ListTags: descending semver order.
	out := append([]string(nil), f.tags[ref]...)
	sortSemverDescending(out)
	return out, nil
}

func sortSemverDescending(tags []string) {
	for i := 0; i < len(tags); i++ {
		for j := i + 1; j < len(tags); j++ {
			if !semverDescending(tags[i], tags[j]) {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}
}

func (f *fakeOCI) Pull(_ context.Context, ref, version string) (*oci.Bundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pulls++
	if err, ok := f.errOn["pull:"+ref+":"+version]; ok {
		return nil, err
	}
	if m, ok := f.bundles[ref]; ok {
		if b, ok := m[version]; ok {
			return b, nil
		}
	}
	return nil, fmt.Errorf("fakeOCI: no bundle at %s:%s", ref, version)
}

func (f *fakeOCI) putBundle(ref, version string, b *oci.Bundle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.bundles[ref]; !ok {
		f.bundles[ref] = map[string]*oci.Bundle{}
	}
	f.bundles[ref][version] = b
	// Append tag if new.
	for _, t := range f.tags[ref] {
		if t == version {
			return
		}
	}
	f.tags[ref] = append(f.tags[ref], version)
}

func withModuleSourceReconciler(fake *fakeOCI) setupReconciler {
	return func(mgr manager.Manager) error {
		return (&ModuleSourceReconciler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
			NewClient: func(_ oci.CredentialFunc, _ bool) ociClient {
				return fake
			},
		}).SetupWithManager(mgr)
	}
}

// fixtureBundle constructs a Bundle whose Metadata matches the given
// name+version. Just enough to populate ModuleEntry status fields.
func fixtureBundle(name, version, displayName string) *oci.Bundle {
	return &oci.Bundle{
		Digest: "sha256:" + name + "-" + version,
		Metadata: oci.Metadata{
			APIVersion:  "kestrel.gg/module/v1",
			Name:        name,
			DisplayName: displayName,
			Version:     version,
			Game:        name,
			Summary:     displayName + " — test fixture",
		},
		TemplateYAML: []byte("apiVersion: kestrel.gg/v1alpha1\nkind: GameTemplate\n" +
			"spec:\n  displayName: " + displayName + "\n  game: " + name +
			"\n  version: " + version + "\n  image: ghcr.io/test/" + name + ":" + version + "\n"),
	}
}

// TestModuleSource_IndexesCatalog — happy path: status.modules
// reflects the bundles the registry has, sorted descending.
func TestModuleSource_IndexesCatalog(t *testing.T) {
	_ = newNamespace(t) // doesn't matter; ModuleSource is cluster-scoped
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleSourceReconciler(fake))

	// Pre-populate the fake registry with two modules, two versions each.
	fake.putBundle("local/test/minecraft-java", "1.0.0", fixtureBundle("minecraft-java", "1.0.0", "Minecraft (Java)"))
	fake.putBundle("local/test/minecraft-java", "1.1.0", fixtureBundle("minecraft-java", "1.1.0", "Minecraft (Java)"))
	fake.putBundle("local/test/valheim", "0.9.0", fixtureBundle("valheim", "0.9.0", "Valheim"))

	src := &kestrelv1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("indexed")},
		Spec: kestrelv1alpha1.ModuleSourceSpec{
			URL: "local/test",
			Modules: []kestrelv1alpha1.ModuleRef{
				{Name: "minecraft-java"},
				{Name: "valheim"},
			},
		},
	}
	if err := k8sClient.Create(context.Background(), src); err != nil {
		t.Fatalf("create modulesource: %v", err)
	}
	deleteCleanup(t, src)

	eventually(t, func() (bool, string) {
		got := getModuleSource(t, src.Name)
		if len(got.Status.Modules) != 2 {
			return false, fmt.Sprintf("got %d modules", len(got.Status.Modules))
		}
		mc := byName(got.Status.Modules, "minecraft-java")
		if mc == nil || mc.LatestVersion != "1.1.0" {
			return false, fmt.Sprintf("minecraft-java latest = %v", mc)
		}
		if len(mc.Versions) != 2 || mc.Versions[0] != "1.1.0" || mc.Versions[1] != "1.0.0" {
			return false, fmt.Sprintf("minecraft-java versions = %v", mc.Versions)
		}
		if mc.DisplayName != "Minecraft (Java)" {
			return false, fmt.Sprintf("minecraft-java displayName = %q", mc.DisplayName)
		}
		vh := byName(got.Status.Modules, "valheim")
		if vh == nil || vh.LatestVersion != "0.9.0" {
			return false, fmt.Sprintf("valheim latest = %v", vh)
		}
		return conditionTrue(got.Status.Conditions, kestrelv1alpha1.ModuleSourceConditionSynced),
			"Synced condition not True yet"
	})
}

// TestModuleSource_KeepsPartialCatalogOnError — when one module fails
// to index, the others still appear in status.
func TestModuleSource_KeepsPartialCatalogOnError(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleSourceReconciler(fake))

	fake.putBundle("local/test/good", "1.0.0", fixtureBundle("good", "1.0.0", "Good"))
	fake.errOn["tags:local/test/broken"] = fmt.Errorf("simulated registry error")

	src := &kestrelv1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("partial")},
		Spec: kestrelv1alpha1.ModuleSourceSpec{
			URL:     "local/test",
			Modules: []kestrelv1alpha1.ModuleRef{{Name: "good"}, {Name: "broken"}},
		},
	}
	if err := k8sClient.Create(context.Background(), src); err != nil {
		t.Fatalf("create: %v", err)
	}
	deleteCleanup(t, src)

	eventually(t, func() (bool, string) {
		got := getModuleSource(t, src.Name)
		good := byName(got.Status.Modules, "good")
		broken := byName(got.Status.Modules, "broken")
		if good == nil || good.LatestVersion != "1.0.0" {
			return false, fmt.Sprintf("good = %+v", good)
		}
		if broken == nil || len(broken.Versions) != 0 {
			return false, fmt.Sprintf("broken = %+v", broken)
		}
		return true, ""
	})
}

func getModuleSource(t *testing.T, name string) *kestrelv1alpha1.ModuleSource {
	t.Helper()
	var src kestrelv1alpha1.ModuleSource
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name}, &src); err != nil {
		t.Fatalf("get modulesource: %v", err)
	}
	return &src
}

func byName(entries []kestrelv1alpha1.ModuleEntry, name string) *kestrelv1alpha1.ModuleEntry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

func conditionTrue(conds []metav1.Condition, t string) bool {
	for _, c := range conds {
		if c.Type == t {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

// TestModuleSource_ReportsFailureWhenAllModulesError — when every
// module fails to index (registry unreachable), the source must not
// publish a catalog of empty stubs as if it were healthy: modules stay
// empty, lastSync records the attempt, and Synced/Ready go False.
func TestModuleSource_ReportsFailureWhenAllModulesError(t *testing.T) {
	_ = newNamespace(t)
	fake := newFakeOCI()
	startMgr(t, "kestrel-system", withModuleSourceReconciler(fake))

	fake.errOn["tags:unreachable/test/ghost"] = fmt.Errorf("dial tcp: no such host")

	src := &kestrelv1alpha1.ModuleSource{
		ObjectMeta: metav1.ObjectMeta{Name: uniqueName("unreachable")},
		Spec: kestrelv1alpha1.ModuleSourceSpec{
			URL:     "unreachable/test",
			Modules: []kestrelv1alpha1.ModuleRef{{Name: "ghost"}},
		},
	}
	if err := k8sClient.Create(context.Background(), src); err != nil {
		t.Fatalf("create: %v", err)
	}
	deleteCleanup(t, src)

	eventually(t, func() (bool, string) {
		got := getModuleSource(t, src.Name)
		if got.Status.LastSync == nil {
			return false, "lastSync not recorded"
		}
		if len(got.Status.Modules) != 0 {
			return false, fmt.Sprintf("modules unexpectedly populated: %d", len(got.Status.Modules))
		}
		if conditionTrue(got.Status.Conditions, kestrelv1alpha1.ModuleSourceConditionSynced) {
			return false, "Synced should be False"
		}
		if conditionTrue(got.Status.Conditions, kestrelv1alpha1.ModuleSourceConditionReady) {
			return false, "Ready should be False"
		}
		return true, ""
	})
}
