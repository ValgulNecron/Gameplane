package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ValgulNecron/gameplane/gp-module/internal/archetypes"
)

func TestScaffoldArchetypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gp-module-scaffold-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, arch := range []string{"steamcmd", "java", "generic"} {
		moduleName := "test-" + arch
		outDir := filepath.Join(tmpDir, moduleName)

		res, err := Scaffold(Options{
			Name:      moduleName,
			Archetype: arch,
			OutputDir: outDir,
		})
		if err != nil {
			t.Fatalf("Scaffold(%q) failed: %v", arch, err)
		}

		if res.Dir != outDir {
			t.Errorf("expected dir %q, got %q", outDir, res.Dir)
		}

		// Verify files exist on disk
		for _, file := range []string{"module.yaml", "template.yaml", "README.md", "icon.png"} {
			p := filepath.Join(outDir, file)
			if fi, err := os.Stat(p); err != nil || fi.Size() == 0 {
				t.Errorf("expected non-empty file at %s, err = %v", p, err)
			}
		}

		// Check modelines
		moduleContent, _ := os.ReadFile(filepath.Join(outDir, "module.yaml"))
		if !strings.HasPrefix(string(moduleContent), "# yaml-language-server: $schema=../.schema/module.schema.json") {
			t.Errorf("module.yaml missing expected modeline")
		}

		templateContent, _ := os.ReadFile(filepath.Join(outDir, "template.yaml"))
		if !strings.HasPrefix(string(templateContent), "# yaml-language-server: $schema=../.schema/gametemplate.schema.json") {
			t.Errorf("template.yaml missing expected modeline")
		}
	}
}

func TestScaffoldOverwriteProtection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gp-module-overwrite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outDir := filepath.Join(tmpDir, "my-game")
	opts := Options{
		Name:      "my-game",
		OutputDir: outDir,
	}

	// First run: success
	if _, err := Scaffold(opts); err != nil {
		t.Fatalf("initial scaffold failed: %v", err)
	}

	// Second run without overwrite: must fail
	if _, err := Scaffold(opts); err == nil {
		t.Errorf("expected error when scaffolding over existing directory without overwrite")
	}

	// Third run with overwrite: must succeed
	opts.Overwrite = true
	if _, err := Scaffold(opts); err != nil {
		t.Errorf("expected success with Overwrite=true, got: %v", err)
	}
}

func TestScaffoldInvalidName(t *testing.T) {
	_, err := Scaffold(Options{
		Name: "Invalid_Name!",
	})
	if err == nil {
		t.Errorf("expected error for invalid DNS-1123 name")
	}
}

func TestScaffoldCustomPorts_Validation(t *testing.T) {
	tests := []struct {
		name    string
		ports   []archetypes.PortDef
		wantErr bool
	}{
		{
			name: "valid ports",
			ports: []archetypes.PortDef{
				{Name: "game", ContainerPort: 27015, Protocol: "UDP"},
				{Name: "rcon", ContainerPort: 27016, Protocol: "TCP"},
			},
			wantErr: false,
		},
		{
			name: "port number too small",
			ports: []archetypes.PortDef{
				{Name: "game", ContainerPort: 0, Protocol: "UDP"},
			},
			wantErr: true,
		},
		{
			name: "port number too large",
			ports: []archetypes.PortDef{
				{Name: "game", ContainerPort: 70000, Protocol: "UDP"},
			},
			wantErr: true,
		},
		{
			name: "invalid protocol",
			ports: []archetypes.PortDef{
				{Name: "game", ContainerPort: 27015, Protocol: "HTTP"},
			},
			wantErr: true,
		},
		{
			name: "duplicate port collision",
			ports: []archetypes.PortDef{
				{Name: "game", ContainerPort: 27015, Protocol: "UDP"},
				{Name: "query", ContainerPort: 27015, Protocol: "UDP"},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateFiles(Options{
				Name:  "my-game",
				Ports: tc.ports,
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("GenerateFiles() err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestGenerateFiles_IconBase64(t *testing.T) {
	files, err := GenerateFiles(Options{
		Name: "icon-test",
	})
	if err != nil {
		t.Fatalf("GenerateFiles failed: %v", err)
	}
	if files.IconBase64 == "" {
		t.Errorf("expected non-empty IconBase64")
	}
	if len(files.IconBytes) == 0 {
		t.Errorf("expected non-empty IconBytes")
	}
}
