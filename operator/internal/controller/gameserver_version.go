package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// modVolumeDefaultSize is the size of each per-(version+loader) mod PVC.
// Mod/plugin sets are small relative to world data; 5Gi comfortably holds
// a large modpack while keeping the per-combo footprint modest.
var modVolumeDefaultSize = resource.MustParse("5Gi")

// resolveVersion returns the GameVersion the server selected, the
// template's default version, or nil when the template declares none. It
// errors when the template declares versions and spec.version names one
// that doesn't exist, so a bad version fails the GameServer loudly instead
// of silently falling back — keeping kubectl-apply and the wizard in
// lockstep (Rule 10).
func resolveVersion(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate) (*kestrelv1alpha1.GameVersion, error) {
	vers := tmpl.Spec.Versions
	if len(vers) == 0 {
		return nil, nil
	}
	if gs.Spec.Version != "" {
		for i := range vers {
			if vers[i].ID == gs.Spec.Version {
				return &vers[i], nil
			}
		}
		return nil, fmt.Errorf("unknown version %q", gs.Spec.Version)
	}
	// No explicit choice: the entry marked default, else the first.
	for i := range vers {
		if vers[i].Default {
			return &vers[i], nil
		}
	}
	return &vers[0], nil
}

// resolveImage picks the game container image: an explicit spec.image
// override wins, then the selected version's image, then the template's
// Image (today's behavior when no versions are declared).
func resolveImage(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) string {
	if gs.Spec.Image != "" {
		return gs.Spec.Image
	}
	if ver != nil && ver.Image != "" {
		return ver.Image
	}
	return tmpl.Spec.Image
}

// activeLoader returns the selected version's mod loader, or "".
func activeLoader(ver *kestrelv1alpha1.GameVersion) string {
	if ver == nil {
		return ""
	}
	return ver.Loader
}

// activeModLoader returns the ModLoaderSpec for the active version's
// loader and true when the template declares a per-loader mod volume for
// it. It reports false for legacy single-path mods, a version whose loader
// has no mods entry (e.g. vanilla), or no mods block at all.
func activeModLoader(tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) (kestrelv1alpha1.ModLoaderSpec, bool) {
	if tmpl.Spec.Capabilities == nil || tmpl.Spec.Capabilities.Mods == nil {
		return kestrelv1alpha1.ModLoaderSpec{}, false
	}
	loader := activeLoader(ver)
	if loader == "" || len(tmpl.Spec.Capabilities.Mods.Loaders) == 0 {
		return kestrelv1alpha1.ModLoaderSpec{}, false
	}
	spec, ok := tmpl.Spec.Capabilities.Mods.Loaders[loader]
	return spec, ok
}

// modVolumeKey is the stable per-(version+loader) key, or "" when this
// server has no dedicated mod volume. The version id already encodes
// version+loader, so it is the key.
func modVolumeKey(tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) string {
	if _, ok := activeModLoader(tmpl, ver); !ok {
		return ""
	}
	return ver.ID
}

// dnsSafe lowercases and replaces characters invalid in a DNS-1123 label
// (e.g. the dots in "1.21.4-paper") with hyphens.
func dnsSafe(s string) string {
	return strings.ToLower(strings.NewReplacer(".", "-", "_", "-").Replace(s))
}

// truncateDNS keeps names within max, appending a short content hash so
// distinct keys don't collide after truncation.
func truncateDNS(s string, max int) string {
	if len(s) <= max {
		return s
	}
	sum := sha256.Sum256([]byte(s))
	suffix := "-" + hex.EncodeToString(sum[:])[:8]
	return strings.TrimRight(s[:max-len(suffix)], "-") + suffix
}

// modVolumeName is the pod volume name for a mod combo (DNS-1123 label).
func modVolumeName(key string) string {
	return truncateDNS("mods-"+dnsSafe(key), 63)
}

// modPVCName is the PVC name backing a server's mod combo.
func modPVCName(gs *kestrelv1alpha1.GameServer, key string) string {
	return truncateDNS(gs.Name+"-mods-"+dnsSafe(key), 253)
}

// modVolumeMount returns the game/agent container mount for the active mod
// combo, or nil when the server has no dedicated mod volume. The PVC is
// mounted at storage.mountPath/<loaderPath> — a nested mount over the data
// volume so the game image reads mods from its usual directory while the
// files persist on a per-combo PVC.
func modVolumeMount(tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) *corev1.VolumeMount {
	spec, ok := activeModLoader(tmpl, ver)
	if !ok {
		return nil
	}
	return &corev1.VolumeMount{
		Name:      modVolumeName(ver.ID),
		MountPath: path.Join(effectiveMountPath(tmpl), spec.Path),
	}
}

// modVolume returns the pod volume backing the active mod combo, or nil.
func modVolume(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) *corev1.Volume {
	key := modVolumeKey(tmpl, ver)
	if key == "" {
		return nil
	}
	return &corev1.Volume{
		Name: modVolumeName(key),
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: modPVCName(gs, key),
			},
		},
	}
}

// resolveCapabilities returns the capabilities the agent should run with
// for this server. The agent is loader-agnostic — it just manages files at
// Mods.Path — so this collapses the per-loader mods map into the single
// concrete Path for the active version's loader and drops the map. A
// version whose loader has no mod volume (e.g. vanilla) gets no mod
// manager at all. Returns nil when the template declares no capabilities.
func resolveCapabilities(tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion) *kestrelv1alpha1.CapabilitiesSpec {
	if tmpl.Spec.Capabilities == nil {
		return nil
	}
	caps := tmpl.Spec.Capabilities.DeepCopy()
	if caps.Mods == nil || len(caps.Mods.Loaders) == 0 {
		// No mods, or legacy single-path mods (Path already concrete).
		return caps
	}
	spec, ok := activeModLoader(tmpl, ver)
	if !ok {
		// This version's loader has no mod volume — no mod manager.
		caps.Mods = nil
		return caps
	}
	caps.Mods.Path = spec.Path
	if len(spec.Extensions) > 0 {
		caps.Mods.Extensions = spec.Extensions
	}
	caps.Mods.Extract = spec.Extract
	caps.Mods.Loaders = nil
	return caps
}

// reconcileModPVC provisions the per-(version+loader) mod volume for the
// server's ACTIVE version. Inactive combos' PVCs are deliberately left
// untouched so switching versions preserves every combo's mod set; they're
// garbage-collected (via owner reference) only when the GameServer is
// deleted.
func (r *GameServerReconciler) reconcileModPVC(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
	ver *kestrelv1alpha1.GameVersion,
) error {
	key := modVolumeKey(tmpl, ver)
	if key == "" {
		return nil
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: modPVCName(gs, key), Namespace: gs.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if pvc.CreationTimestamp.IsZero() {
			pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
			pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: modVolumeDefaultSize}
			if gs.Spec.Storage != nil && gs.Spec.Storage.StorageClassName != nil {
				pvc.Spec.StorageClassName = gs.Spec.Storage.StorageClassName
			} else if tmpl.Spec.Storage.StorageClassName != nil {
				pvc.Spec.StorageClassName = tmpl.Spec.Storage.StorageClassName
			}
		}
		return controllerutil.SetControllerReference(gs, pvc, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile mod PVC %s/%s: %w", gs.Namespace, pvc.Name, err)
	}
	return nil
}
