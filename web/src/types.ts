// Minimal TypeScript surface mirroring the Gameplane CRDs. Kept lean —
// we use `unknown` for nested structures the UI doesn't yet render.

export interface ObjectMeta {
  name: string;
  namespace?: string;
  creationTimestamp?: string;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

// A single user-supplied input for a declared server action.
export interface ActionParamDecl {
  name: string;
  displayName?: string;
  description?: string;
  type: "string" | "int" | "bool" | "enum";
  default?: string;
  enum?: string[];
  required?: boolean;
}

// A module-declared operator action surfaced as a button on the server
// detail page (spec.capabilities.actions[]). `command` is the agent's
// concern; the UI renders displayName/params and POSTs id+params.
export interface ServerActionDecl {
  id: string;
  displayName: string;
  description?: string;
  icon?: string;
  command?: string;
  params?: ActionParamDecl[];
  confirm?: boolean;
  danger?: boolean;
}

// A module-declared live metric (spec.capabilities.status.metrics[]).
export interface StatusMetricDecl {
  id: string;
  displayName: string;
  command?: string;
  regex?: string;
  unit?: string;
}

// Install policy for the mods capability (spec.capabilities.mods.install).
export interface ModInstallPolicy {
  allowedHosts: string[];
  maxSizeMB?: number;
}

// spec.capabilities.mods.loaders[<loader>] — the mods directory for one
// loader/server-type, selected by the active version's loader.
export interface ModLoaderDecl {
  path: string;
  displayName?: string;
  extensions?: string[];
}

// spec.capabilities.mods.registry — selects a built-in external mod
// registry the dashboard can browse/search. When present (and the server's
// game supports it) the Mods tab offers a "Search registry" mode; absent
// means install-by-URL only.
export interface ModRegistryDecl {
  providers: ModProviderDecl[];
}

// One declared registry provider on a template.
export interface ModProviderDecl {
  provider: "modrinth" | "thunderstore" | "curseforge" | "hangar" | "factorio";
  community?: string;
  modpacks?: ModpackDecl;
}

// registry.providers[].modpacks — enables the Modpacks tab for that
// provider. refEnv set = env-mode install (e.g. Minecraft/itzg
// MODRINTH_MODPACK); empty = deps-mode (resolve + install the pack's deps).
export interface ModpackDecl {
  refEnv?: string;
  env?: { name: string; value?: string }[];
}

// GET /servers/{name}/mods/registry/providers — drives the provider switch.
export interface RegistryProviderInfo {
  provider: string;
  available: boolean;
  modpacks: boolean;
}

// spec.capabilities.mods — declares the mod directory and (optionally)
// the URL-install policy. A template uses either a single `path` (legacy)
// or a per-loader `loaders` map keyed by GameVersion.loader; install is
// offered only when `install` is set; `registry` enables in-app browse.
export interface ModsCapability {
  path?: string;
  loaders?: Record<string, ModLoaderDecl>;
  extensions?: string[];
  install?: ModInstallPolicy;
  registry?: ModRegistryDecl;
}

// spec.capabilities — only the surfaces the dashboard renders are typed;
// players/quiesce are agent-side and left opaque here.
export interface GameCapabilities {
  players?: unknown;
  quiesce?: unknown;
  actions?: ServerActionDecl[];
  status?: { metrics?: StatusMetricDecl[] };
  mods?: ModsCapability;
}

// Registry identity of an installed mod, recorded in the agent's install
// manifest at install time and echoed back in listings. provider "upload"
// marks direct uploads (never update-checked).
export interface ModMeta {
  provider: string;
  projectId?: string;
  projectName?: string;
  versionId?: string;
  versionNumber?: string;
  gameVersion?: string;
  loader?: string;
  sourceUrl?: string;
  installedAt?: string;
}

// One installed mod file, as returned by GET /servers/{name}/mods.
// meta is null/absent for unmanaged files (placed outside the panel).
export interface InstalledMod {
  name: string;
  size: number;
  modTime?: string;
  meta?: ModMeta | null;
}

// One available upgrade from GET /servers/{name}/mods/updates.
export interface ModUpdate {
  name: string;
  provider: string;
  projectId: string;
  projectName?: string;
  installedVersionId: string;
  installedVersionNumber?: string;
  latestVersionId: string;
  latestVersionNumber?: string;
  file: RegistryFile;
}

export interface ModUpdatesResponse {
  checkedAt: string;
  updates: ModUpdate[];
  errors?: { name: string; error: string }[];
}

// One reading from GET /servers/{name}/status (the agent resolves each
// declared metric to a value; empty when the command didn't match).
export interface StatusReading {
  id: string;
  displayName?: string;
  value: string;
  unit?: string;
}

// A Kubernetes probe. Only the timing fields are edited per-server; the
// action (httpGet / tcpSocket / exec) and anything else is carried through
// unchanged via the index signature.
export interface Probe {
  initialDelaySeconds?: number;
  periodSeconds?: number;
  timeoutSeconds?: number;
  failureThreshold?: number;
  successThreshold?: number;
  [key: string]: unknown;
}

export interface ProbeSet {
  readiness?: Probe;
  liveness?: Probe;
  startup?: Probe;
}

export type ProbeKind = "readiness" | "liveness" | "startup";

// One entry in a template's version catalog (spec.versions[]). Selecting it
// (GameServer.spec.version = id) pins the image and, when `loader` keys into
// capabilities.mods.loaders, that loader's per-(version+loader) mod volume.
// `env` is operator-side only and intentionally not surfaced here.
export interface GameVersion {
  id: string;
  displayName: string;
  image?: string;
  loader?: string;
  default?: boolean;
  // Clean upstream version token (e.g. "1.21.4") passed to a mod registry
  // to filter results; distinct from `id` (a Gameplane selector).
  gameVersion?: string;
}

// A search hit from the mod-registry browse endpoint, normalized across
// providers (GET /servers/{name}/mods/registry/search).
export interface RegistryProject {
  id: string;
  slug?: string;
  title: string;
  description?: string;
  author?: string;
  iconUrl?: string;
  downloads?: number;
  pageUrl?: string;
  provider: "modrinth" | "thunderstore" | "curseforge" | "hangar" | "factorio";
}

// A downloadable artifact of a RegistryVersion; downloadUrl is handed to
// the existing install endpoint. requiresAuth marks files the portal only
// serves with the user's own credentials (e.g. Factorio's username+token
// query params) — the UI offers a from-URL handoff instead of one-click
// install.
export interface RegistryFile {
  filename: string;
  downloadUrl: string;
  size?: number;
  primary?: boolean;
  requiresAuth?: boolean;
}

// One release of a RegistryProject (newest first), already filtered to the
// active loader + game version by the API.
export interface RegistryVersion {
  id: string;
  name?: string;
  versionNumber?: string;
  gameVersions?: string[];
  loaders?: string[];
  files: RegistryFile[];
}

export interface GameTemplate {
  metadata: ObjectMeta;
  spec: {
    displayName: string;
    game: string;
    category?: string;
    version: string;
    description?: string;
    icon?: string;
    accentColor?: string;
    image: string;
    versions?: GameVersion[];
    logPath?: string;
    consoleMode?: "rcon" | "pty" | "none";
    rcon?: { protocol?: string; port?: number };
    probes?: ProbeSet;
    capabilities?: GameCapabilities;
    configSchema?: Array<{
      name: string;
      displayName?: string;
      description?: string;
      type: "string" | "int" | "bool" | "enum" | "password";
      default?: string;
      enum?: string[];
      required?: boolean;
      target?: "env" | "file";
      autoFromMemoryLimit?: { percent: number };
    }>;
  };
  status?: { inUseCount?: number };
}

export type GameServerPhase =
  | "Pending" | "Starting" | "Running" | "Stopping" | "Stopped" | "Suspended" | "Failed";

export type Expose = "ClusterIP" | "NodePort" | "LoadBalancer" | "Hostport";

export interface SecretKeyRef {
  name: string;
  key: string;
  optional?: boolean;
}

export interface EnvVar {
  name: string;
  value?: string;
  valueFrom?: {
    secretKeyRef?: SecretKeyRef;
  };
}

export interface ResourceRequirements {
  requests?: Partial<Record<"cpu" | "memory", string>>;
  limits?: Partial<Record<"cpu" | "memory", string>>;
}

export interface PortOverride {
  name: string;
  servicePort?: number;
  nodePort?: number;
}

export interface GameServerNetworking {
  expose?: Expose;
  hostname?: string;
  serviceAnnotations?: Record<string, string>;
  portOverrides?: PortOverride[];
  sourceRanges?: string[];
}

export interface GameServerStorage {
  size?: string;
  storageClassName?: string;
  mountPath?: string;
}

export interface GameServer {
  metadata: ObjectMeta & { resourceVersion?: string };
  spec: {
    templateRef: { name: string };
    suspend?: boolean;
    // Seconds the operator waits for the template's stop sequence to run
    // over RCON before scaling the pod down (soft-stop). Range 0–600,
    // default 30. No effect when the template declares no stop sequence.
    stopGracePeriodSeconds?: number;
    image?: string;
    // Selects a GameTemplate.spec.versions[].id (image + per-loader mod
    // volume). Omit to use the template's default version.
    version?: string;
    config?: Record<string, string>;
    env?: EnvVar[];
    probes?: ProbeSet;
    resources?: ResourceRequirements;
    storage?: GameServerStorage;
    networking?: GameServerNetworking;
    nodeSelector?: Record<string, string>;
    serviceAccountName?: string;
  };
  status?: {
    phase?: GameServerPhase;
    endpoints?: Array<{ name: string; host: string; port: number; protocol?: string }>;
    agent?: {
      lastHeartbeat?: string;
      // null/absent means "unknown" (agent couldn't query the game, or
      // the heartbeat is stale and the API blanked it).
      playersOnline?: number | null;
      playersMax?: number;
      gameVersion?: string;
      // Resource usage the agent reads from its own cgroup + a statfs of
      // the data volume. null/absent means "unknown" (unreadable source,
      // or a stale heartbeat the API blanked) — render "—", not a zero.
      cpuMillicores?: number | null;
      cpuLimitMillicores?: number | null;
      memoryBytes?: number | null;
      memoryLimitBytes?: number | null;
      diskUsedBytes?: number | null;
      diskTotalBytes?: number | null;
      // Set by the API when the heartbeat is older than the freshness
      // window — the reported values are no longer current.
      stale?: boolean;
    };
    // Ready / Progressing / Healthy. The operator refines Progressing's
    // message while Starting (e.g. "pulling the game image") — the
    // dashboard surfaces it as a provisioning sub-status.
    conditions?: Array<{
      type: string;
      status: string;
      reason?: string;
      message?: string;
      lastTransitionTime?: string;
    }>;
    startedAt?: string;
  };
}

// A Kubernetes Event about a server's pod/StatefulSet/GameServer, as
// returned by GET /servers/{name}/events. Mirrors the API's PodEvent DTO.
export interface ServerEvent {
  id: string;
  time: string;
  type: string; // Normal | Warning
  reason: string;
  message: string;
  source: string;
  object: string;
  count: number;
}

export interface Backup {
  metadata: ObjectMeta;
  // strategy: restic-snapshot writes to a restic repo; volume-snapshot takes a
  // CSI snapshot and is restored by provisioning a NEW server from it.
  spec: { serverRef: { name: string }; strategy?: "restic-snapshot" | "volume-snapshot" };
  status?: {
    phase?: "Pending" | "Running" | "Succeeded" | "Failed";
    startTime?: string;
    completionTime?: string;
    snapshotID?: string;
    size?: string;
  };
}

export interface BackupSchedule {
  metadata: ObjectMeta;
  spec: {
    serverRef: { name: string };
    schedule: string;
    suspend?: boolean;
    retention?: {
      keepLast?: number;
      keepHourly?: number;
      keepDaily?: number;
      keepWeekly?: number;
      keepMonthly?: number;
      keepYearly?: number;
    };
  };
  status?: {
    lastSuccessfulTime?: string;
    nextScheduleTime?: string;
  };
}

export type RestorePhase =
  | "Pending" | "Suspending" | "Running" | "Resuming" | "Succeeded" | "Failed";

export interface Restore {
  metadata: ObjectMeta;
  spec: {
    backupRef: { name: string };
    serverRef: { name: string };
  };
  status?: {
    phase?: RestorePhase;
    snapshotID?: string;
    startTime?: string;
    completionTime?: string;
    message?: string;
  };
}

// BackupDestination is a labelled Kubernetes Secret holding restic repo
// credentials. The API only ever returns a redacted projection — the
// password is never shipped to the browser; `hasPassword` confirms it
// is stored. Mutating the password requires re-POSTing both fields.
export interface BackupDestination {
  name: string;
  url: string;
  hasPassword: boolean;
  createdAt?: string;
}

// Role names are open-ended now that custom roles exist. BuiltinRole is
// kept for the few places that special-case the seeded roles (e.g. the
// role badge colors).
export type UserRole = string;
export type BuiltinRole = "admin" | "operator" | "viewer";

// "pending" = local account that has never set a password (e.g. created
// before the admin attached one); "oidc" = bound to an external IdP via
// the oidc_links table; "local" = password-backed local account.
export type UserProvider = "local" | "oidc" | "pending";

export interface User {
  id: number;
  username: string;
  displayName: string;
  email: string;
  role: UserRole;
  provider?: UserProvider;
  createdAt?: string;
  // Effective permission set keyed by namespace ("*" = cluster-wide; a "*"
  // permission means all). Present on /users/me; drives can()-based UI
  // gating. Absent elsewhere.
  permissions?: Record<string, string[]>;
}

// A named set of catalog permissions.
export interface Role {
  name: string;
  description: string;
  builtin: boolean;
  permissions: string[];
}

// One grantable permission and whether it is namespaced.
export interface Permission {
  key: string;
  label: string;
  namespaced: boolean;
}

// Permissions grouped by resource, for the permission picker.
export interface PermissionGroup {
  resource: string;
  label: string;
  permissions: Permission[];
}

// A user's role grant in a namespace ("*" = cluster-wide).
export interface RoleBinding {
  roleName: string;
  namespace: string;
}

export type ExtendedUser = User;

export interface List<T> {
  items: T[];
}

export interface ClusterStats {
  nodes?: number;
  totalStorageBytes?: number;
  usedStorageBytes?: number;
}

export interface ClusterNode {
  name: string;
  roles?: string[];
  status?: "Ready" | "NotReady" | string;
  uptime?: string;
  startedAt?: string;
  pods?: { used?: number; capacity?: number };
  cpu?: { used?: number; capacity?: number };
  memory?: { used?: number; capacity?: number };
  labels?: string[];
}

export interface ClusterView {
  nodes?: ClusterNode[];
  version?: string;
  name?: string;
  ready?: number;
  total?: number;
}

export interface ClusterInfo {
  clusterName?: string;
  version?: string; // Kubernetes server version
  gameplaneVersion?: string; // Gameplane control-plane build
  clusterOps?: boolean; // node-join / kubeconfig minting enabled on this install
  updateChannel?: string; // informational chart updates.channel label
}

export interface NodeJoinInfo {
  command: string;
  token: string;
  caCertHash: string;
  endpoint: string;
  expiresAt: string;
}

export interface LoginProvider {
  name?: string; // route slug for /auth/oidc/{name}/start ("helm" = the Helm-flag provider)
  kind: "local" | "oidc" | string;
  label: string;
}

export interface LoginProvidersResp {
  providers: LoginProvider[];
}

export interface PlayerCapabilities {
  kick: boolean;
  ban: boolean;
  unban: boolean;
  whitelist?: boolean;
}

export interface PlayersResp {
  online: number;
  max: number;
  players: string[];
  asOf: string;
  capabilities: PlayerCapabilities;
}

export interface BannedPlayer {
  name: string;
  reason?: string;
  source?: string;
}

export interface AuditEvent {
  id: number;
  ts: string;
  actor: string;
  method: string;
  path: string;
  target?: string;
  status: number;
  ip?: string;
}

// Module catalog types — shape of /modules and /modules/* responses.

export type ModuleSourceType = "oci" | "git" | "http" | "local" | "upload";

export interface OCISourceSpec {
  url: string;
  modules: Array<{ name: string }>;
  insecure?: boolean;
  pullSecretRef?: { name: string };
}

export interface GitSourceSpec {
  url: string;
  ref?: string;
  subPath?: string;
  secretRef?: { name: string };
}

export interface HTTPSourceSpec {
  url: string;
  secretRef?: { name: string };
  insecure?: boolean;
}

export interface LocalSourceSpec {
  path?: string;
}

// Keyless (Fulcio) cosign verification: the signing certificate must carry
// this OIDC issuer and SAN identity.
export interface ModuleVerifyKeyless {
  issuer: string;
  identity: string;
}

// Cosign signature policy mirroring ModuleSource.spec.verify. Exactly one of
// key/keyless is set. The CRD restricts verify to OCI sources via CEL.
export interface ModuleVerifySpec {
  // Keyed verification: a Secret holding the cosign public key (cosign.pub).
  key?: { name: string };
  keyless?: ModuleVerifyKeyless;
}

// Discriminated union mirroring ModuleSourceSpec on the CRD: exactly
// the nested config matching `type` is set (upload needs none).
export interface ModuleSourceSpec {
  type?: ModuleSourceType;
  oci?: OCISourceSpec;
  git?: GitSourceSpec;
  http?: HTTPSourceSpec;
  local?: LocalSourceSpec;
  allow?: string[];
  refreshInterval?: string;
  // Only meaningful for OCI sources (CEL-enforced on the CRD).
  verify?: ModuleVerifySpec;
}

export interface ModuleEntry {
  name: string;
  displayName?: string;
  summary?: string;
  game?: string;
  icon?: string;
  reference?: string;
  versions?: string[];
  latestVersion?: string;
  digest?: string;
}

// SourceRef names one ModuleSource offering a catalog entry, with its
// type so the UI can badge where the module comes from.
export interface SourceRef {
  name: string;
  type: ModuleSourceType;
}

export interface ModuleSource {
  metadata: ObjectMeta;
  spec: ModuleSourceSpec;
  status?: {
    lastSync?: string;
    modules?: ModuleEntry[];
    conditions?: Array<{
      type: string;
      status: string;
      reason?: string;
      message?: string;
      lastTransitionTime?: string;
    }>;
  };
}

export type ModulePhase = "Pending" | "Pulling" | "Ready" | "Failed";

export interface Module {
  metadata: ObjectMeta;
  spec: {
    source: { name: string };
    name: string;
    version?: string;
  };
  status?: {
    phase?: ModulePhase;
    appliedVersion?: string;
    appliedDigest?: string;
    appliedTemplate?: string;
    previousVersion?: string;
    previousDigest?: string;
    lastError?: string;
    conditions?: Array<{
      type: string;
      status: string;
      reason?: string;
      message?: string;
    }>;
  };
}

// CatalogEntry is the merged-view row served by /modules/catalog. The
// API joins each ModuleSource.status.modules with the live Module CRs
// to produce one row per logical module name.
export interface CatalogEntry {
  name: string;
  displayName?: string;
  summary?: string;
  game?: string;
  category?: string;
  icon?: string;
  sources: SourceRef[];
  versions?: string[];
  latestVersion?: string;
  digest?: string;
  installed: boolean;
  installedVersion?: string;
  installedFrom?: string; // ModuleSource name
  moduleName?: string;    // Module CR name (= GameTemplate name)
  phase?: ModulePhase;
  lastError?: string;
  appliedDigest?: string;    // digest of the installed bundle
  previousVersion?: string;  // rollback target (operator-owned)
  previousDigest?: string;
}
