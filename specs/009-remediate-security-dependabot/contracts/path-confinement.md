# Path Confinement Helper Contract

## Signature

```go
// ConfinePath validates an untrusted path component against a root directory and returns 
// a safe absolute path confined within the root, or an error if the component violates 
// confinement rules.
//
// root: absolute path to a trusted root directory (e.g., /var/mods)
// component: untrusted path component from user input, archive member, or external source
// 
// Returns: a cleaned, absolute path confined within root, or an error if validation fails.
func ConfinePath(root, component string) (string, error)
```

---

## Contract Guarantees

### Postcondition (always true when error is nil)

1. **Returned path is cleaned**: The returned value satisfies `filepath.Clean(result) == result` — no redundant separators, no `.` or `..` elements.

2. **Returned path is confined**: One of the following holds:
   - `result == root` (component resolved to the root itself)
   - `strings.HasPrefix(result, root + string(os.PathSeparator))` (component resolved strictly under root with separator)

   This ensures the resolved path cannot escape the sandbox, even after symlink resolution.

3. **Both root and target are resolved**: If the returned path exists on disk and contains symlinks, those symlinks have been resolved. The deepest existing ancestor has also been resolved. Both the final target and all symlinks in its path remain inside root.

---

## Validation Rules (Rejection Table)

A call to `ConfinePath(root, component)` MUST return an error if any of the following is true:

| Condition | Reason | Example |
|-----------|--------|---------|
| `component == ""` | Empty path is meaningless | `ConfinePath("/var/mods", "") → error` |
| `component == "."` | Resolves to root; call `ConfinePath(root, "")` or handle explicitly | `ConfinePath("/var/mods", ".") → error` |
| `component == ".."` | Attempts traversal | `ConfinePath("/var/mods", "..") → error` |
| `strings.HasPrefix(component, "../")` or `component == ".."` | Traversal prefix | `ConfinePath("/var/mods", "../../../etc") → error` |
| `strings.HasPrefix(component, "/")` | Absolute path; use root instead | `ConfinePath("/var/mods", "/etc/passwd") → error` |
| `strings.Contains(component, string(os.PathSeparator))` | Separators indicate directory traversal; component MUST be a single path element | `ConfinePath("/var/mods", "subdir/file") → error` |
| `strings.HasPrefix(component, ".")` | Leading dot; component begins with `.` but is not exactly `"."` or `".."` | `ConfinePath("/var/mods", ".hidden") → error` |
| `len(component) > 200` | Length limit prevents excessively long path attacks | `ConfinePath("/var/mods", strings.Repeat("a", 201)) → error` |
| Contains control characters (U+0000–U+001F, U+007F) | Control chars in paths are always dangerous | `ConfinePath("/var/mods", "file\x00.txt") → error` |
| Contains backslash `\` | Windows path separator; normalize to forward slash or reject | `ConfinePath("/var/mods", "file\\dir") → error` |
| `filepath.Join(root, component)` escapes root after `Clean()` | The joined path, after cleaning, does not fall within root | (see symlink handling below) |
| Symlink target resolves outside root | The symlink, after resolution, points outside the sandbox | `ConfinePath("/var/mods", "link") → error` when `link → /etc/passwd` |
| Deepest existing ancestor is outside root | A symlink component in an ancestor path points outside root | `ConfinePath("/var/mods/foo/bar", "baz")` where `/var/mods/foo` is a symlink to `/tmp` |

---

## Symlink Handling (Critical)

When `component` or any ancestor directory is a symlink:

1. **Resolve the target**: Call `filepath.EvalSymlinks()` or equivalent to resolve the symlink to its final target.
2. **Check the final target**: After resolution, verify that the target path satisfies the postcondition (confined within root).
3. **Check intermediate ancestors**: Any ancestor directory that is a symlink MUST also resolve to a path inside root.
4. **Reject if escape is detected**: If a symlink escapes root (either directly or via its target), return an error immediately.

**Example**:
```
root = "/var/mods"
/var/mods/config is a symlink to /etc/config
component = "config/password"

After filepath.Join: /var/mods/config/password
After Clean: /var/mods/config/password
After EvalSymlinks on ancestor: /var/mods/config → /etc/config
Result: /etc/config/password is outside /var/mods → ERROR
```

---

## Call Sites — Consolidation Required

The following locations currently use ad-hoc path validation and MUST be migrated to use `ConfinePath`:

| File | Lines | Current Pattern | Note |
|------|-------|-----------------|------|
| `agent/internal/mods/mods.go` | 389 | `os.RemoveAll(target)` in `removeEntry` | Inline Join+Clean+HasPrefix guard; consolidate into ConfinePath call |
| `agent/internal/mods/mods.go` | 391 | `os.Remove(target)` in `removeEntry` | Same guard as line 389 |
| `agent/internal/mods/mods.go` | 446 | `os.Rename(tmpName, filepath.Join(...))` in `download()` | Guard is in caller (safeName); move into ConfinePath, called in download() |
| `agent/internal/mods/mods.go` | 486 | `os.RemoveAll(final)` in `swapInArchive` | Join+Clean+HasPrefix guard; consolidate |
| `agent/internal/mods/mods.go` | 490 | `os.Rename(staging, final)` in `swapInArchive` | Same guard as line 486 |
| `agent/internal/mods/mods.go` | 508 | (unzipInto) archive entry extraction | Pre-check and post-clean re-check on archive members; use ConfinePath |
| `agent/internal/mods/mods.go` | 594 | `os.Stat(target)` in `remove()` | In-function Join+Clean+HasPrefix guard; consolidate |

---

## Migration Pattern

After `ConfinePath` is defined, all call sites MUST:

1. **Call ConfinePath** before any file operation:
   ```go
   target, err := ConfinePath(h.dir, name)
   if err != nil {
     return fmt.Errorf("path confinement failed: %w", err)
   }
   // Use target, which is guaranteed safe
   os.RemoveAll(target)
   ```

2. **Use the returned value**: Never re-derive the path from the raw input. The returned value is what has been validated; re-deriving defeats the sanitizer recognition.

3. **Remove ad-hoc guards**: The old inline Join+Clean+HasPrefix checks MUST be deleted, not duplicated. `ConfinePath` is the single source of validation.

---

## Testing Requirements

- Unit tests MUST cover every rejection case in the table above.
- Symlink tests MUST verify both escape attempts (symlink target outside root) and safe cases (symlink target inside root).
- Integration tests MUST verify that all call sites use the returned validated path and do not re-derive from the raw input.
- A test case for archive extraction MUST verify that zip-slip entries (e.g., `../../etc/passwd`) are rejected.

---

## Relationship to Existing Helpers

- **`safeName()`** (agent/internal/mods/mods.go:625–645): Rejects path components with `.`, `..`, leading dot, len>200, control chars, `/`, `\`. Remains in use for initial filtering; `ConfinePath` is more comprehensive and includes symlink resolution.
- **`archiveFolderName()`** (agent/internal/mods/mods.go:557–565): Strips archive suffix (`.tar.gz`, `.tgz`, `.zip`). Orthogonal to confinement; works alongside `ConfinePath`.
- **`resolve()`** (agent/internal/files/files.go:57–98): The existing most-complete confinement helper, used for arbitrary file-service requests. `ConfinePath` SHOULD follow `resolve()`'s pattern for consistency across the agent.
- **`extractUploadArchive()`** (api/internal/handlers/module_upload.go:290–363): API-side archive extraction; uses `path.Clean` + reject absolute/`..`/`../` prefix. Should be aligned with the consolidated agent-side confinement.
