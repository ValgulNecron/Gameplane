package preview

import (
	"strings"
	"testing"
)

func TestCalculateAutoMemory(t *testing.T) {
	tests := []struct {
		memory  string
		percent int
		want    string
		wantErr bool
	}{
		{"4Gi", 75, "3072M", false},
		{"8Gi", 75, "6144M", false},
		{"2048Mi", 50, "1024M", false},
		{"1Gi", 100, "1024M", false},
		{"1024Mi", 25, "256M", false},
		{"invalid", 50, "", true},
		{"4Gi", 0, "", true},
		{"4Gi", 101, "", true},
	}

	for _, tc := range tests {
		got, err := CalculateAutoMemory(tc.memory, tc.percent)
		if (err != nil) != tc.wantErr {
			t.Errorf("CalculateAutoMemory(%q, %d) error = %v, wantErr %v", tc.memory, tc.percent, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("CalculateAutoMemory(%q, %d) = %q, want %q", tc.memory, tc.percent, got, tc.want)
		}
	}
}

func TestPreviewManifest(t *testing.T) {
	tmplYAML := `apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: mc-java
spec:
  displayName: Minecraft Java
  game: minecraft-java
  image: "ghcr.io/example/mc:default@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  ports:
    - name: game
      containerPort: 25565
      protocol: TCP
  storage:
    size: 10Gi
    mountPath: /data
  env:
    - name: EULA
      value: "TRUE"
    - name: MOTD
      value: "Welcome"
  configSchema:
    - name: MOTD
      type: string
      default: "Default MOTD"
    - name: MEMORY
      type: string
      autoFromMemoryLimit:
        percent: 75
    - name: RCON_PASSWORD
      type: password
  versions:
    - id: "1.20.4"
      name: "1.20.4 Paper"
      image: "ghcr.io/example/mc:1.20.4@sha256:1111111111111111111111111111111111111111111111111111111111111111"
      env:
        - name: VERSION
          value: "1.20.4"
`

	opts := Options{
		TemplateYAML: []byte(tmplYAML),
		VersionID:    "1.20.4",
		MemoryLimit:  "4Gi",
		UserConfig: map[string]string{
			"MOTD":          "Custom Server MOTD",
			"RCON_PASSWORD": "supersecretpassword",
		},
	}

	res, err := GeneratePreview(opts)
	if err != nil {
		t.Fatalf("GeneratePreview failed: %v", err)
	}

	if res.Game != "minecraft-java" {
		t.Errorf("expected game minecraft-java, got %s", res.Game)
	}
	if !strings.Contains(res.EffectiveImage, "1.20.4") {
		t.Errorf("expected version overlay image, got %s", res.EffectiveImage)
	}
	if res.ComputedConfig["MEMORY"] != "3072M" {
		t.Errorf("expected MEMORY = 3072M, got %s", res.ComputedConfig["MEMORY"])
	}
	if res.ComputedConfig["MOTD"] != "Custom Server MOTD" {
		t.Errorf("expected MOTD override, got %s", res.ComputedConfig["MOTD"])
	}
	if res.ComputedConfig["RCON_PASSWORD"] != "******" {
		t.Errorf("expected password redaction, got %s", res.ComputedConfig["RCON_PASSWORD"])
	}
	if res.EffectiveEnv["VERSION"] != "1.20.4" {
		t.Errorf("expected VERSION env var from version overlay, got %s", res.EffectiveEnv["VERSION"])
	}
	if res.EffectiveEnv["EULA"] != "TRUE" {
		t.Errorf("expected base EULA env var, got %s", res.EffectiveEnv["EULA"])
	}
	if res.EffectiveEnv["MEMORY"] != "3072M" {
		t.Errorf("expected MEMORY env var in effectiveEnv, got %s", res.EffectiveEnv["MEMORY"])
	}

	humanOut := res.FormatHuman()
	if !strings.Contains(humanOut, "3072M") || !strings.Contains(humanOut, "Custom Server MOTD") {
		t.Errorf("human output missing expected info:\n%s", humanOut)
	}
	if strings.Contains(humanOut, "supersecretpassword") {
		t.Errorf("password leaked in human output:\n%s", humanOut)
	}
}

func TestPreview_VersionNotFound(t *testing.T) {
	tmplYAML := `apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: test
spec:
  game: test
  image: "img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  versions:
    - id: "1.0"
`
	_, err := GeneratePreview(Options{
		TemplateYAML: []byte(tmplYAML),
		VersionID:    "nonexistent",
	})
	if err == nil {
		t.Errorf("expected error for nonexistent version ID")
	}
}

func TestPreview_TypedDefaultsAndInvalidAutoMemory(t *testing.T) {
	tmplYAML := `apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: test
spec:
  game: test
  image: "img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  configSchema:
    - name: ENABLED
      type: boolean
      default: true
    - name: MAX_PLAYERS
      type: int
      default: 32
`
	res, err := GeneratePreview(Options{
		TemplateYAML: []byte(tmplYAML),
	})
	if err != nil {
		t.Fatalf("GeneratePreview failed: %v", err)
	}
	if res.ComputedConfig["ENABLED"] != "true" {
		t.Errorf("expected boolean default string 'true', got %s", res.ComputedConfig["ENABLED"])
	}
	if res.ComputedConfig["MAX_PLAYERS"] != "32" {
		t.Errorf("expected int default string '32', got %s", res.ComputedConfig["MAX_PLAYERS"])
	}

	// Invalid auto memory
	badAutoMemYAML := `apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: test
spec:
  game: test
  image: "img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  configSchema:
    - name: MEM
      type: string
      autoFromMemoryLimit:
        percent: 150
`
	_, err = GeneratePreview(Options{
		TemplateYAML: []byte(badAutoMemYAML),
	})
	if err == nil {
		t.Errorf("expected error for percent > 100")
	}
}

func TestPreview_PortAdvertise(t *testing.T) {
	tmplYAML := `apiVersion: gameplane.local/v1alpha1
kind: GameTemplate
metadata:
  name: test
spec:
  game: test
  image: "img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  ports:
    - name: game
      containerPort: 27015
      protocol: UDP
      advertise: true
    - name: rcon
      containerPort: 27020
      protocol: TCP
      advertise: false
`
	res, err := GeneratePreview(Options{
		TemplateYAML: []byte(tmplYAML),
	})
	if err != nil {
		t.Fatalf("GeneratePreview failed: %v", err)
	}
	if len(res.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(res.Ports))
	}
	if !res.Ports[0].Advertise {
		t.Errorf("expected port 0 Advertise=true")
	}
	if res.Ports[1].Advertise {
		t.Errorf("expected port 1 Advertise=false")
	}
}
