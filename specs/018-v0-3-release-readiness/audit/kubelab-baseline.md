# kubelab Baseline

Captured by T012 before any change to kubelab. Machine-readable snapshot: [evidence/baseline/](evidence/baseline/). Secret-looking values are redacted; only key names appear.

## Helm release

| Field | Value |
|-------|-------|
| Release name | gameplane |
| Namespace | gameplane-system |
| Revision | 6 |
| Chart label | gameplane-0.2.0-beta.8 |
| App version | 0.2.0-beta.8 |
| Updated time | 2026-09-23 00:20:46.834221067 +0200 +0200 |
| Kube context | default |
| Status | deployed |

## Images in use

The release is labelled chart `0.2.0-beta.8` but runs side-loaded images from the private tag `gameplane-test/*:016`. The image registry is `gameplane-test` (image.registry) and tag is `016` (image.tag). The operator agent and sentinel images are also `:016` (operator.agentImage=`gameplane-test/agent:016`, operator.sentinelImage=`gameplane-test/sentinel:016`).

| Kind | Name | Namespace | Images |
|------|------|-----------|--------|
| Deployment | gameplane-api | gameplane-system | gameplane-test/api:016 |
| Deployment | gameplane-operator | gameplane-system | gameplane-test/operator:016 |
| Deployment | gameplane-web | gameplane-system | gameplane-test/web:016 |

## Helm values (user-supplied, redacted)

Keys matching `secret|token|password|key|dsn` are redacted by the snapshot script; none were present.

```json
{
  "crds": {
    "autoApply": {
      "enabled": false
    }
  },
  "defaultModuleSource": {
    "git": {
      "ref": "main",
      "url": "https://github.com/ValgulNecron/gameplane-module.git"
    },
    "type": "git"
  },
  "image": {
    "pullPolicy": "IfNotPresent",
    "registry": "gameplane-test",
    "tag": "016"
  },
  "ingress": {
    "host": "gameplane.local"
  },
  "operator": {
    "addressManager": "metallb",
    "agentImage": "gameplane-test/agent:016",
    "sentinelImage": "gameplane-test/sentinel:016"
  }
}
```

## Site-specific settings to preserve (FR-018)

- `image.registry`: gameplane-test
- `image.tag`: 016
- `operator.agentImage`: gameplane-test/agent:016
- `operator.sentinelImage`: gameplane-test/sentinel:016
- `operator.addressManager`: metallb
- `ingress.host`: gameplane.local
- `defaultModuleSource`: git https://github.com/ValgulNecron/gameplane-module.git ref main (note: differs from chart default ref v0.2.0-beta.6)
- `crds.autoApply.enabled`: false

## Namespaces and ingress

- API namespace: gameplane-system
- Games namespace: gameplane-games
- Ingress: gameplane-system/gameplane host gameplane.local (does not resolve from the audit devbox; see OD-016 in ../OPEN-DECISIONS.md)
- Service gameplane-api: ClusterIP port 80
- Service gameplane-web: ClusterIP port 80

## Nodes

| Name | UID | Roles | Schedulable | Kubelet Version |
|------|-----|-------|-------------|-----------------|
| kubelab-control | 7dc653ab-d4d2-4ff8-8939-00c827743102 | control-plane | true | v1.36.2+k3s1 |
| kubelab-worker-1 | daf806bc-1137-4a84-8126-2f0bcdc9403f | (worker) | true | v1.36.2+k3s1 |
| kubelab-worker-2 | a0d20e24-d778-41c8-8584-46198f35e820 | (worker) | true | v1.36.2+k3s1 |

## Pre-existing resources

### GameServers

| Name | Namespace | UID | Generation | Phase |
|------|-----------|-----|-----------|-------|
| mc-fabric | gameplane-games | 2d67c5ad-ff8f-4e26-a188-be5ea9b6ad0b | 4 | Running |
| soak-bogus-pool | gameplane-games | e12cb363-e632-4433-9bec-deae2e9d5a57 | 1 | Running |
| soak-no-preference | gameplane-games | 62412014-b54a-4da4-b232-4e5543a80c90 | 1 | Running |
| soak-pool-west | gameplane-games | 65e25ef6-3eda-4eb4-935a-1f2e45389a77 | 1 | Running |
| squad | gameplane-games | a86ae4e2-ef35-4463-8202-657daff52236 | 1 | Failed |

Note: `squad` is pre-existing with phase Failed and its pod in ImagePullBackOff. The audit does not touch it.

### ModuleSources

| Name | UID | Generation |
|------|-----|-----------|
| default | 7af97f90-c94a-4981-b7a9-53ce894a8810 | 3 |
| uploads | e59e60d6-f77b-4814-a1e7-01beb074c43e | 1 |

Count: 2 ModuleSources

### GameTemplates

Count: 30 GameTemplates.

| Name | Generation | Name | Generation |
|------|-----------|------|-----------|
| 7-days-to-die | 3 | mount-and-blade-2-bannerlord | 1 |
| ark-survival-ascended | 3 | palworld | 2 |
| ark-survival-evolved | 1 | probe-palworld | 1 |
| arma-reforger | 1 | project-zomboid | 2 |
| beammp | 1 | rust | 2 |
| cs2 | 3 | satisfactory | 3 |
| dayz | 3 | squad | 1 |
| dont-starve-together | 3 | team-fortress-2 | 1 |
| enshrouded | 2 | terraria | 1 |
| euro-truck-simulator-2 | 1 | the-isle | 1 |
| factorio | 3 | tmodloader | 1 |
| farming-simulator-25 | 1 | v-rising | 2 |
| fivem | 1 | valheim | 3 |
| garrys-mod | 2 | | |
| hell-let-loose | 1 | | |
| left-4-dead-2 | 2 | | |
| minecraft-java | 1 | | |

See evidence/baseline/crd-gametemplates.json for full detail.

### Modules

Count: 30 Modules; 28 are generation 1, phase `Ready`. Exceptions: `minecraft-java` generation 2, phase `Pulling`; `nuclear-option` generation 1, phase `Failed`; `terraria` generation 2, phase `Failed`. See evidence/baseline/crd-modules.json for full detail.

### PVCs

| Name | Namespace | Capacity | Phase |
|------|-----------|----------|-------|
| mc-fabric-data | gameplane-games | 20Gi | Bound |
| mc-fabric-mods-latest-fabric | gameplane-games | 5Gi | Bound |
| soak-bogus-pool-data | gameplane-games | 5Gi | Bound |
| soak-bogus-pool-mods-stable | gameplane-games | 5Gi | Bound |
| soak-no-preference-data | gameplane-games | 5Gi | Bound |
| soak-no-preference-mods-stable | gameplane-games | 5Gi | Bound |
| soak-pool-west-data | gameplane-games | 5Gi | Bound |
| soak-pool-west-mods-stable | gameplane-games | 5Gi | Bound |
| squad-data | gameplane-games | 20Gi | Bound |
| gameplane-api-data | gameplane-system | 2Gi | Bound |

### Other CRDs

- Backups: empty
- BackupSchedules: empty
- Restores: empty
- NetworkCaptures: empty
- Clusters: empty

## Pre-existing game server placement

Observed 2026-09-23 with kubectl get pods -o wide:

- `mc-fabric-0` on kubelab-control
- `soak-bogus-pool-0` on kubelab-control
- `soak-no-preference-0` on kubelab-worker-1
- `squad-0` on kubelab-worker-1
- `soak-pool-west-0` on kubelab-worker-2

Note: relevant to OD-017 (node-loss test node selection).

## API state (pending)

The API user list, role list, module-source list, auth-provider names and notification-sink names need one admin login. Pending OD-015 (admin access) and OD-016 (API reachability).

## Migration level and OD-005 path (pending)

`v0.2.0-beta.8` ships API migrations up to `006_share_links.sql` (git ls-tree v0.2.0-beta.8 api/internal/db/migrations/); `master` has up to `011_user_theme_preferences.sql`. kubelab runs images built from the 016 branch, so its DB is very likely at migration 011, which would make OD-005 path (b) apply. Reading the DB's applied migration level needs `kubectl exec` into the API pod, blocked in the 2026-09-23 session (OD-018). Not yet confirmed.
