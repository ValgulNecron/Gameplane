package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kestrelv1alpha1 "github.com/kestrel-gg/kestrel/operator/api/v1alpha1"
)

func configFixtures(
	schema []kestrelv1alpha1.ConfigField, config map[string]string,
) (*kestrelv1alpha1.GameServer, *kestrelv1alpha1.GameTemplate) {
	gs := &kestrelv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{Name: "smp", Namespace: "games"},
		Spec:       kestrelv1alpha1.GameServerSpec{Config: config},
	}
	tmpl := &kestrelv1alpha1.GameTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "minecraft"},
		Spec:       kestrelv1alpha1.GameTemplateSpec{ConfigSchema: schema},
	}
	return gs, tmpl
}

func envMap(env []corev1.EnvVar) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}

func TestMaterializeConfig_ResolvesValuesAndDefaults(t *testing.T) {
	gs, tmpl := configFixtures([]kestrelv1alpha1.ConfigField{
		{Name: "TYPE", Type: "enum", Enum: []string{"VANILLA", "PAPER"}, Default: "VANILLA", Required: true},
		{Name: "VERSION", Type: "string", Default: "LATEST", Required: true},
		{Name: "MAX_PLAYERS", Type: "int", Default: "20"},
		{Name: "HARDCORE", Type: "bool"},
		{Name: "MOTD", Type: "string"},
	}, map[string]string{
		"TYPE":     "PAPER",
		"HARDCORE": "true",
	})

	mc, err := materializeConfig(gs, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	got := envMap(mc.env)
	want := map[string]string{
		"TYPE":        "PAPER",  // explicit value wins over default
		"VERSION":     "LATEST", // default applied when key absent
		"MAX_PLAYERS": "20",     // optional default still applied
		"HARDCORE":    "true",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("env %s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["MOTD"]; ok {
		t.Errorf("empty optional field MOTD should not produce an env var")
	}
	if len(mc.env) != len(want) {
		t.Errorf("got %d env vars, want %d: %v", len(mc.env), len(want), got)
	}
	if mc.hash == "" {
		t.Errorf("hash should be set when config materializes")
	}
}

func TestMaterializeConfig_SchemaOrderIsDeterministic(t *testing.T) {
	gs, tmpl := configFixtures([]kestrelv1alpha1.ConfigField{
		{Name: "B", Type: "string", Default: "2"},
		{Name: "A", Type: "string", Default: "1"},
	}, nil)

	mc, err := materializeConfig(gs, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	if len(mc.env) != 2 || mc.env[0].Name != "B" || mc.env[1].Name != "A" {
		t.Fatalf("env should follow schema order, got %v", mc.env)
	}
}

func TestMaterializeConfig_HashTracksValues(t *testing.T) {
	schema := []kestrelv1alpha1.ConfigField{{Name: "DIFFICULTY", Type: "string", Default: "easy"}}

	gs1, tmpl := configFixtures(schema, map[string]string{"DIFFICULTY": "hard"})
	gs2, _ := configFixtures(schema, map[string]string{"DIFFICULTY": "easy"})
	gs3, _ := configFixtures(schema, nil) // default resolves to "easy" too

	mc1, err := materializeConfig(gs1, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	mc2, err := materializeConfig(gs2, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	mc3, err := materializeConfig(gs3, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	if mc1.hash == mc2.hash {
		t.Errorf("different values should hash differently")
	}
	if mc2.hash != mc3.hash {
		t.Errorf("identical resolved values should hash identically")
	}
}

func TestMaterializeConfig_NoSchemaNoConfig(t *testing.T) {
	gs, tmpl := configFixtures(nil, nil)
	mc, err := materializeConfig(gs, tmpl)
	if err != nil {
		t.Fatalf("materializeConfig: %v", err)
	}
	if len(mc.env) != 0 || mc.hash != "" {
		t.Fatalf("no schema should materialize nothing, got env=%v hash=%q", mc.env, mc.hash)
	}
}

func TestMaterializeConfig_Errors(t *testing.T) {
	cases := []struct {
		name    string
		schema  []kestrelv1alpha1.ConfigField
		config  map[string]string
		wantErr string
	}{
		{
			name:    "unknown key",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "TYPE", Type: "string"}},
			config:  map[string]string{"TPYE": "PAPER"},
			wantErr: "unknown config keys TPYE",
		},
		{
			name:    "required missing without default",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "WORLD", Type: "string", Required: true}},
			config:  nil,
			wantErr: `required config field "WORLD"`,
		},
		{
			name:    "required explicitly emptied despite default",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "WORLD", Type: "string", Default: "w", Required: true}},
			config:  map[string]string{"WORLD": ""},
			wantErr: `required config field "WORLD"`,
		},
		{
			name:    "bad int",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "MAX_PLAYERS", Type: "int"}},
			config:  map[string]string{"MAX_PLAYERS": "lots"},
			wantErr: "not an integer",
		},
		{
			name:    "bad bool",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "HARDCORE", Type: "bool"}},
			config:  map[string]string{"HARDCORE": "yep"},
			wantErr: "not a boolean",
		},
		{
			name:    "enum violation",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "MODE", Type: "enum", Enum: []string{"survival", "creative"}}},
			config:  map[string]string{"MODE": "hardcore"},
			wantErr: "not one of",
		},
		{
			name:    "file target not implemented",
			schema:  []kestrelv1alpha1.ConfigField{{Name: "SERVER_CFG", Type: "string", Target: "file"}},
			config:  map[string]string{"SERVER_CFG": "motd=hi"},
			wantErr: "file targets are not implemented",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs, tmpl := configFixtures(tc.schema, tc.config)
			_, err := materializeConfig(gs, tmpl)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestMaterializeConfig_FileTargetUnsetIsAllowed(t *testing.T) {
	// A declared-but-unset file field must not block servers that
	// don't use it; only a value we would have to drop is an error.
	gs, tmpl := configFixtures([]kestrelv1alpha1.ConfigField{
		{Name: "SERVER_CFG", Type: "string", Target: "file"},
	}, nil)
	if _, err := materializeConfig(gs, tmpl); err != nil {
		t.Fatalf("unset optional file field should be fine, got: %v", err)
	}
}
