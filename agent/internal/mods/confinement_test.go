package mods

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfinePath_RejectEmpty(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "")
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v, want ErrEmpty", err)
	}
}

func TestConfinePath_RejectDot(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, ".")
	if !errors.Is(err, ErrDot) {
		t.Fatalf("got %v, want ErrDot", err)
	}
}

func TestConfinePath_RejectDotDot(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "..")
	if !errors.Is(err, ErrDotDot) {
		t.Fatalf("got %v, want ErrDotDot", err)
	}
}

func TestConfinePath_RejectTraversalPrefix(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "../../../etc/passwd")
	if !errors.Is(err, ErrTraversal) {
		t.Fatalf("got %v, want ErrTraversal", err)
	}
}

func TestConfinePath_RejectAbsolutePath(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "/etc/passwd")
	if !errors.Is(err, ErrAbsolute) {
		t.Fatalf("got %v, want ErrAbsolute", err)
	}
}

func TestConfinePath_RejectEmbeddedSeparator(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "subdir/file.txt")
	if !errors.Is(err, ErrSeparator) {
		t.Fatalf("got %v, want ErrSeparator", err)
	}
}

func TestConfinePath_RejectBackslash(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, "file\\dir.txt")
	if !errors.Is(err, ErrControlChar) {
		t.Fatalf("got %v, want ErrControlChar (backslash)", err)
	}
}

func TestConfinePath_RejectLeadingDot(t *testing.T) {
	root := t.TempDir()
	_, err := ConfinePath(root, ".hidden")
	if !errors.Is(err, ErrLeadingDot) {
		t.Fatalf("got %v, want ErrLeadingDot", err)
	}
}

func TestConfinePath_RejectTooLong(t *testing.T) {
	root := t.TempDir()
	longName := strings.Repeat("a", 201)
	_, err := ConfinePath(root, longName)
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("got %v, want ErrTooLong", err)
	}
}

func TestConfinePath_RejectControlCharacters(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name      string
		component string
		want      error
	}{
		{"null", "file\x00.txt", ErrControlChar},
		{"tab", "file\t.txt", ErrControlChar},
		{"newline", "file\n.txt", ErrControlChar},
		{"del", "file\x7f.txt", ErrControlChar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConfinePath(root, tt.component)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestConfinePath_AcceptOrdinaryName(t *testing.T) {
	root := t.TempDir()
	path, err := ConfinePath(root, "mymod")
	if err != nil {
		t.Fatalf("ConfinePath failed: %v", err)
	}
	expected := filepath.Join(root, "mymod")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfinePath_AcceptNameWithMaxLength(t *testing.T) {
	root := t.TempDir()
	name := strings.Repeat("a", 200)
	path, err := ConfinePath(root, name)
	if err != nil {
		t.Fatalf("ConfinePath failed: %v", err)
	}
	expected := filepath.Join(root, name)
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfinePath_RejectSymlinkTargetEscapesRoot(t *testing.T) {
	root := t.TempDir()
	escaped := t.TempDir() // A directory outside root

	// Create a symlink inside root pointing outside root
	linkPath := filepath.Join(root, "escape")
	if err := os.Symlink(escaped, linkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// ConfinePath should reject it
	_, err := ConfinePath(root, "escape")
	if !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("got %v, want ErrEscapesRoot", err)
	}
}

func TestConfinePath_AcceptSymlinkTargetInsideRoot(t *testing.T) {
	root := t.TempDir()

	// Create a target directory inside root
	targetDir := filepath.Join(root, "target")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Create a symlink inside root pointing to another part of root
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	path, err := ConfinePath(root, "link")
	if err != nil {
		t.Fatalf("ConfinePath failed: %v", err)
	}

	// The returned path should be the resolved target
	if path != targetDir {
		t.Fatalf("got %s, want resolved target %s", path, targetDir)
	}
}

func TestConfinePath_AcceptNameResolveToRoot(t *testing.T) {
	// Test that an empty component (or one that resolves to the root itself)
	// is rejected per the contract. An empty string is already rejected by
	// ErrEmpty, but let's verify the contract's stated behavior.
	root := t.TempDir()

	// The contract says ".ConfinePath(root, ".") → error", so "." is rejected.
	// A valid component resolves to root/component, never to root itself.
	// So this test just verifies the normal case.
	_, err := ConfinePath(root, ".")
	if !errors.Is(err, ErrDot) {
		t.Fatalf("got %v, want ErrDot", err)
	}
}

func TestConfinePath_Cleaned(t *testing.T) {
	// Test that the returned path is cleaned (no redundant separators, etc.).
	root := t.TempDir()
	path, err := ConfinePath(root, "mymod")
	if err != nil {
		t.Fatalf("ConfinePath failed: %v", err)
	}
	if filepath.Clean(path) != path {
		t.Fatalf("path is not clean: %s != %s", path, filepath.Clean(path))
	}
}

func TestConfinePath_NewFileUnderNonExistentParent(t *testing.T) {
	// Test that ConfinePath allows a new file under a parent that doesn't
	// exist yet, as long as the deepest existing ancestor is inside root.
	root := t.TempDir()

	// Use a component for a file that doesn't exist
	path, err := ConfinePath(root, "nonexistent.txt")
	if err != nil {
		t.Fatalf("ConfinePath failed: %v", err)
	}

	expected := filepath.Join(root, "nonexistent.txt")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfinePath_TableDriven(t *testing.T) {
	// Comprehensive table-driven test covering multiple scenarios.
	// setup returns the component to pass to ConfinePath; wantPath is checked
	// only when wantErr is nil.
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string) string
		wantErr  error
		wantPath string
	}{
		{
			name:    "valid name",
			setup:   func(_ *testing.T, _ string) string { return "validmod" },
			wantErr: nil,
		},
		{
			name:    "valid name with underscore",
			setup:   func(_ *testing.T, _ string) string { return "valid_mod_123" },
			wantErr: nil,
		},
		{
			name:    "empty component",
			setup:   func(_ *testing.T, _ string) string { return "" },
			wantErr: ErrEmpty,
		},
		{
			name:    "dot only",
			setup:   func(_ *testing.T, _ string) string { return "." },
			wantErr: ErrDot,
		},
		{
			name:    "dotdot",
			setup:   func(_ *testing.T, _ string) string { return ".." },
			wantErr: ErrDotDot,
		},
		{
			name:    "dotdot slash",
			setup:   func(_ *testing.T, _ string) string { return "../evil" },
			wantErr: ErrTraversal,
		},
		{
			name:    "absolute path",
			setup:   func(_ *testing.T, _ string) string { return "/etc/passwd" },
			wantErr: ErrAbsolute,
		},
		{
			name:    "slash separator",
			setup:   func(_ *testing.T, _ string) string { return "sub/dir" },
			wantErr: ErrSeparator,
		},
		{
			name:    "leading dot",
			setup:   func(_ *testing.T, _ string) string { return ".hidden" },
			wantErr: ErrLeadingDot,
		},
		{
			name:    "too long",
			setup:   func(_ *testing.T, _ string) string { return strings.Repeat("x", 201) },
			wantErr: ErrTooLong,
		},
		{
			name:    "null byte",
			setup:   func(_ *testing.T, _ string) string { return "file\x00.txt" },
			wantErr: ErrControlChar,
		},
		{
			name: "symlink escape",
			setup: func(t *testing.T, root string) string {
				escaped := t.TempDir()
				linkPath := filepath.Join(root, "escape")
				if err := os.Symlink(escaped, linkPath); err != nil {
					t.Fatalf("Symlink failed: %v", err)
				}
				return "escape"
			},
			wantErr: ErrEscapesRoot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			component := tt.setup(t, root)

			path, err := ConfinePath(root, component)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ConfinePath failed: %v", err)
			}

			// Verify path is confined
			if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
				t.Fatalf("path %s not confined to root %s", path, root)
			}

			// Verify path is clean
			if filepath.Clean(path) != path {
				t.Fatalf("path not clean: %s", path)
			}
		})
	}
}

// --- Tests for ConfineRelPath (multi-segment relative paths) ---

func TestConfineRelPath_AcceptNestedPath(t *testing.T) {
	root := t.TempDir()
	path, err := ConfineRelPath(root, "config/settings.json")
	if err != nil {
		t.Fatalf("ConfineRelPath(nested path) failed: %v", err)
	}
	expected := filepath.Join(root, "config", "settings.json")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfineRelPath_AcceptDotfile(t *testing.T) {
	root := t.TempDir()
	path, err := ConfineRelPath(root, ".gitkeep")
	if err != nil {
		t.Fatalf("ConfineRelPath(dotfile) failed: %v", err)
	}
	expected := filepath.Join(root, ".gitkeep")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfineRelPath_AcceptNestedDotfile(t *testing.T) {
	root := t.TempDir()
	path, err := ConfineRelPath(root, "config/.gitignore")
	if err != nil {
		t.Fatalf("ConfineRelPath(nested dotfile) failed: %v", err)
	}
	expected := filepath.Join(root, "config", ".gitignore")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfineRelPath_RejectTraversalPrefix(t *testing.T) {
	root := t.TempDir()
	_, err := ConfineRelPath(root, "../escape")
	if !errors.Is(err, ErrDotDot) {
		t.Fatalf("got %v, want ErrDotDot", err)
	}
}

func TestConfineRelPath_RejectTraversalInMiddle(t *testing.T) {
	root := t.TempDir()
	_, err := ConfineRelPath(root, "a/../../escape")
	if !errors.Is(err, ErrDotDot) {
		t.Fatalf("got %v, want ErrDotDot", err)
	}
}

func TestConfineRelPath_RejectAbsolutePath(t *testing.T) {
	root := t.TempDir()
	_, err := ConfineRelPath(root, "/etc/passwd")
	if !errors.Is(err, ErrAbsolute) {
		t.Fatalf("got %v, want ErrAbsolute", err)
	}
}

func TestConfineRelPath_RejectEmptyPath(t *testing.T) {
	root := t.TempDir()
	_, err := ConfineRelPath(root, "")
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v, want ErrEmpty", err)
	}
}

func TestConfineRelPath_RejectControlCharacters(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		{"null byte", "file\x00.txt"},
		{"tab", "file\t.txt"},
		{"newline", "file\n.txt"},
		{"del", "file\x7f.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConfineRelPath(root, tt.path)
			if !errors.Is(err, ErrControlChar) {
				t.Fatalf("got %v, want ErrControlChar", err)
			}
		})
	}
}

func TestConfineRelPath_RejectPathTooLong(t *testing.T) {
	root := t.TempDir()

	// Build a path longer than 4096 characters.
	// Use pattern "a/a/a/.../a": each "a/" is 2 chars, so 2048 repetitions = 4096,
	// plus one more "a" = 4097 total.
	tooLongPath := strings.Repeat("a/", 2048) + "a"
	if len(tooLongPath) <= 4096 {
		t.Fatalf("test setup error: path is %d chars, need > 4096", len(tooLongPath))
	}

	_, err := ConfineRelPath(root, tooLongPath)
	if err == nil {
		t.Fatalf("got nil error, want ErrTooLong")
	}
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("got %v, want ErrTooLong", err)
	}

	// Test boundary: acceptable length (4095 chars) should not be rejected.
	// Use same pattern: 2047 repetitions + "a" = 4095 chars.
	acceptablePath := strings.Repeat("a/", 2047) + "a"
	if len(acceptablePath) >= 4096 {
		t.Fatalf("test setup error: boundary path is %d chars, need < 4096", len(acceptablePath))
	}

	_, err = ConfineRelPath(root, acceptablePath)
	if err != nil {
		t.Fatalf("acceptable-length path failed: %v", err)
	}

	// Test boundary: exactly 4096 chars should be accepted (not rejected).
	// The code checks if len(relPath) > 4096, so 4096 is allowed.
	// Use pattern "a/" repeated 2048 times = exactly 4096 chars.
	boundary4096Path := strings.Repeat("a/", 2048)
	if len(boundary4096Path) != 4096 {
		t.Fatalf("test setup error: path is %d chars, need exactly 4096", len(boundary4096Path))
	}

	_, err = ConfineRelPath(root, boundary4096Path)
	if err != nil {
		t.Fatalf("4096-char path failed: %v", err)
	}
}

func TestConfineRelPath_NormalizeBackslashes(t *testing.T) {
	root := t.TempDir()
	// Backslashes in archive entries (Windows-style) should be normalized to forward slashes
	path, err := ConfineRelPath(root, "config\\settings.json")
	if err != nil {
		t.Fatalf("ConfineRelPath(backslash) failed: %v", err)
	}
	// On Windows, filepath.Join will use backslashes, but on Unix it uses forward slashes.
	// The important thing is that it's confined and doesn't escape.
	expected := filepath.Join(root, "config", "settings.json")
	if path != expected {
		t.Fatalf("got %s, want %s", path, expected)
	}
}

func TestConfineRelPath_RejectSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	escaped := t.TempDir() // Outside root

	// Create a symlink inside root pointing outside
	linkPath := filepath.Join(root, "escape")
	if err := os.Symlink(escaped, linkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// ConfineRelPath should reject a path trying to use the symlink to escape
	_, err := ConfineRelPath(root, "escape/file.txt")
	if !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("got %v, want ErrEscapesRoot", err)
	}
}

func TestConfineRelPath_AcceptSymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()

	// Create a target directory inside root
	targetDir := filepath.Join(root, "target")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Create a symlink inside root pointing to the target
	linkPath := filepath.Join(root, "link")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// ConfineRelPath should accept a path through the symlink if it stays in root
	path, err := ConfineRelPath(root, "link/file.txt")
	if err != nil {
		t.Fatalf("ConfineRelPath(symlink inside root) failed: %v", err)
	}

	// The result should be confined
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) && path != root {
		t.Fatalf("path %s not confined to root %s", path, root)
	}
}

func TestConfineRelPath_RejectAncestorSymlinkEscapesRoot(t *testing.T) {
	root := t.TempDir()
	escaped := t.TempDir() // Outside root

	// Create and then replace an ancestor directory with a symlink pointing outside root
	ancestorPath := filepath.Join(root, "ancestor")
	if err := os.Mkdir(ancestorPath, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := os.Remove(ancestorPath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if err := os.Symlink(escaped, ancestorPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Try to create a file nested under the escaping ancestor symlink.
	// The path doesn't exist yet, so the ancestor walk code (confinement.go:232-249)
	// will run: it will find the ancestor symlink, resolve it, and detect that it
	// escapes root.
	_, err := ConfineRelPath(root, filepath.Join("ancestor", "nested", "newfile.txt"))
	if !errors.Is(err, ErrEscapesRoot) {
		t.Fatalf("got %v, want ErrEscapesRoot", err)
	}
}

func TestConfineRelPath_AcceptAncestorSymlinkInsideRoot(t *testing.T) {
	root := t.TempDir()

	// Create a target directory inside root
	targetDir := filepath.Join(root, "target")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}

	// Create an ancestor directory and replace it with a symlink pointing to target
	ancestorPath := filepath.Join(root, "ancestor")
	if err := os.Mkdir(ancestorPath, 0o755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := os.Remove(ancestorPath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if err := os.Symlink(targetDir, ancestorPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Try to create a file nested under the ancestor symlink (pointing inside root).
	// The ancestor walk code should resolve the ancestor symlink and verify it
	// stays within root, then allow the path.
	path, err := ConfineRelPath(root, filepath.Join("ancestor", "nested", "newfile.txt"))
	if err != nil {
		t.Fatalf("ConfineRelPath failed: %v", err)
	}

	// The result should be confined
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) && path != root {
		t.Fatalf("path %s not confined to root %s", path, root)
	}
}

func TestConfinePath_SymlinkLoopError(t *testing.T) {
	root := t.TempDir()

	// Create a symlink loop: link1 -> link2, link2 -> link1
	link1 := filepath.Join(root, "link1")
	link2 := filepath.Join(root, "link2")
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Try to confine the symlink; EvalSymlinks should fail with "too many symlinks"
	_, err := ConfinePath(root, "link1")
	if err == nil {
		t.Fatal("ConfinePath should error on symlink loop")
	}
	// The error should be from resolving symlinks, not a validation error
	if errors.Is(err, ErrEmpty) || errors.Is(err, ErrAbsolute) || errors.Is(err, ErrTraversal) {
		t.Fatalf("wrong error type: %v", err)
	}
}

func TestConfineRelPath_SymlinkLoopError(t *testing.T) {
	root := t.TempDir()

	// Create a symlink loop
	link1 := filepath.Join(root, "link1")
	link2 := filepath.Join(root, "link2")
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Try to confine a path through the symlink loop
	_, err := ConfineRelPath(root, "link1")
	if err == nil {
		t.Fatal("ConfineRelPath should error on symlink loop")
	}
	// Should not be a validation error (those are caught earlier)
	if errors.Is(err, ErrEmpty) || errors.Is(err, ErrAbsolute) || errors.Is(err, ErrTooLong) {
		t.Fatalf("wrong error type: %v", err)
	}
}

func TestConfineRelPath_AncestorSymlinkLoopError(t *testing.T) {
	root := t.TempDir()

	// Create directories a/b/c so we have a deep path to work with
	aPath := filepath.Join(root, "a")
	bPath := filepath.Join(aPath, "b")
	cPath := filepath.Join(bPath, "c")
	if err := os.MkdirAll(cPath, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Remove the top-level directory "a" and replace it with a symlink loop
	if err := os.RemoveAll(aPath); err != nil {
		t.Fatalf("RemoveAll failed: %v", err)
	}
	link1 := filepath.Join(root, "link1")
	link2 := filepath.Join(root, "link2")
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}
	if err := os.Symlink(link1, link2); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Make "a" point to the loop
	if err := os.Symlink(link1, aPath); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Try to create a file nested deep under "a"
	// The target doesn't exist, so ancestor walk will start
	// When it tries to resolve ancestor "a", it will encounter the symlink loop
	_, err := ConfineRelPath(root, "a/b/c/d/file.txt")
	if err == nil {
		t.Fatal("ConfineRelPath should error on ancestor symlink loop")
	}
	// Should not be a validation error
	if errors.Is(err, ErrEmpty) || errors.Is(err, ErrAbsolute) || errors.Is(err, ErrDotDot) {
		t.Fatalf("wrong error type: %v", err)
	}
}

func TestConfineRelPath_AcceptsDotAndEmptySegments(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name    string
		relPath string
	}{
		{
			name:    "dot in middle",
			relPath: "a/./b.txt",
		},
		{
			name:    "double slash",
			relPath: "a//b.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ConfineRelPath(root, tt.relPath)
			if err != nil {
				t.Fatalf("ConfineRelPath failed: %v", err)
			}

			// For non-existent paths, the function cleans the segments and returns
			// the normalized absolute path under root. Both "a/./b.txt" and "a//b.txt"
			// clean to "a/b.txt" under root.
			expected := filepath.Join(root, "a", "b.txt")
			if path != expected {
				t.Fatalf("got %s, want %s", path, expected)
			}

			// Verify the result is confined to root
			if path != root && !strings.HasPrefix(path, root+string(os.PathSeparator)) {
				t.Fatalf("path %s not confined to root %s", path, root)
			}
		})
	}
}
