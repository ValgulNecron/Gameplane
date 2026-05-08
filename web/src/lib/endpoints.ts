import { api } from "@/lib/api";
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
  List,
  Module,
  ModuleSource,
  PlayersResp,
  Restore,
  User,
} from "@/types";

// Typed wrappers around the generic api<T>() fetcher. Every URL and request
// shape is declared here once; callers reference these instead of restating
// path strings. The generic api<T>() remains available as an escape hatch.

export type LifecycleVerb = "start" | "stop" | "restart" | "clone";
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
};

export const Templates = {
  list: () => api<List<GameTemplate>>("/templates"),
  get: (name: string) => api<GameTemplate>(`/templates/${name}`),
};

export const Cluster = {
  info: () => api<ClusterInfo>("/cluster/info"),
  stats: () => api<ClusterStats>("/cluster/stats"),
  view: () => api<ClusterView>("/cluster"),
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
  role: User["role"];
  password?: string;
}

// All fields optional; omit to leave the column unchanged. The API
// distinguishes "absent" from "empty string" so a caller can clear an
// email by sending "".
export interface UserUpdate {
  displayName?: string;
  email?: string;
  role?: User["role"];
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
};

export const Auth = {
  login: (body: { username: string; password: string }) =>
    api<User>("/auth/login", { method: "POST", body }),
  logout: () => api<void>("/auth/logout", { method: "POST" }),
  oidcStartURL: () => "/auth/oidc/start",
};

export const Audit = {
  page: (limit: number, before: number) => {
    const qs = new URLSearchParams({ limit: String(limit) });
    if (before > 0) qs.set("before", String(before));
    return api<AuditEvent[]>(`/admin/audit?${qs.toString()}`);
  },
};

export interface FileEntry {
  name: string;
  path: string;
  size?: number;
  isDir?: boolean;
  modTime?: string;
}

export const Files = {
  list: (server: string, path: string) =>
    api<FileEntry[]>(
      `/servers/${server}/files/list?path=${encodeURIComponent(path)}`,
    ),
  read: (server: string, path: string) =>
    api<{ content: string; truncated?: boolean }>(
      `/servers/${server}/files/read?path=${encodeURIComponent(path)}`,
    ),
  write: (server: string, path: string, content: string) =>
    api<void>(`/servers/${server}/files/write`, {
      method: "POST",
      body: { path, content },
    }),
  mkdir: (server: string, path: string) =>
    api<void>(`/servers/${server}/files/mkdir`, {
      method: "POST",
      body: { path },
    }),
  remove: (server: string, path: string) =>
    api<void>(`/servers/${server}/files/delete`, {
      method: "POST",
      body: { path },
    }),
  downloadURL: (server: string, path: string) =>
    `/servers/${server}/files/download?path=${encodeURIComponent(path)}`,
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

export const ModuleSources = {
  list: () => api<List<ModuleSource>>("/modules/sources"),
};
