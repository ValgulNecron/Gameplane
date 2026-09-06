package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func TestValidateServerEnvSecrets(t *testing.T) {
	s := testScheme(t)

	// 1. An unowned secret belonging to restic/subsystem
	unownedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restic-repo-backups",
			Namespace: "games",
		},
		Data: map[string][]byte{
			"password": []byte("sensitive-password"),
		},
	}

	// 2. A foreign same-named secret (matches server name suffix "-tunnel-auth", but NO ownerReference)
	foreignSameNamedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-tunnel-auth",
			Namespace: "games",
		},
		Data: map[string][]byte{
			"token": []byte("foreign-token"),
		},
	}

	// 3. A secret with user-settable label, but NO ownerReference
	labeledSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-labeled-secret",
			Namespace: "games",
			Labels: map[string]string{
				"gameplane.local/server-name": "alpha",
			},
		},
		Data: map[string][]byte{
			"data": []byte("labeled-data"),
		},
	}

	// 4. A secret with ownerReference pointing to a different GameServer UID
	mismatchedUIDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-stale-secret",
			Namespace: "games",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "gameplane.local/v1alpha1",
					Kind:       "GameServer",
					Name:       "alpha",
					UID:        "old-deleted-uid",
				},
			},
		},
		Data: map[string][]byte{
			"token": []byte("stale-token"),
		},
	}

	// 5. A valid server-owned secret with OwnerReference to alpha with matching UID
	ownedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-rcon",
			Namespace: "games",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "gameplane.local/v1alpha1",
					Kind:       "GameServer",
					Name:       "alpha",
					UID:        "alpha-uid",
				},
			},
		},
		Data: map[string][]byte{
			"password": []byte("rcon-pass"),
		},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(
		unownedSecret,
		foreignSameNamedSecret,
		labeledSecret,
		mismatchedUIDSecret,
		ownedSecret,
	).Build()
	r := &GameServerReconciler{Client: cl}

	ctx := context.Background()
	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha",
			Namespace: "games",
			UID:       types.UID("alpha-uid"),
		},
	}

	t.Run("unowned secret reference is refused", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name: "ATTACK",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "restic-repo-backups"},
						Key:                  "password",
					},
				},
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err == nil {
			t.Fatal("expected error for unowned secret reference, got nil")
		}
	})

	t.Run("foreign same-named secret without ownerReference is refused", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name: "ATTACK",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "alpha-tunnel-auth"},
						Key:                  "token",
					},
				},
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err == nil {
			t.Fatal("expected error for foreign same-named secret without ownerReference, got nil")
		}
	})

	t.Run("secret with user-settable label but no ownerReference is refused", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name: "ATTACK",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "alpha-labeled-secret"},
						Key:                  "data",
					},
				},
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err == nil {
			t.Fatal("expected error for labeled secret without ownerReference, got nil")
		}
	})

	t.Run("secret with mismatched UID is refused", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name: "ATTACK",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "alpha-stale-secret"},
						Key:                  "token",
					},
				},
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err == nil {
			t.Fatal("expected error for secret with mismatched UID, got nil")
		}
	})

	t.Run("server-owned secret with matching UID is permitted", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name: "RCON_PW",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "alpha-rcon"},
						Key:                  "password",
					},
				},
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err != nil {
			t.Fatalf("unexpected error for server-owned secret: %v", err)
		}
	})

	t.Run("literal env vars are permitted", func(t *testing.T) {
		gsCopy := gs.DeepCopy()
		gsCopy.Spec.Env = []corev1.EnvVar{
			{
				Name:  "EULA",
				Value: "TRUE",
			},
		}

		err := r.validateServerEnvSecrets(ctx, gsCopy)
		if err != nil {
			t.Fatalf("unexpected error for literal env vars: %v", err)
		}
	})
}

func TestReconcileTunnel_CredentialsSecretOwnership(t *testing.T) {
	s := testScheme(t)

	unownedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restic-repo-backups",
			Namespace: "games",
		},
		Data: map[string][]byte{"password": []byte("sensitive")},
	}

	ownedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha-tunnel-auth",
			Namespace: "games",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "gameplane.local/v1alpha1",
					Kind:       "GameServer",
					Name:       "alpha",
					UID:        "alpha-uid",
				},
			},
		},
		Data: map[string][]byte{"token": []byte("tunnel-token")},
	}

	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(unownedSecret, ownedSecret).Build()
	r := &GameServerReconciler{Client: cl, Scheme: s}

	gs := &gameplanev1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "alpha",
			Namespace: "games",
			UID:       types.UID("alpha-uid"),
		},
		Spec: gameplanev1alpha1.GameServerSpec{
			Networking: gameplanev1alpha1.GameServerNetworking{
				Tunnel: &gameplanev1alpha1.GameServerTunnel{
					Enabled:  true,
					Provider: "frp",
					Frp: &gameplanev1alpha1.FrpTunnelSpec{
						ServerAddr: "frp.example.com",
					},
					CredentialsSecretRef: &gameplanev1alpha1.SecretNameRef{
						Name: "restic-repo-backups",
					},
				},
			},
		},
	}

	tmpl := &gameplanev1alpha1.GameTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "minecraft"},
	}

	ctx := context.Background()

	// 1. Unowned secret must be refused
	err := r.reconcileTunnel(ctx, gs, tmpl, true)
	if err == nil {
		t.Fatal("expected error mounting unowned tunnel credentials secret, got nil")
	}

	// 2. Server-owned secret with matching UID must succeed
	gsOwned := gs.DeepCopy()
	gsOwned.Spec.Networking.Tunnel.CredentialsSecretRef.Name = "alpha-tunnel-auth"
	err = r.reconcileTunnel(ctx, gsOwned, tmpl, true)
	if err != nil {
		t.Fatalf("unexpected error mounting owned tunnel credentials secret: %v", err)
	}
}

