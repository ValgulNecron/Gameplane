package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ValgulNecron/gameplane/gp-module/internal/scaffold"
)

func TestValidate_ValidScaffoldedModule(t *testing.T) {
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "my-game")

	opts := scaffold.Options{
		Name:        "my-game",
		DisplayName: "My Game",
		Archetype:   "steamcmd",
		OutputDir:   modDir,
		Overwrite:   false,
	}

	if _, err := scaffold.Scaffold(opts); err != nil {
		t.Fatalf("failed to scaffold: %v", err)
	}

	report, err := ValidateDirectory(modDir, ValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	if !report.Clean {
		t.Errorf("expected module to be clean, got findings: %+v", report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(report.Findings))
	}
}

func TestValidate_MissingRequiredFiles(t *testing.T) {
	tempDir := t.TempDir()
	modDir := filepath.Join(tempDir, "empty-module")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}

	report, err := ValidateDirectory(modDir, ValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Clean {
		t.Errorf("expected module to fail validation")
	}

	missing := make(map[string]bool)
	for _, f := range report.Findings {
		if f.RuleID == RuleMissingRequiredFile {
			missing[f.File] = true
		}
	}

	if !missing["module.yaml"] || !missing["template.yaml"] || !missing["README.md"] {
		t.Errorf("expected missing module.yaml, template.yaml, and README.md, got %+v", missing)
	}
}

func TestValidate_InvalidModuleName(t *testing.T) {
	files := map[string][]byte{
		"module.yaml": []byte(`apiVersion: gameplane.local/module/v1
name: INVALID_NAME
displayName: Bad Name
version: 1.0.0
game: bad
summary: Bad test module
`),
		"template.yaml": []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: bad
spec:
  image: "ghcr.io/example/game@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
`),
		"README.md": []byte("# Test\n"),
	}

	report, err := ValidateFiles("invalid-dir", files, ValidateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, f := range report.Findings {
		if f.RuleID == RuleInvalidModuleName {
			found = true
			if f.Line != 2 {
				t.Errorf("expected line 2 for invalid module name, got %d", f.Line)
			}
		}
	}
	if !found {
		t.Errorf("expected invalid-module-name error")
	}
}

func TestValidate_ImageUnpinned(t *testing.T) {
	// 1. Unpinned without comment
	files := map[string][]byte{
		"module.yaml": []byte(`apiVersion: gameplane.local/module/v1
name: test-mod
displayName: Test Mod
version: 1.0.0
game: test
summary: Test
`),
		"template.yaml": []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: test-mod
spec:
  image: "docker.io/mygame/server:latest"
`),
		"README.md": []byte("# Test\n"),
	}

	report, err := ValidateFiles("test-mod", files, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range report.Findings {
		if f.RuleID == RuleImageUnpinned {
			found = true
			if f.Line != 6 {
				t.Errorf("expected line 6 for unpinned image, got %d", f.Line)
			}
		}
	}
	if !found {
		t.Errorf("expected image-unpinned error")
	}

	// 2. Unpinned with # gameplane:floating comment
	files["template.yaml"] = []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: test-mod
spec:
  image: "docker.io/mygame/server:latest" # gameplane:floating
`)

	report, err = ValidateFiles("test-mod", files, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Findings {
		if f.RuleID == RuleImageUnpinned {
			t.Errorf("expected image-unpinned to be suppressed by gameplane:floating comment")
		}
	}
}

func TestValidate_UnrecognizedCategory(t *testing.T) {
	files := map[string][]byte{
		"module.yaml": []byte(`apiVersion: gameplane.local/module/v1
name: test-mod
displayName: Test Mod
version: 1.0.0
game: test
summary: Test
categories:
  - Survival
  - SpaceOpera
`),
		"template.yaml": []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: test-mod
spec:
  image: "example.com/img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
`),
		"README.md": []byte("# Test\n"),
	}

	report, err := ValidateFiles("test-mod", files, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, f := range report.Findings {
		if f.RuleID == RuleUnrecognizedCategory {
			found = true
			if f.Level != SeverityWarn {
				t.Errorf("expected WARN level, got %s", f.Level)
			}
		}
	}
	if !found {
		t.Errorf("expected unrecognized-category warning")
	}
}

func TestValidate_PortChecks(t *testing.T) {
	files := map[string][]byte{
		"module.yaml": []byte(`apiVersion: gameplane.local/module/v1
name: test-mod
displayName: Test Mod
version: 1.0.0
game: test
summary: Test
`),
		"template.yaml": []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: test-mod
spec:
  image: "example.com/img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  ports:
    - name: game
      containerPort: 70000
      protocol: TCP
    - name: query
      containerPort: 27015
      protocol: ICMP
    - name: dup1
      containerPort: 8080
      protocol: TCP
    - name: dup2
      containerPort: 8080
      protocol: TCP
`),
		"README.md": []byte("# Test\n"),
	}

	report, err := ValidateFiles("test-mod", files, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rulesFound := make(map[string]bool)
	for _, f := range report.Findings {
		rulesFound[f.RuleID] = true
	}

	if !rulesFound[RuleInvalidPortNumber] {
		t.Errorf("expected invalid-port-number error")
	}
	if !rulesFound[RuleInvalidPortProtocol] {
		t.Errorf("expected invalid-port-protocol error")
	}
	if !rulesFound[RuleDuplicatePortCollision] {
		t.Errorf("expected duplicate-port-collision error")
	}
}

func TestValidate_ConfigSchemaAndMemory(t *testing.T) {
	files := map[string][]byte{
		"module.yaml": []byte(`apiVersion: gameplane.local/module/v1
name: test-mod
displayName: Test Mod
version: 1.0.0
game: test
summary: Test
`),
		"template.yaml": []byte(`apiVersion: gameplane.io/v1alpha1
kind: GameTemplate
metadata:
  name: test-mod
spec:
  image: "example.com/img@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
  configSchema:
    - name: SERVER_PASSWORD
      type: string
    - name: INVALID_TYPE_FIELD
      type: float
    - name: MEM_FIELD
      type: string
      autoFromMemoryLimit:
        percent: 150
`),
		"README.md": []byte("# Test\n"),
	}

	report, err := ValidateFiles("test-mod", files, ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}

	rulesFound := make(map[string]bool)
	for _, f := range report.Findings {
		rulesFound[f.RuleID] = true
	}

	if !rulesFound[RuleInsecureAuthField] {
		t.Errorf("missing expected rule %s", RuleInsecureAuthField)
	}
	if !rulesFound[RuleInvalidConfigType] {
		t.Errorf("expected invalid-config-type error")
	}
	if !rulesFound[RuleInvalidMemoryPercent] {
		t.Errorf("expected invalid-memory-percent error")
	}
}

func TestValidate_ReportJSON(t *testing.T) {
	report := ValidationReport{
		Timestamp:    "2026-08-27T12:00:00Z",
		Clean:        false,
		ErrorCount:   1,
		WarningCount: 1,
		Modules: []ModuleReport{
			{
				Name:  "test-mod",
				Path:  "/path/to/test-mod",
				Clean: false,
				Findings: []Finding{
					{
						Level:       SeverityError,
						RuleID:      RuleInvalidPortNumber,
						File:        "template.yaml",
						Line:        10,
						Field:       "spec.ports[0].containerPort",
						Message:     "port out of range",
						Remediation: "fix port",
					},
					{
						Level:       SeverityWarn,
						RuleID:      RuleUnrecognizedCategory,
						File:        "module.yaml",
						Line:        7,
						Field:       "categories",
						Message:     "unrecognized category",
						Remediation: "use canonical category",
					},
				},
			},
		},
	}

	out, err := report.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("invalid json produced: %v", err)
	}

	if parsed["clean"] != false || parsed["errorCount"].(float64) != 1 {
		t.Errorf("unexpected parsed json values: %+v", parsed)
	}

	humanOut := report.FormatHuman()
	if !strings.Contains(humanOut, "test-mod") || !strings.Contains(humanOut, "ERROR") {
		t.Errorf("human output missing expected text:\n%s", humanOut)
	}
}

func TestValidate_RealModuleCS2(t *testing.T) {
	cs2Dir := filepath.Join("..", "..", "..", "modules", "cs2")
	if _, err := os.Stat(cs2Dir); os.IsNotExist(err) {
		t.Skip("modules/cs2 does not exist, skipping")
	}

	report, err := ValidateDirectory(cs2Dir, ValidateOptions{})
	if err != nil {
		t.Fatalf("failed to validate cs2: %v", err)
	}

	if !report.Clean {
		t.Errorf("expected cs2 to be clean, got findings: %+v", report.Findings)
	}
}

func TestValidate_DirectoryErrors(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "does-not-exist")
	if _, err := ValidateDirectory(nonExistent, ValidateOptions{}); err == nil {
		t.Errorf("expected error for non-existent directory")
	}

	filePath := filepath.Join(tempDir, "regular-file")
	_ = os.WriteFile(filePath, []byte("hello"), 0o644)
	if _, err := ValidateDirectory(filePath, ValidateOptions{}); err == nil {
		t.Errorf("expected error when path is not a directory")
	}
}

func TestValidationReport(t *testing.T) {
	repEmpty := NewValidationReport(nil)
	if !repEmpty.Clean || repEmpty.ErrorCount != 0 || repEmpty.WarningCount != 0 {
		t.Errorf("expected empty report to be clean with 0 counts")
	}

	modReports := []ModuleReport{
		{
			Name:  "mod-ok",
			Clean: true,
		},
		{
			Name:  "mod-err",
			Clean: false,
			Findings: []Finding{
				{
					RuleID:      "TEST-001",
					Level:       SeverityError,
					File:        "module.yaml",
					Line:        10,
					Message:     "test error",
					Remediation: "fix it",
				},
				{
					RuleID:  "TEST-002",
					Level:   SeverityWarn,
					File:    "template.yaml",
					Message: "test warning",
				},
			},
		},
	}

	rep := NewValidationReport(modReports)
	if rep.Clean {
		t.Errorf("expected report to not be clean")
	}
	if rep.ErrorCount != 1 || rep.WarningCount != 1 {
		t.Errorf("expected 1 error and 1 warning, got %d, %d", rep.ErrorCount, rep.WarningCount)
	}

	human := rep.FormatHuman()
	if !strings.Contains(human, "OK (no findings)") || !strings.Contains(human, "TEST-001") || !strings.Contains(human, "Remediation: fix it") {
		t.Errorf("FormatHuman missing expected text:\n%s", human)
	}

	jsonBytes, err := rep.ToJSON()
	if err != nil || len(jsonBytes) == 0 {
		t.Errorf("ToJSON failed: %v", err)
	}
}
