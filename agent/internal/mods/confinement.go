package mods

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfinePath validates an untrusted path component against a root directory
// and returns a safe absolute path confined within the root, or an error if
// the component violates confinement rules.
//
// The validation and symlink resolution must happen in the same function so
// static taint analysis (like CodeQL) recognizes the component as validated
// before use. A separate sanitizer function is invisible to taint analysis,
// which is what produced the original alerts.
//
// root: absolute path to a trusted root directory (e.g., /var/mods)
// component: untrusted path component from user input, archive member, or external source
//
// Returns: a cleaned, absolute path confined within root, or an error if validation fails.
func ConfinePath(root, component string) (string, error) {
	// Validate component as a single path element (no traversal, no separators).
	if err := validateComponent(component); err != nil {
		return "", err
	}

	// Join the component under root.
	abs := filepath.Join(root, component)
	abs = filepath.Clean(abs)

	// Check that the joined path is still confined to root.
	if !isConfined(abs, root) {
		return "", fmt.Errorf("path %s escapes root %s: %w", abs, root, ErrEscapesRoot)
	}

	// Resolve symlinks to block the "/data/escape -> /etc/passwd" attack.
	// If the path exists, EvalSymlinks resolves it and all ancestors.
	// If not, check the deepest existing ancestor to ensure symlinks in
	// the path don't escape the root (e.g., /var/mods/foo is a symlink to /tmp).
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if !isConfined(resolved, root) {
			return "", fmt.Errorf("resolved path %s escapes root %s: %w", resolved, root, ErrEscapesRoot)
		}
		return resolved, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	// The target doesn't exist yet (new file/dir about to be written).
	// Walk up to the deepest ancestor that exists and verify it resolves
	// inside root. This allows creation in new subdirectories while still
	// catching symlink escapes.
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
			if !isConfined(resolvedParent, root) {
				return "", fmt.Errorf("ancestor %s (resolved to %s) escapes root %s: %w", parent, resolvedParent, root, ErrEscapesRoot)
			}
			return abs, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve ancestor %s: %w", parent, err)
		}
		// Parent doesn't exist; loop up to its parent.
	}
}

// validateComponent checks that the component satisfies all confinement rules.
// It rejects path separators, traversal, absolute paths, leading dots, etc.
func validateComponent(component string) error {
	component = strings.TrimSpace(component)

	if component == "" {
		return fmt.Errorf("component is empty: %w", ErrEmpty)
	}

	if component == "." {
		return fmt.Errorf("component is '.': %w", ErrDot)
	}

	if component == ".." {
		return fmt.Errorf("component is '..': %w", ErrDotDot)
	}

	if strings.HasPrefix(component, "../") {
		return fmt.Errorf("component has '../' prefix: %w", ErrTraversal)
	}

	if strings.HasPrefix(component, "/") {
		return fmt.Errorf("component is absolute: %w", ErrAbsolute)
	}

	if strings.Contains(component, string(os.PathSeparator)) {
		return fmt.Errorf("component contains path separator: %w", ErrSeparator)
	}

	if strings.HasPrefix(component, ".") {
		return fmt.Errorf("component has leading dot: %w", ErrLeadingDot)
	}

	if len(component) > 200 {
		return fmt.Errorf("component is too long: %w", ErrTooLong)
	}

	// Reject control characters (U+0000-U+001F, U+007F) and backslash.
	for _, r := range component {
		if r < 0x20 || r == 0x7f || r == '\\' {
			return fmt.Errorf("component contains illegal character: %w", ErrControlChar)
		}
	}

	return nil
}

// isConfined checks that path is either the root itself or is strictly
// under root with a path separator. This ensures the path cannot escape
// the sandbox.
func isConfined(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(os.PathSeparator))
}

// ConfineRelPath validates an untrusted relative path (potentially multi-segment)
// against a root directory and returns a safe absolute path confined within root,
// or an error if the path violates confinement rules.
//
// Unlike ConfinePath, which validates a single path component, ConfineRelPath
// accepts and validates a relative subtree path (e.g., "config/settings.json"
// or a dotfile like ".gitkeep"). This is used by unzipInto to safely extract
// archive entries that legitimately contain subdirectories and leading dots.
//
// Validation rules:
// - Rejects absolute paths, empty paths, and paths that resolve to ".." or "../..."
// - Rejects control characters (U+0000-U+001F, U+007F)
// - Allows interior path separators (nested directories)
// - Allows leading dots on individual segments (dotfiles are legitimate in archives)
// - Rejects any path segment equal to ".." (traversal)
// - Normalizes backslashes to forward slashes (Windows zip entries)
//
// Symlink handling mirrors ConfinePath: the target is resolved via EvalSymlinks
// if it exists, or the deepest existing ancestor is checked if the target is new.
// Both the target and its ancestors must stay within the root directory.
//
// root: absolute path to a trusted root directory (e.g., /extract/staging/)
// relPath: untrusted relative path from user input or archive member
//
// Returns: a cleaned, absolute path confined within root, or an error if validation fails.
func ConfineRelPath(root, relPath string) (string, error) {
	relPath = strings.TrimSpace(relPath)

	// Reject empty path
	if relPath == "" {
		return "", fmt.Errorf("relative path is empty: %w", ErrEmpty)
	}

	// Reject absolute paths
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("path is absolute: %w", ErrAbsolute)
	}

	// Normalize backslashes to forward slashes (Windows zip entries may use backslashes)
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	// Reject control characters
	for _, r := range relPath {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("path contains control character: %w", ErrControlChar)
		}
	}

	// Split the path into segments and validate each
	segments := strings.Split(relPath, "/")
	for i, seg := range segments {
		if seg == "" && i == 0 {
			// Leading slash (now impossible after we reject absolute paths,
			// but defense in depth)
			continue
		}
		if seg == "" {
			// Empty segment (e.g., double slash) — skip without error
			// (filepath.Clean handles this)
			continue
		}
		if seg == "." {
			// Single dot is allowed (cleaned away by filepath.Clean)
			continue
		}
		if seg == ".." {
			// Explicit rejection of traversal segments
			return "", fmt.Errorf("path segment is '..': %w", ErrDotDot)
		}
	}

	// Join the relative path under root and clean
	abs := filepath.Join(root, relPath)
	abs = filepath.Clean(abs)

	// Check that the joined path is still confined to root
	if !isConfined(abs, root) {
		return "", fmt.Errorf("path %s escapes root %s: %w", abs, root, ErrEscapesRoot)
	}

	// Resolve symlinks using the same logic as ConfinePath
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if !isConfined(resolved, root) {
			return "", fmt.Errorf("resolved path %s escapes root %s: %w", resolved, root, ErrEscapesRoot)
		}
		return resolved, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	// The target doesn't exist yet (new file/dir about to be written).
	// Walk up to the deepest ancestor that exists and verify it resolves
	// inside root. This allows creation in new subdirectories while still
	// catching symlink escapes.
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
			if !isConfined(resolvedParent, root) {
				return "", fmt.Errorf("ancestor %s (resolved to %s) escapes root %s: %w", parent, resolvedParent, root, ErrEscapesRoot)
			}
			return abs, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve ancestor %s: %w", parent, err)
		}
		// Parent doesn't exist; loop up to its parent.
	}
}

// Sentinel errors exported for use in errors.Is and tests.
var (
	ErrEmpty       = errors.New("empty component")
	ErrDot         = errors.New("component is '.'")
	ErrDotDot      = errors.New("component is '..'")
	ErrTraversal   = errors.New("component has '../' prefix")
	ErrAbsolute    = errors.New("component is absolute")
	ErrSeparator   = errors.New("component contains path separator")
	ErrLeadingDot  = errors.New("component has leading dot")
	ErrTooLong     = errors.New("component is too long")
	ErrControlChar = errors.New("component contains control character")
	ErrEscapesRoot = errors.New("path escapes root")
)
