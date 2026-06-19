package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

// rconPasswordPath is where the resolved RCON password is mounted into
// the agent sidecar; the agent reads it via --rcon-password-file.
const rconPasswordPath = "/etc/kestrel/rcon"

// rconSecretName is the operator-managed Secret holding a generated RCON
// password when the template doesn't reference an external one.
func rconSecretName(gs *kestrelv1alpha1.GameServer) string {
	return gs.Name + "-rcon"
}

// resolvedRCON describes where the RCON password lives for a GameServer:
// either an operator-generated Secret or the template's PasswordSecretRef.
// enabled is false when the game exposes no RCON.
type resolvedRCON struct {
	enabled    bool
	secretName string
	secretKey  string
	// passwordEnv is the game-container env var to inject the password
	// into, "" when the template doesn't declare one.
	passwordEnv string
}

func resolveRCON(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate) resolvedRCON {
	if !templateHasRCON(tmpl) {
		return resolvedRCON{}
	}
	r := resolvedRCON{enabled: true, passwordEnv: tmpl.Spec.RCON.PasswordEnv}
	if ref := tmpl.Spec.RCON.PasswordSecretRef; ref != nil {
		r.secretName = ref.Name
		r.secretKey = ref.Key
	} else {
		r.secretName = rconSecretName(gs)
		r.secretKey = "password"
	}
	return r
}

// reconcileRCONSecret ensures an RCON password exists for the GameServer.
// When the template declares RCON and doesn't point at an external
// Secret, the operator generates a random password into <gs>-rcon and
// preserves it across reconciles. Templates that reference their own
// Secret, or expose no RCON, get nothing (the <gs>-rcon Secret is removed
// when RCON is disabled, same lifecycle as the config Secret).
func (r *GameServerReconciler) reconcileRCONSecret(
	ctx context.Context, gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate,
) error {
	rc := resolveRCON(gs, tmpl)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: rconSecretName(gs), Namespace: gs.Namespace},
	}
	// Only manage <gs>-rcon ourselves; an external PasswordSecretRef is
	// the user's to provide.
	if !rc.enabled || tmpl.Spec.RCON.PasswordSecretRef != nil {
		return client.IgnoreNotFound(r.Delete(ctx, sec))
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		if len(sec.Data["password"]) == 0 {
			pw, gerr := generatePassword()
			if gerr != nil {
				return gerr
			}
			if sec.Data == nil {
				sec.Data = map[string][]byte{}
			}
			sec.Data["password"] = []byte(pw)
		}
		return controllerutil.SetControllerReference(gs, sec, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconcile rcon Secret %s/%s: %w", gs.Namespace, sec.Name, err)
	}
	return nil
}

// agentVolumeMounts returns the agent sidecar's volume mounts, adding the
// RCON password mount when the game exposes RCON and the per-(version+
// loader) mod volume when the active version selects one — the agent must
// see the same mounted mod dir the game reads so the Mods tab operates on
// it.
func agentVolumeMounts(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate, ver *kestrelv1alpha1.GameVersion, dataMount string) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: dataMount},
		{Name: "agent-tls", MountPath: "/etc/kestrel/agent-tls", ReadOnly: true},
	}
	if resolveRCON(gs, tmpl).enabled {
		mounts = append(mounts, corev1.VolumeMount{
			Name: "rcon-password", MountPath: rconPasswordPath, ReadOnly: true,
		})
	}
	if m := modVolumeMount(tmpl, ver); m != nil {
		mounts = append(mounts, *m)
	}
	return mounts
}

// rconGameEnv returns the env var that injects the resolved RCON password
// into the game container, or nil when the template declares no
// PasswordEnv. Appended last so it overrides any user-set value, keeping
// the game and agent in agreement on the password.
func rconGameEnv(gs *kestrelv1alpha1.GameServer, tmpl *kestrelv1alpha1.GameTemplate) *corev1.EnvVar {
	rc := resolveRCON(gs, tmpl)
	if !rc.enabled || rc.passwordEnv == "" {
		return nil
	}
	return &corev1.EnvVar{
		Name: rc.passwordEnv,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: rc.secretName},
				Key:                  rc.secretKey,
			},
		},
	}
}

// generatePassword returns a 32-hex-char (128-bit) random password.
func generatePassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate rcon password: %w", err)
	}
	return hex.EncodeToString(b), nil
}
