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

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

// rconPasswordPath is where the resolved RCON password is mounted into
// the agent sidecar; the agent reads it via --rcon-password-file.
const rconPasswordPath = "/etc/gameplane/rcon"

// rconSecretName is the operator-managed Secret holding a generated RCON
// password when the template doesn't reference an external one.
func rconSecretName(gs *gameplanev1alpha1.GameServer) string {
	return gs.Name + "-rcon"
}

// resolvedRCON describes where the RCON password lives for a GameServer:
// either an operator-generated Secret, the template's PasswordSecretRef, or
// a game-managed password file. enabled is false when the game exposes no RCON.
type resolvedRCON struct {
	enabled      bool
	secretName   string
	secretKey    string
	passwordFile string
	// passwordEnv is the game-container env var to inject the password
	// into, "" when the template doesn't declare one or when using passwordFile.
	passwordEnv string
}

func resolveRCON(gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate) resolvedRCON {
	if !templateHasRCON(tmpl) {
		return resolvedRCON{}
	}
	r := resolvedRCON{enabled: true, passwordEnv: tmpl.Spec.RCON.PasswordEnv}
	// Precedence: PasswordSecretRef > PasswordFile > operator-generated Secret
	if ref := tmpl.Spec.RCON.PasswordSecretRef; ref != nil {
		r.secretName = ref.Name
		r.secretKey = ref.Key
	} else if tmpl.Spec.RCON.PasswordFile != "" {
		r.passwordFile = tmpl.Spec.RCON.PasswordFile
		r.passwordEnv = ""
	} else {
		r.secretName = rconSecretName(gs)
		r.secretKey = "password"
	}
	return r
}

// reconcileRCONSecret ensures an RCON password exists for the GameServer.
// When the template declares RCON, doesn't reference an external Secret, and
// doesn't use a game-managed password file, the operator generates a random
// password into <gs>-rcon and preserves it across reconciles. Templates that
// reference their own Secret, use a password file, or expose no RCON, get
// nothing (the <gs>-rcon Secret is removed when RCON is disabled, same
// lifecycle as the config Secret).
func (r *GameServerReconciler) reconcileRCONSecret(
	ctx context.Context, gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate,
) error {
	rc := resolveRCON(gs, tmpl)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: rconSecretName(gs), Namespace: gs.Namespace},
	}
	// Only manage <gs>-rcon ourselves when RCON is enabled, no external
	// PasswordSecretRef is set, and no game-managed password file is used.
	if !rc.enabled || rc.secretName == "" {
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
// RCON password mount when the game exposes RCON and doesn't use a game-
// managed password file, the per-(version+loader) mod volume when the active
// version selects one, and per-provider mod-portal credential mounts. The agent
// must see the same mounted mod dir the game reads so the Mods tab operates on it.
func agentVolumeMounts(gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate, ver *gameplanev1alpha1.GameVersion, dataMount string) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{Name: "data", MountPath: dataMount},
		{Name: "agent-tls", MountPath: "/etc/gameplane/agent-tls", ReadOnly: true},
	}
	rc := resolveRCON(gs, tmpl)
	if rc.enabled && rc.passwordFile == "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name: "rcon-password", MountPath: rconPasswordPath, ReadOnly: true,
		})
	}
	if m := modVolumeMount(tmpl, ver); m != nil {
		mounts = append(mounts, *m)
	}
	mounts = append(mounts, modCredVolumeMounts(resolveModCreds(tmpl))...)
	return mounts
}

// rconGameEnv returns the env var that injects the resolved RCON password
// into the game container, or nil when the template declares no PasswordEnv
// or uses a game-managed password file. Appended last so it overrides any
// user-set value, keeping the game and agent in agreement on the password.
func rconGameEnv(gs *gameplanev1alpha1.GameServer, tmpl *gameplanev1alpha1.GameTemplate) *corev1.EnvVar {
	rc := resolveRCON(gs, tmpl)
	if !rc.enabled || rc.passwordEnv == "" || rc.passwordFile != "" {
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
