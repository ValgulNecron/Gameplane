package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gameplanev1alpha1 "github.com/ValgulNecron/gameplane/operator/api/v1alpha1"
)

func rconGS() *gameplanev1alpha1.GameServer {
	return &gameplanev1alpha1.GameServer{ObjectMeta: metav1.ObjectMeta{Name: "smp", Namespace: "ns"}}
}

func rconTmpl(spec *gameplanev1alpha1.RCONSpec) *gameplanev1alpha1.GameTemplate {
	return &gameplanev1alpha1.GameTemplate{Spec: gameplanev1alpha1.GameTemplateSpec{RCON: spec}}
}

func TestResolveRCON(t *testing.T) {
	gs := rconGS()

	t.Run("no rcon → disabled", func(t *testing.T) {
		if resolveRCON(gs, rconTmpl(nil)).enabled {
			t.Fatal("expected disabled")
		}
		if resolveRCON(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{Protocol: "none"})).enabled {
			t.Fatal("protocol none must be disabled")
		}
	})

	t.Run("generated secret by default", func(t *testing.T) {
		rc := resolveRCON(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{Protocol: "source", PasswordEnv: "RCON_PASSWORD"}))
		if !rc.enabled || rc.secretName != "smp-rcon" || rc.secretKey != "password" {
			t.Fatalf("got %+v", rc)
		}
		if rc.passwordEnv != "RCON_PASSWORD" {
			t.Fatalf("passwordEnv = %q", rc.passwordEnv)
		}
	})

	t.Run("external PasswordSecretRef wins", func(t *testing.T) {
		rc := resolveRCON(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{
			Protocol:          "source",
			PasswordSecretRef: &gameplanev1alpha1.SecretKeySelector{Name: "my-secret", Key: "pw"},
		}))
		if rc.secretName != "my-secret" || rc.secretKey != "pw" {
			t.Fatalf("got %+v", rc)
		}
	})

	t.Run("passwordFile sets file mode", func(t *testing.T) {
		rc := resolveRCON(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{
			Protocol:     "source",
			PasswordFile: "config/rconpw",
		}))
		if !rc.enabled || rc.passwordFile != "config/rconpw" {
			t.Fatalf("got %+v", rc)
		}
		if rc.secretName != "" || rc.secretKey != "" {
			t.Fatalf("passwordFile mode should not set secretName/secretKey: %+v", rc)
		}
		if rc.passwordEnv != "" {
			t.Fatalf("passwordFile mode should clear passwordEnv, got %q", rc.passwordEnv)
		}
	})

	t.Run("PasswordSecretRef wins over PasswordFile", func(t *testing.T) {
		rc := resolveRCON(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{
			Protocol:          "source",
			PasswordSecretRef: &gameplanev1alpha1.SecretKeySelector{Name: "my-secret", Key: "pw"},
			PasswordFile:      "config/rconpw",
		}))
		if rc.secretName != "my-secret" || rc.secretKey != "pw" {
			t.Fatalf("got %+v", rc)
		}
		if rc.passwordFile != "" {
			t.Fatalf("PasswordSecretRef should win over PasswordFile: %+v", rc)
		}
	})
}

func TestRCONGameEnv(t *testing.T) {
	gs := rconGS()
	// No passwordEnv → no env injected.
	if rconGameEnv(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{Protocol: "source"})) != nil {
		t.Fatal("no passwordEnv should inject nothing")
	}
	e := rconGameEnv(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{Protocol: "source", PasswordEnv: "RCON_PASSWORD"}))
	if e == nil || e.Name != "RCON_PASSWORD" || e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
		t.Fatalf("got %+v", e)
	}
	if e.ValueFrom.SecretKeyRef.Name != "smp-rcon" || e.ValueFrom.SecretKeyRef.Key != "password" {
		t.Fatalf("secretKeyRef = %+v", e.ValueFrom.SecretKeyRef)
	}
	// PasswordFile mode → no env injected even if PasswordEnv is set.
	if rconGameEnv(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{
		Protocol:     "source",
		PasswordEnv:  "RCON_PASSWORD",
		PasswordFile: "config/rconpw",
	})) != nil {
		t.Fatal("passwordFile mode should not inject env")
	}
}

func TestAgentVolumeMounts(t *testing.T) {
	gs := rconGS()
	base := agentVolumeMounts(gs, rconTmpl(nil), nil, "/data")
	if len(base) != 2 {
		t.Fatalf("non-rcon agent should have 2 mounts, got %d", len(base))
	}
	withRCON := agentVolumeMounts(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{Protocol: "source"}), nil, "/data")
	found := false
	for _, m := range withRCON {
		if m.Name == "rcon-password" && m.MountPath == rconPasswordPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("rcon agent missing rcon-password mount: %+v", withRCON)
	}
	// PasswordFile mode should not mount rcon-password volume
	withFile := agentVolumeMounts(gs, rconTmpl(&gameplanev1alpha1.RCONSpec{
		Protocol:     "source",
		PasswordFile: "config/rconpw",
	}), nil, "/data")
	for _, m := range withFile {
		if m.Name == "rcon-password" {
			t.Fatalf("passwordFile mode should not mount rcon-password volume: %+v", withFile)
		}
	}
}

func TestGeneratePassword(t *testing.T) {
	a, err := generatePassword()
	if err != nil || len(a) != 32 {
		t.Fatalf("password = %q err=%v", a, err)
	}
	b, _ := generatePassword()
	if a == b {
		t.Fatal("two generated passwords should differ")
	}
}
