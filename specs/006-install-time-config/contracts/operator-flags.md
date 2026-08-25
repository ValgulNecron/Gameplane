# Operator Flags Contract: Install-Time Configuration

## 1. New Operator Flag: Game Data Storage Class

**Flag Name**: `--game-data-storage-class`

**Type**: `string`

**Default Value**: `""` (empty string)

**Format**:
```
--game-data-storage-class=<storage-class-name>
```

**Semantics**:
- Specifies the Kubernetes StorageClass name to use for game server data volumes when neither a GameServer nor its template explicitly override it.
- Empty string (default) means the cluster's default StorageClass is used (Kubernetes behavior when `pvc.Spec.StorageClassName` is nil).
- Non-empty string must be a valid StorageClass name in the cluster; if the class does not exist, PVC provisioning fails and GameServer status reflects `StorageClassNotFound` error.

**Help Text** (as in `operator/cmd/main.go`):
```
StorageClass for game server data volumes (GameServer spec.storage.storageClassName). 
Empty (default) uses the cluster's default StorageClass. 
Applies when a GameServer does not explicitly set spec.storage.storageClassName.
```

**Source**: Helm value `operator.gameDataStorage.storageClassName`

**Requirement**: FR-006

---

## 2. Storage Class Precedence Chain

**Operator Injection Point**: Three PVC reconciliation functions in `operator/internal/controller/`:
1. `reconcilePVC()` (gameserver_controller.go, after line 524-525)
2. `reconcileExtraPVCs()` (gameserver_extravolumes.go, after line 129)
3. `reconcileModPVC()` (gameserver_version.go, after line 212)

**Precedence Order** (highest to lowest):
1. **GameServer.Spec.Storage.StorageClassName** (explicit server override)
   - If set (non-nil): use this value
   - Type: `*string` (pointer, can be nil)

2. **GameTemplate.Spec.Storage.StorageClassName** (explicit template default)
   - If step 1 is nil AND this is set: use this value
   - Type: `*string` (pointer, can be nil)

3. **`--game-data-storage-class` operator flag** (install-time default)
   - If steps 1-2 return nil AND this flag is non-empty: use this value
   - Type: `string` (can be empty)

4. **Cluster default StorageClass** (Kubernetes cluster-wide default)
   - If all above are nil/empty: `pvc.Spec.StorageClassName` remains nil
   - Kubernetes automatically uses the cluster's default StorageClass

**Code Pattern** (pseudocode for injection):
```go
if pvc.Spec.StorageClassName == nil && r.DefaultStorageClassName != "" {
    className := r.DefaultStorageClassName
    pvc.Spec.StorageClassName = &className
}
```

---

## 3. Operator Struct Changes

**Struct**: `GameServerReconciler` (operator/internal/controller/gameserver_controller.go)

**New Field**:
```go
type GameServerReconciler struct {
    // ... existing fields (Client, Scheme, AgentImage, SentinelImage, etc.) ...
    
    // DefaultStorageClassName is the install-time default for game server data volumes.
    // Set from the --game-data-storage-class CLI flag.
    // Empty string means use cluster default.
    DefaultStorageClassName string
}
```

**Initialization** (in `operator/cmd/main.go`):
```go
var gameDataStorageClass string
flag.StringVar(&gameDataStorageClass, "game-data-storage-class", "",
    "StorageClass for game server data volumes (GameServer spec.storage.storageClassName). "+
        "Empty (default) uses the cluster's default StorageClass. "+
        "Applies when a GameServer does not explicitly set spec.storage.storageClassName.")

// ... later, when creating GameServerReconciler ...
reconcilers.GameServerReconciler.DefaultStorageClassName = gameDataStorageClass
```

---

## 4. Flag Naming Convention

**Naming Pattern** (matches existing 30 operator flags):
- Kebab-case (hyphens between words)
- Prefix matches functional area (e.g., `--game-ingress-*`, `--capture-*`, `--module-*`)
- Suffix is the parameter name (e.g., `--flag-name`)

**Existing Examples from operator/cmd/main.go**:
- `--agent-image` (string)
- `--leader-elect` (boolean)
- `--game-ingress-policy` (boolean; true enables, false disables ingress NetworkPolicy reconciliation)
- `--capture-default-retention-seconds` (int)
- `--game-ingress-from-cidr` (repeatable string)

**New Flag Alignment**:
- `--game-data-storage-class` fits the pattern (game-scoped, data-descriptive, storage-class is the value).

---

## 5. Environment Variable Fallback (Optional)

**Convention** (if env var support is added):
```
GAMEPLANE_GAME_DATA_STORAGE_CLASS
```

**Pattern** (if implemented):
```go
flag.StringVar(&gameDataStorageClass, "game-data-storage-class", os.Getenv("GAMEPLANE_GAME_DATA_STORAGE_CLASS"),
    "StorageClass for game server data volumes. ...")
```

**Note**: Helm provides values → CLI flags directly (via Deployment `args:`). Env vars are optional for direct CLI invocation or development.

---

## 6. Precedence with Other Install-Time Operators

This flag joins the existing set of install-time defaults passed via CLI flags:

| Flag | Type | Scope | Requirement |
|------|------|-------|-------------|
| `--agent-image` | string | agent Pod image | Base |
| `--sentinel-image` | string | sentinel Pod image | Base |
| `--game-data-storage-class` | string | game data PVC StorageClass | FR-006 |

All are passed unconditionally from Helm (if set) and override only when the corresponding CR field is nil.

---

## 7. Error Handling

**Error Condition**: Named StorageClass does not exist in the cluster.

**Detection**:
- During `reconcilePVC()` after StorageClassName is set, call a validation helper: `validateStorageClassExists(ctx, className)`.
- Perform a direct Kubernetes GET of the StorageClass resource.
- If 404 NotFound: record error reason "StorageClassNotFound" in GameServer.status.conditions.

**Error Surface**:
- GameServer.status.phase = "Failed"
- GameServer.status.conditions[] has a Ready=False condition with:
  - Reason: "StorageClassNotFound"
  - Message: `"StorageClass '<name>' not found in cluster"`

**User Remediation**:
- Create the missing StorageClass in the cluster, or
- Change `operator.gameDataStorage.storageClassName` in Helm values and re-run `helm upgrade`.

---

## 8. Testing Requirements

**Unit/Envtest**:
- Verify precedence: GameServer override > Template default > operator flag > nil
- Verify error path: PVC created with nonexistent class → reconcile detects 404 → status reflects error

**E2E**:
- Create GameServer with no explicit storage class; verify PVC inherits operator flag value.
- Test fallback to cluster default (empty flag).

---

## 9. Backward Compatibility

- **Existing operators** (without the flag): `DefaultStorageClassName` defaults to `""`, equivalent to no override. All PVCs fall through to cluster default. No change in behavior.
- **New flag with empty value**: Same as existing behavior. Opt-in explicit default via Helm values only.
