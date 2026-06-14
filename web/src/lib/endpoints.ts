import { APIError, api, csrfHeaders } from "@/lib/api";
import type {
  AuditEvent,
  Backup,
  BackupDestination,
  BackupSchedule,
  BannedPlayer,
  CatalogEntry,
  ClusterInfo,
  ClusterStats,
  ClusterView,
  ExtendedUser,
  GameServer,
  GameTemplate,
  InstalledMod,
  List,
  LoginProvidersResp,
  Module,
  NodeJoinInfo,
  ModuleSource,
  ModuleSourceSpec,
  PermissionGroup,
  PlayersResp,
  Restore,
  Role,
  RoleBinding,
  StatusReading,
  User,
} from "@/types";

// Typed wrappers around the generic api<T>() fetcher. Every URL and request
// shape is declared here once; callers reference these instead of restating
// path strings. The generic api<T>() remains available as an escape hatch.

export type LifecycleVerb = "start" | "stop" | "restart";
export type ModerateAction = "kick" | "ban" | "unban";

export interface ServerCreate {
  name: string;
  description?: string;
  templateRef: { name: string };
  config?: Record<string, string>;
  storage?: { size?: string };
  networking?: { expose?: string; hostname?: string };
  resources?: unknown;
  nodeSelector?: Record<string, string>;
}

function gameServerEnvelope(input: ServerCreate) {
  const { name, description, ...spec } = input;
  return {
    apiVersion: "kestrel.gg/v1alpha1",
    kind: "GameServer",
    metadata: {
      name,
      ...(description ? { annotations: { "kestrel.gg/description": description } } : {}),
    },
    spec,
  };
}

export const Servers = {
  list: () => api<List<GameServer>>("/servers"),
  get: (name: string) => api<GameServer>(`/servers/${name}`),
  create: (body: ServerCreate) =>
    api<GameServer>("/servers", { method: "POST", body: gameServerEnvelope(body) }),
  update: (name: string, body: GameServer) =>
    api<GameServer>(`/servers/${name}`, { method: "PUT", body }),
  remove: (name: string) =>
    api<void>(`/servers/${name}`, { method: "DELETE" }),
  lifecycle: (name: string, verb: LifecycleVerb) =>
    api<void>(`/servers/${name}:${verb}`, { method: "POST" }),
  clone: (name: string, newName: string) =>
    api<GameServer>(`/servers/${name}:clone`, { method: "POST", body: { newName } }),
  // Live module-declared metrics for the Overview tab. The agent returns
  // [] when the game has no RCON, so the UI hides the panel.
  status: (name: string) => api<StatusReading[]>(`/servers/${name}/status`),
  // Run a module-declared action (spec.capabilities.actions[]). params
  // are the user-supplied values for the action's declared inputs.
  runAction: (name: string, body: { id: string; params?: Record<string, string> }) =>
    api<{ ok: boolean; raw?: string }>(`/servers/${name}/actions/run`, {
      method: "POST",
      body,
    }),
  // Mod/plugin management (spec.capabilities.mods). The agent enforces
  // the install policy (host allowlist, size cap); the dashboard just
  // lists, installs by URL, and removes by name.
  mods: (name: string) => api<InstalledMod[]>(`/servers/${name}/mods`),
  installMod: (name: string, body: { url: string; name?: string }) =>
    api<InstalledMod>(`/servers/${name}/mods/install`, { method: "POST", body }),
  removeMod: (name: string, mod: string) =>
    api<void>(`/servers/${name}/mods?name=${encodeURIComponent(mod)}`, {
      method: "DELETE",
    }),
};

export const Templates = {
  list: () => api<List<GameTemplate>>("/templates"),
  get: (name: string) => api<GameTemplate>(`/templates/${name}`),
};

export const Cluster = {
  info: () => api<ClusterInfo>("/cluster/info"),
  stats: () => api<ClusterStats>("/cluster/stats"),
  view: () => api<ClusterView>("/cluster"),
  // Credential-minting ops (admin-only; 501 unless clusterOps is enabled).
  addNode: () => api<NodeJoinInfo>("/cluster/nodes:join", { method: "POST" }),
  // kubeconfig is a file download, so it bypasses api()'s JSON handling.
  kubeconfig: async (): Promise<Blob> => {
    const res = await fetch("/cluster/kubeconfig", {
      method: "POST",
      credentials: "include",
      headers: csrfHeaders(),
    });
    if (!res.ok) throw new APIError(res.status, await res.text().catch(() => ""));
    return res.blob();
  },
};

// envelope wraps a typed `spec` in the unstructured Kubernetes envelope the
// API expects for create/update. Either `name` or `generateName` is required.
function envelope<K extends string, S>(
  kind: K,
  ident: { name: string } | { generateName: string },
  spec: S,
) {
  return {
    apiVersion: "kestrel.gg/v1alpha1",
    kind,
    metadata: ident,
    spec,
  };
}

export interface BackupCreate {
  serverRef: { name: string };
  repoRef?: { name: string; key: string };
  strategy?: "restic-snapshot" | "volume-snapshot";
  quiesce?: boolean;
  tags?: string[];
  /** Either name (explicit) or generateName (server-assigned) must be set. */
  name?: string;
  generateName?: string;
}

export const Backups = {
  list: () => api<List<Backup>>("/backups"),
  get: (name: string) => api<Backup>(`/backups/${name}`),
  create: (opts: BackupCreate) => {
    const { name, generateName, ...spec } = opts;
    const ident = name ? { name } : { generateName: generateName ?? `${spec.serverRef.name}-manual-` };
    return api<Backup>("/backups", { method: "POST", body: envelope("Backup", ident, spec) });
  },
  remove: (name: string) =>
    api<void>(`/backups/${name}`, { method: "DELETE" }),
};

export interface ScheduleCreate {
  serverRef: { name: string };
  schedule: string;
  repoRef: { name: string; key: string };
  retention?: BackupSchedule["spec"]["retention"];
  suspend?: boolean;
  name?: string;
  generateName?: string;
}

export const Schedules = {
  list: () => api<List<BackupSchedule>>("/schedules"),
  get: (name: string) => api<BackupSchedule>(`/schedules/${name}`),
  create: (opts: ScheduleCreate) => {
    const { name, generateName, ...spec } = opts;
    const ident = name ? { name } : { generateName: generateName ?? `${spec.serverRef.name}-sched-` };
    return api<BackupSchedule>("/schedules", {
      method: "POST",
      body: envelope("BackupSchedule", ident, spec),
    });
  },
  // Read-modify-write: fetches the current object, applies changes to its
  // spec, and PUTs the merged result. Used for the suspend toggle.
  patchSpec: async (name: string, patch: Partial<BackupSchedule["spec"]>) => {
    const current = await api<BackupSchedule>(`/schedules/${name}`);
    const next = { ...current, spec: { ...current.spec, ...patch } };
    return api<BackupSchedule>(`/schedules/${name}`, { method: "PUT", body: next });
  },
  remove: (name: string) =>
    api<void>(`/schedules/${name}`, { method: "DELETE" }),
};

export interface RestoreCreate {
  backupRef: { name: string };
  serverRef: { name: string };
  name?: string;
  generateName?: string;
}

export const Restores = {
  list: () => api<List<Restore>>("/restores"),
  create: (opts: RestoreCreate) => {
    const { name, generateName, ...spec } = opts;
    const ident = name ? { name } : { generateName: generateName ?? "restore-" };
    return api<Restore>("/restores", { method: "POST", body: envelope("Restore", ident, spec) });
  },
  remove: (name: string) =>
    api<void>(`/restores/${name}`, { method: "DELETE" }),
};

export interface BackupDestinationCreate {
  name: string;
  url: string;
  password: string;
}

export const BackupDestinations = {
  list: () => api<List<BackupDestination>>("/backup-destinations"),
  get: (name: string) =>
    api<BackupDestination>(`/backup-destinations/${name}`),
  // POST is also used to rotate the password of an existing destination —
  // the server treats {name} as the upsert key.
  upsert: (body: BackupDestinationCreate) =>
    api<BackupDestination>("/backup-destinations", { method: "POST", body }),
  remove: (name: string) =>
    api<void>(`/backup-destinations/${name}`, { method: "DELETE" }),
};

export const Players = {
  snapshot: (server: string) => api<PlayersResp>(`/servers/${server}/players`),
  banned: (server: string) =>
    api<BannedPlayer[]>(`/servers/${server}/players/banned`),
  moderate: (
    server: string,
    action: ModerateAction,
    body: { name: string; reason?: string },
  ) =>
    api<{ ok: boolean; raw?: string }>(
      `/servers/${server}/players/${action}`,
      { method: "POST", body },
    ),
};

export interface UserCreate {
  username: string;
  displayName?: string;
  email?: string;
  role: string;
  password?: string;
}

// All fields optional; omit to leave the column unchanged. The API
// distinguishes "absent" from "empty string" so a caller can clear an
// email by sending "".
export interface UserUpdate {
  displayName?: string;
  email?: string;
  role?: string;
}

export const Users = {
  me: () => api<User>("/users/me"),
  list: () => api<ExtendedUser[]>("/users"),
  create: (body: UserCreate) =>
    api<ExtendedUser>("/users", { method: "POST", body }),
  update: (id: number, body: UserUpdate) =>
    api<ExtendedUser>(`/users/${id}`, { method: "PATCH", body }),
  remove: (id: number) =>
    api<void>(`/users/${id}`, { method: "DELETE" }),
  resetPassword: (id: number, password: string) =>
    api<void>(`/users/${id}/reset-password`, {
      method: "POST",
      body: { password },
    }),
  bindings: (id: number) => api<RoleBinding[]>(`/users/${id}/bindings`),
  addBinding: (id: number, body: RoleBinding) =>
    api<RoleBinding>(`/users/${id}/bindings`, { method: "POST", body }),
  removeBinding: (id: number, roleName: string, namespace: string) =>
    api<void>(`/users/${id}/bindings/${roleName}/${namespace}`, {
      method: "DELETE",
    }),
};

export interface RoleWrite {
  name?: string;
  description?: string;
  permissions: string[];
}

export const Roles = {
  list: () => api<Role[]>("/roles"),
  catalog: () => api<{ groups: PermissionGroup[] }>("/roles/permissions"),
  create: (body: RoleWrite & { name: string }) =>
    api<Role>("/roles", { method: "POST", body }),
  update: (name: string, body: Omit<RoleWrite, "name">) =>
    api<Role>(`/roles/${name}`, { method: "PATCH", body }),
  remove: (name: string) =>
    api<void>(`/roles/${name}`, { method: "DELETE" }),
};

export const Auth = {
  login: (body: { username: string; password: string }) =>
    api<User>("/auth/login", { method: "POST", body }),
  logout: () => api<void>("/auth/logout", { method: "POST" }),
  oidcStartURL: () => "/auth/oidc/start",
  // Public, pre-auth: which login methods are enabled + their labels.
  providers: () => api<LoginProvidersResp>("/auth/providers"),
};

export const Audit = {
  page: (limit: number, before: number) => {
    const qs = new URLSearchParams({ limit: String(limit) });
    if (before > 0) qs.set("before", String(before));
    return api<AuditEvent[]>(`/admin/audit?${qs.toString()}`);
  },
};

// FileEntry mirrors the agent's response shape (agent/openapi.yaml). The
// dashboard ignores `mode`, but it's typed here for completeness.
export interface FileEntry {
  name: string;
  path: string;
  size: number;
  dir: boolean;
  mode?: string;
  modTime?: string;
}

function filesBase(server: string): string {
  return `/servers/${encodeURIComponent(server)}/files`;
}

// Shared helper for the non-JSON file endpoints (read/write/mkdir/upload/
// delete). The agent speaks raw bytes and multipart for these, which the
// generic api<T>() helper can't express. Errors are still surfaced as
// APIError so callers and TanStack Query treat them uniformly.
async function filesFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const res = await fetch(input, { credentials: "include", ...init });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new APIError(res.status, text);
  }
  return res;
}

export const Files = {
  list: (server: string, path: string) =>
    api<FileEntry[]>(`${filesBase(server)}/list?path=${encodeURIComponent(path)}`),
  read: async (server: string, path: string): Promise<string> => {
    const res = await filesFetch(
      `${filesBase(server)}/read?path=${encodeURIComponent(path)}`,
    );
    return res.text();
  },
  write: async (
    server: string,
    path: string,
    content: string | Blob,
  ): Promise<void> => {
    await filesFetch(`${filesBase(server)}/write?path=${encodeURIComponent(path)}`, {
      method: "POST",
      headers: { "Content-Type": "application/octet-stream", ...csrfHeaders() },
      body: content,
    });
  },
  mkdir: async (server: string, path: string): Promise<void> => {
    await filesFetch(`${filesBase(server)}/mkdir?path=${encodeURIComponent(path)}`, {
      method: "POST",
      headers: csrfHeaders(),
    });
  },
  remove: async (
    server: string,
    path: string,
    recursive = false,
  ): Promise<void> => {
    const qs = `path=${encodeURIComponent(path)}${recursive ? "&recursive=true" : ""}`;
    await filesFetch(`${filesBase(server)}/delete?${qs}`, {
      method: "DELETE",
      headers: csrfHeaders(),
    });
  },
  upload: async (
    server: string,
    dir: string,
    files: FileList | File[],
  ): Promise<void> => {
    const fd = new FormData();
    for (const f of Array.from(files)) fd.append("files", f, f.name);
    await filesFetch(`${filesBase(server)}/upload?path=${encodeURIComponent(dir)}`, {
      method: "POST",
      headers: csrfHeaders(),
      body: fd,
    });
  },
  downloadURL: (server: string, path: string) =>
    `${filesBase(server)}/download?path=${encodeURIComponent(path)}`,
};

// Historical + live log access. download fetches the whole current log
// file from the agent as an attachment; the two stream paths are the
// live WebSocket sources the Logs tab toggles between.
export const Logs = {
  downloadURL: (server: string) =>
    `/servers/${encodeURIComponent(server)}/logs/download`,
  // Live tail of the configured game log file, via the agent (mTLS).
  fileStreamPath: (server: string) =>
    `/ws/servers/${encodeURIComponent(server)}/logs`,
  // Live stream of the game container's stdout via the pod-log API.
  // Shows download/config output during startup — before the game's own
  // log file exists — and works even when agent mTLS isn't configured.
  podStreamPath: (server: string) =>
    `/ws/servers/${encodeURIComponent(server)}/logs/pod?from=start`,
};

// Module catalog and install/uninstall surface. The dashboard reads
// the merged catalog from /modules/catalog and drives installs by
// creating Module CRs through /modules.

export interface InstallRequest {
  source: string;
  module: string;
  name?: string;
  version?: string;
}

export const Modules = {
  catalog: () => api<List<CatalogEntry>>("/modules/catalog"),
  list: () => api<List<Module>>("/modules"),
  get: (name: string) => api<Module>(`/modules/${name}`),
  install: (body: InstallRequest) =>
    api<Module>("/modules", { method: "POST", body }),
  upgrade: (name: string, version: string) =>
    api<Module>(`/modules/${name}`, { method: "PATCH", body: { version } }),
  uninstall: (name: string) =>
    api<void>(`/modules/${name}`, { method: "DELETE" }),
};

// UploadedModule is the parsed-metadata echo from a bundle upload
// (used as the dry-run preview before committing).
export interface UploadedModule {
  module: {
    name: string;
    displayName: string;
    version: string;
    game: string;
    summary?: string;
  };
  configMap?: string;
  dryRun?: boolean;
}

async function uploadBundle(
  source: string,
  file: Blob,
  opts: { dryRun?: boolean } = {},
): Promise<UploadedModule> {
  const path = `/modules/sources/${source}/upload${opts.dryRun ? "?dryRun=true" : ""}`;
  const res = await fetch(path, {
    method: "POST",
    headers: csrfHeaders(),
    credentials: "include",
    body: file,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new APIError(res.status, text);
  }
  return res.json() as Promise<UploadedModule>;
}

export const ModuleSources = {
  list: () => api<List<ModuleSource>>("/modules/sources"),
  create: (name: string, spec: ModuleSourceSpec) =>
    api<ModuleSource>("/modules/sources", { method: "POST", body: { name, ...spec } }),
  update: (name: string, spec: ModuleSourceSpec) =>
    api<ModuleSource>(`/modules/sources/${name}`, { method: "PUT", body: spec }),
  remove: (name: string) =>
    api<void>(`/modules/sources/${name}`, { method: "DELETE" }),
  upload: uploadBundle,
  removeUpload: (source: string, module: string) =>
    api<void>(`/modules/sources/${source}/upload/${module}`, { method: "DELETE" }),
};
