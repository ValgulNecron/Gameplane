package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// modCredsBasePath is where resolved mod-portal credentials are mounted
// into the agent sidecar as subdirectories per provider.
const modCredsBasePath = "/etc/gameplane/mod-creds"

// resolvedModCreds describes the mod-portal credentials for a GameServer.
type resolvedModCreds struct {
	// providers maps provider name to its secret name (when a
	// CredentialsSecretRef is set).
	providers map[string]string
}

func resolveModCreds(tmpl *gameplanev1alpha1.GameTemplate) resolvedModCreds {
	r := resolvedModCreds{providers: make(map[string]string)}
	if tmpl.Spec.Capabilities == nil || tmpl.Spec.Capabilities.Mods == nil ||
		tmpl.Spec.Capabilities.Mods.Registry == nil {
		return r
	}
	for _, provider := range tmpl.Spec.Capabilities.Mods.Registry.Providers {
		if provider.CredentialsSecretRef != nil {
			r.providers[provider.Provider] = provider.CredentialsSecretRef.Name
		}
	}
	return r
}

// modCredVolumeMounts returns the agent sidecar volume mounts for
// mod-portal credentials, mounted as subdirectories under modCredsBasePath.
func modCredVolumeMounts(creds resolvedModCreds) []corev1.VolumeMount {
	if len(creds.providers) == 0 {
		return nil
	}
	mounts := make([]corev1.VolumeMount, 0, len(creds.providers))
	for provider := range creds.providers {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      modCredsVolumeName(provider),
			MountPath: fmt.Sprintf("%s/%s", modCredsBasePath, provider),
			ReadOnly:  true,
		})
	}
	return mounts
}

// modCredsVolumes returns the Secret volumes for mod-portal credentials.
func modCredsVolumes(creds resolvedModCreds) []corev1.Volume {
	if len(creds.providers) == 0 {
		return nil
	}
	volumes := make([]corev1.Volume, 0, len(creds.providers))
	for provider, secretName := range creds.providers {
		volumes = append(volumes, corev1.Volume{
			Name: modCredsVolumeName(provider),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
					Items: []corev1.KeyToPath{
						{Key: "username", Path: "username"},
						{Key: "token", Path: "token"},
					},
				},
			},
		})
	}
	return volumes
}

func modCredsVolumeName(provider string) string {
	return "mod-creds-" + provider
}
