# Address Pool API Contract

**Feature**: 002-nuclear-option-ip-pool  
**Phase**: 1 (Contracts)  
**Date**: 2026-08-21  
**Status**: Proposal for Code Review  

This document specifies the additions to `GameServerNetworking` (spec FR-014 to FR-022, FR-025) that enable operators to pin a game server's public address to a chosen load-balancer address pool and optionally to a specific IP address.

---

## Overview

The load-balancer address-pool override feature allows operators to express a preference for which address pool a GameServer's external endpoint should draw from, and optionally to request a specific fixed address. The operator (via the Kubernetes cluster's address manager, e.g., MetalLB or Cilium) is responsible for provisioning pools. Gameplane records the preference on the GameServer CRD, translates it into the address manager's expected format (annotations for MetalLB, labels for Cilium), and reports back the assigned address and pool in the status.

**Module Location**: `operator/api/v1alpha1/gameserver_types.go` (struct definition), `operator/internal/controller/gameserver_controller.go` (reconciliation), `operator/internal/controller/gameserver_status.go` (status conditions).

---

## GameServerNetworking Field Additions

The following two fields are added to the `GameServerNetworking` struct (lines 225–276 of `operator/api/v1alpha1/gameserver_types.go`), after the existing `SourceRanges` field (line 268) and before the `Tunnel` field (line 275):

### `AddressPool` Field

```go
// AddressPool is the name of the load-balancer address pool from which
// the server's public address should be drawn. If specified and Expose=LoadBalancer,
// the operator translates this into the address manager's pool-selection mechanism
// (e.g., annotation for MetalLB, label for Cilium). Ignored if Expose is not LoadBalancer.
// If both addressPool and address are set, addressPool provides the pool preference
// and address requests a specific address within (or compatible with) that pool.
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`
// +optional
AddressPool string `json:"addressPool,omitempty"`

// Address is a specific IP address (or FQDN, depending on the address manager)
// that the operator requests for this server's public endpoint. If set and Expose=LoadBalancer,
// the operator requests this address from the address manager. If the address is already
// in use by another server, the status reports an error. If both addressPool and address are set,
// address takes priority for assignment (the address manager attempts to assign the requested
// address; if that fails, the request fails — the system does not fall back to the pool).
// Ignored if Expose is not LoadBalancer.
// Format and availability validation occur operator-side and are reported through the
// AddressAssignment status condition, not at admission time (to avoid CEL cost budgeting issues).
// +kubebuilder:validation:MaxLength=45
// +optional
Address string `json:"address,omitempty"`
```

**Kubebuilder Markers Rationale**:
- `AddressPool`: MaxLength 63 to match Kubernetes DNS label limits; Pattern enforces RFC 1123 subdomain syntax (lowercase alphanumerics and hyphens, no leading/trailing hyphen).
- `Address`: MaxLength 45 accommodates the longest possible IPv6 address. Format validation is deferred to the operator during reconciliation to avoid CEL cost budgeting constraints (prior CRDs in this repo have been rejected by the apiserver for unbounded CEL rules); invalid formats are reported as status conditions, not as validation rejections.

**Field Semantics**:
- Both fields are optional and independent of each other.
- `addressPool` alone: the server receives an address from that pool (any address within the pool).
- `address` alone: the server receives that specific address (if available).
- Both set: the server attempts to receive the specific `address`; if assignment fails, status reports the error.
- Neither set: backward-compatible; the server receives an address from the cluster's default pool.

**JSON Schema**: Both fields are JSON-omitempty. Zero values (empty string) serialize to absent fields; they do not appear in the CRD YAML unless set by the operator.

---

## Translation Contract: CRD → Service Annotations & Labels

The operator translates `AddressPool` and `Address` into cluster-specific address-manager metadata. The address manager (MetalLB, Cilium, etc.) is the source of truth for whether a pool exists, whether an address is available, and which address is ultimately assigned. Gameplane does not manage address allocation; it only conveys the operator's preference.

### MetalLB (v0.14.9+) Translation

**Pool Preference**:
- If `Expose == "LoadBalancer"` and `AddressPool` is set:
  - Apply Service annotation: `metallb.io/address-pool: <AddressPool>`
  - (Legacy prefix `metallb.universe.tf/address-pool` is deprecated as of MetalLB v0.14 but still honored; do not use it for new features.)

**Explicit Address**:
- If `Expose == "LoadBalancer"` and `Address` is set:
  - Apply Service annotation: `metallb.io/loadBalancerIPs: <Address>`
  - (Legacy prefix `metallb.universe.tf/loadBalancerIPs` is deprecated but still honored.)

**Precedence**:
- If both are set, both annotations are applied. MetalLB's behavior is to attempt the explicit `loadBalancerIPs` first; if that assignment succeeds, the pool annotation is ignored for that IP (the address is used as-is). If the address is unavailable, MetalLB fails the Service and does not fall back to the pool.

**Backward Compatibility**:
- If neither is set: no pool-related annotations are added. MetalLB assigns from its default pool (usually named "default").

### Cilium LB-IPAM Translation

**Pool Preference** (Asymmetric to MetalLB):
- If `Expose == "LoadBalancer"` and `AddressPool` is set:
  - Apply Service **label** (not annotation): `gameplane.local/lb-pool: <AddressPool>`
  - (This is a **Gameplane-chosen convention**, not a natively recognized Cilium label. The cluster administrator must configure `CiliumLoadBalancerIPPool.spec.serviceSelector` to match this label key in order for pool selection to work. If the selector is not configured to match, the label is set but nothing matches it, and the Service silently receives an address from the default pool — this is exactly the kind of silent no-op this project avoids.)

**Explicit Address**:
- If `Expose == "LoadBalancer"` and `Address` is set:
  - Apply Service annotation: `lbipam.cilium.io/ips: <Address>`
  - (Older Cilium v1.13–v1.14 use annotation key `io.cilium/lb-ipam-ips`; prefer the newer key for v1.15+.)

**Precedence**:
- If both are set: Cilium attempts the explicit address first (via annotation). If successful, that address is used. If the address is unavailable, Cilium fails to assign.
- Pool selection (via label) applies only if no explicit address is set, or if the explicit address fails.

**Backward Compatibility**:
- If neither is set: no pool labels or address annotations are added. Cilium uses its default assignment policy.

### Decision Table: Flavor × Settings → Service Mutation

| Address Manager | `addressPool` | `address` | Service Mutation |
|---|---|---|---|
| MetalLB | — | — | No pool/address annotations |
| MetalLB | set | — | Add `metallb.io/address-pool: <pool>` |
| MetalLB | — | set | Add `metallb.io/loadBalancerIPs: <addr>` |
| MetalLB | set | set | Add both annotations; MetalLB tries explicit first |
| Cilium | — | — | No pool labels or address annotations |
| Cilium | set | — | Add label `gameplane.local/lb-pool: <pool>` |
| Cilium | — | set | Add annotation `lbipam.cilium.io/ips: <addr>` |
| Cilium | set | set | Add both; Cilium tries address first |

**Important Asymmetry**: MetalLB uses annotations for both pool and address. Cilium uses a **label** for pool selection (matched by pool.spec.serviceSelector) and an **annotation** for explicit addresses. This asymmetry is a required modeling difference driven by each address manager's design.

---

## Deprecated Field: `loadBalancerIP`

**Constraint**: Kubernetes deprecated `Service.spec.loadBalancerIP` in v1.24. Gameplane MUST NOT set this field, regardless of the values of `AddressPool` or `Address`. All address requests flow through annotations/labels only. Any cloud-provider-specific handling (e.g., AWS LoadBalancer controller reading cloud provider's own annotations) is the cloud provider's responsibility, not Gameplane's.

---

## Idempotency & Managed Annotation & Label Ownership

The operator already tracks managed Service annotation keys via the `gameplane.local/managed-service-annotations` annotation (see `operator/internal/controller/gameserver_controller.go`, lines 474–530).

**Managed Key Registration**:
- Pool-derived annotation keys (e.g., `metallb.io/address-pool`, `metallb.io/loadBalancerIPs`, `lbipam.cilium.io/ips`) and labels (e.g., `gameplane.local/lb-pool`) are added to the managed set when the operator applies them.
- When an annotation/label key is removed from the desired set (e.g., the operator unsets `AddressPool`), the operator deletes the corresponding annotation or label from the Service in the next reconcile.
- This ensures that unsetting the field removes the annotation/label cleanly, without leaving stale keys that could interfere with future assignments.

**Implementation Detail** (per lines 509–531 of `gameserver_controller.go`):
- Call `desiredServiceAnnotations(gs)` to build the full desired map (including pool-derived keys).
- Pass the desired map to `applyManagedServiceAnnotations(svc, desired)`, which:
  - Reads the previous managed-key list from `svc.Annotations[managedServiceAnnotationsKey]`.
  - Deletes any keys that were managed last time but are absent from desired now.
  - Applies all desired keys.
  - Records the new managed-key list.

---

## Exposure Mode Compatibility & Warnings

**Constraint** (FR-018):
- Pool and address preferences MUST only take effect when `Expose == "LoadBalancer"`.
- If `Expose` is set to "ClusterIP", "NodePort", or "Hostport", any `AddressPool` or `Address` value is meaningless and MUST be ignored by the operator.
- The status MUST emit a warning condition (see Status Contract, below) if the operator detects that a preference is set but `Expose != "LoadBalancer"`.

**Dashboard UI Signal**:
- The dashboard MUST display a warning when the user sets a pool/address preference while `Expose` is not "LoadBalancer", stating: "Pool preference is ignored when exposure mode is not 'Load Balancer'."

---

## Backward Compatibility Guarantee (FR-017, SC-006)

When both `AddressPool` and `Address` are empty (the default):
- The Service produced by the operator is byte-identical to the Service produced before the pool-override feature existed.
- No new annotations or labels are added to the Service.
- The cluster's address manager assigns an address using its default policy (usually a "default" pool or round-robin allocation).
- Existing servers created before this feature continue to work unchanged.

---

## Status Contract: Conditions and Assigned Address Reporting

### New Condition Type: `AddressAssignment`

The operator emits a new condition type `AddressAssignment` on the GameServer status to report pool assignment outcomes.

**Condition Definition**:
```
Type: "AddressAssignment"
ObservedGeneration: gs.Generation
```

**Reason Vocabulary**:
- `"Assigned"`: The server has been assigned a public address. In this case, the address is available in `status.networking.address` (see Address Field, below).
- `"PoolNotFound"`: The requested pool does not exist in the cluster. Message includes the pool name.
- `"PoolExhausted"`: The requested pool has no available addresses. Message explains that the pool is in use or depleted.
- `"AddressInUse"`: The requested address is already assigned to another server. Message includes the conflicting server name.
- `"IgnoredForExposureMode"`: The pool/address preference was set, but `Expose` is not "LoadBalancer", so the preference was ignored. Status is not an error; it is informational.
- `"AssignmentPending"`: The address manager has not yet assigned an address. This is normal during the early phases of server startup (implies Status is False).
- `"ServiceNotReady"`: The Service exists but the address manager has not updated it with an assigned address yet (implies Status is False).
- `"ManagerFailure"`: The address manager (MetalLB, Cilium, etc.) reported an error when attempting assignment. Message includes the error from the address manager.

**Condition Status**:
- `Status: "True"` when a server has an assigned address (Reason="Assigned").
- `Status: "False"` when assignment is pending, ignored, or has failed.
- `Status: "Unknown"` is not used for this condition.

**Example Condition**:
```yaml
type: AddressAssignment
status: "True"
reason: Assigned
message: "Address 198.51.100.42 assigned from pool 'production-us-east'"
observedGeneration: 3
```

### Status.Networking.Address Field

A new field is added to `status.networking` (assuming a `NetworkingStatus` sub-struct exists; if not, add one):

```go
type NetworkingStatus struct {
    // Address is the public IP address (or FQDN) assigned to this server's external endpoint,
    // populated by the address manager (MetalLB, Cilium, cloud provider).
    // Empty if no address has been assigned yet.
    Address string `json:"address,omitempty"`
    
    // AddressPool records the name of the pool from which the address was drawn (if known).
    // Empty if the address was not drawn from a named pool.
    AddressPool string `json:"addressPool,omitempty"`
}
```

**Semantics**:
- `Address` is populated as soon as the address manager assigns an address (typically within seconds of the Service being created).
- `AddressPool` is set to the name of the pool if the address manager exposes that information (MetalLB does; Cilium may or may not, depending on configuration).
- Both are informational and read-only; they reflect what the cluster's address manager has assigned.

**Audit Trail**:
- The assigned address and pool are also surfaced in the audit event (spec FR-025) if audit logging is enabled, so operators can trace which pool/address each server ended up with.

---

## Precedence & Conflict Resolution

### User Sets Both Pool and Address

**Intended Behavior**:
- The operator translates both into annotations/labels and passes them both to the address manager.
- The address manager's policy determines which takes precedence. (MetalLB and Cilium both prefer the explicit address if set.)
- If the address is unavailable, the address manager fails (it does not fall back to the pool).
- The status reports the failure (e.g., `Reason="AddressInUse"`).

### User Sets Same Key in Both Typed Field and `ServiceAnnotations`

**Constraint**:
- The typed field (`AddressPool`, `Address`) is authoritative. If the user also sets a conflicting key in `ServiceAnnotations` (e.g., manually setting `metallb.io/address-pool` to a different value), the typed field wins.
- The operator includes pool-related keys in the managed annotation set, so the typed field's value overwrites any manual annotation.
- The dashboard MUST warn if it detects a conflict (user setting the same annotation key in both places).

### User Changes Pool/Address While Server is Running

**Intended Behavior**:
- Changing the preference on a running server may trigger a pod restart (depending on the address manager's behavior and cluster configuration).
- A brief service interruption may occur while the new address is assigned.
- Alternatively, the operator may defer the change until the next operator-initiated restart.
- **Design Note**: The exact behavior is a planning-phase decision (see Assumptions, spec.md). For v1, the operator applies the change immediately (reconcile updates the Service; the address manager handles the consequence). Future versions may add a "defer until restart" option.

---

## Conflict Detection & Error Reporting

### NonExistent Pool

**Scenario**: Operator requests pool `"us-west"`, but the cluster has no such pool.

**Detection**: The address manager (MetalLB, Cilium) does not recognize the pool name and either:
- Leaves the Service without an assigned address (address stays pending).
- Returns an explicit error/event to the Service.

**Operator's Responsibility**:
- Monitor the Service for lack of assignment or errors.
- Emit a condition `AddressAssignment.Status=False, Reason="PoolNotFound", Message="Pool 'us-west' not found or has no addresses"`.
- The dashboard displays this to the operator.

**Important caveat for Cilium**: Because `gameplane.local/lb-pool` is a Gameplane convention (not a natively recognized Cilium label), the operator **cannot distinguish** between "pool does not exist" and "cluster administrator never configured the `CiliumLoadBalancerIPPool.spec.serviceSelector` to match this label key." Both produce the same outcome: the Service receives no address or a default-pool address. The error message should reflect this ambiguity — the operator must check both that the pool exists AND that the selector is correctly configured.

**Timeline** (SC-003): Status must be updated within 30 seconds of the misconfiguration.

### Exhausted Pool

**Scenario**: Pool `"us-east"` exists but all addresses are in use.

**Detection**: The address manager cannot assign an address from the pool.

**Operator's Responsibility**:
- Emit a condition `AddressAssignment.Status=False, Reason="PoolExhausted", Message="Pool 'us-east' has no available addresses"`.

**Resolution**: Operator must either request a different pool, request a specific available address, or wait for another server in the pool to be deleted.

### Address In Use

**Scenario**: Operator requests address `198.51.100.42`, but it is already assigned to another server.

**Detection**: The address manager rejects the assignment.

**Operator's Responsibility**:
- Emit a condition `AddressAssignment.Status=False, Reason="AddressInUse", Message="Address 198.51.100.42 is already in use by server 'minecraft-prod'"` (ideally with a link to the conflicting server).

**Resolution**: Operator must either release the address (delete the other server), or choose a different address or pool.

---

## Test & Verification Scope

- **Unit**: CRD schema validation (kubebuilder markers are enforced).
- **Integration**: Reconciler produces correct Service annotations/labels given various combinations of `AddressPool`, `Address`, and `Expose`.
- **Envtest**: Operator reconciles a GameServer with pool preferences and verifies the Service is mutated correctly (annotations/labels are applied, managed-annotation tracking works).
- **E2E** (cluster-dependent): Real pool assignment is verified against a cluster with MetalLB or Cilium configured with test pools; confirm that servers receive addresses and status is updated correctly.

---

## File References

- **CRD Types**: `operator/api/v1alpha1/gameserver_types.go:225–276`
- **Reconciliation Logic**: `operator/internal/controller/gameserver_controller.go:470–530`
- **Managed Annotation Pattern**: `operator/internal/controller/gameserver_controller.go:474–531` (constants and `applyManagedServiceAnnotations` function)
- **Status Condition Pattern**: `operator/internal/controller/gameserver_status.go:210–300` (existing Condition updates)
