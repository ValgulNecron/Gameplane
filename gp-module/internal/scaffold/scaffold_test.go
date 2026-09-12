package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

		res, err := Scaffold(ScaffoldOptions{
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
	opts := ScaffoldOptions{
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
	_, err := Scaffold(ScaffoldOptions{
		Name: "Invalid_Name!",
	})
	if err == nil {
		t.Errorf("expected error for invalid DNS-1123 name")
	}
}
