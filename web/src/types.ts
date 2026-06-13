// Minimal TypeScript surface mirroring the Kestrel CRDs. Kept lean —
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

// spec.capabilities — only the surfaces the dashboard renders are typed;
// players/quiesce are agent-side and left opaque here.
export interface GameCapabilities {
  players?: unknown;
  quiesce?: unknown;
  actions?: ServerActionDecl[];
  status?: { metrics?: StatusMetricDecl[] };
}

// One reading from GET /servers/{name}/status (the agent resolves each
// declared metric to a value; empty when the command didn't match).
export interface StatusReading {
  id: string;
  displayName?: string;
  value: string;
  unit?: string;
}

export interface GameTemplate {
  metadata: ObjectMeta;
  spec: {
    displayName: string;
    game: string;
    version: string;
    description?: string;
    icon?: string;
    accentColor?: string;
    image: string;
    logPath?: string;
    consoleMode?: "rcon" | "pty" | "none";
    rcon?: { protocol?: string; port?: number };
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
    image?: string;
    config?: Record<string, string>;
    env?: EnvVar[];
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
      playersOnline?: number;
      playersMax?: number;
      gameVersion?: string;
    };
    startedAt?: string;
  };
}

export interface Backup {
  metadata: ObjectMeta;
  spec: { serverRef: { name: string } };
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

export type UserRole = "admin" | "operator" | "viewer";

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
  version?: string;
}

export interface PlayerCapabilities {
  kick: boolean;
  ban: boolean;
  unban: boolean;
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
}
