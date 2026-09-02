// Rich, deterministic data set for screenshot/demo purposes, built from
// existing factories to keep shapes in sync. Used when
// localStorage.getItem("gameplane-e2e-dataset") === "screenshots".

import type {
  GameServer,
  GameTemplate,
  User,
  AuditEvent,
  ClusterView,
  ServerEvent,
  InstalledMod,
  RegistryProject,
} from "@/types";
import type { AllConfig } from "@/lib/config";
import {
  makeAudit,
  makeClusterView,
  makeConfig,
  makeRestore,
  makeSchedule,
  makeServer,
  makeTemplate,
  makeUser,
} from "./factories";

// ============================================================================
// GameTemplates: 8 official templates with short descriptions
// ============================================================================

export const screenshotTemplates: GameTemplate[] = [
  makeTemplate({
    metadata: { name: "minecraft-java" },
    spec: {
      displayName: "Minecraft Java Edition",
      game: "minecraft",
      version: "1.21",
      description: "Official Minecraft Java Edition server",
      image: "ghcr.io/valgulnecron/gameplane/minecraft:1.21",
    },
  }),
  makeTemplate({
    metadata: { name: "valheim-default" },
    spec: {
      displayName: "Valheim",
      game: "valheim",
      version: "0.218",
      description: "Norse exploration and survival game",
      image: "ghcr.io/valgulnecron/gameplane/valheim:0.218",
      capabilities: {
        mods: {
          path: "BepInEx/plugins",
          extensions: [".dll", ".zip"],
          install: { allowedHosts: ["thunderstore.io", "gcdn.thunderstore.io"] },
          registry: { providers: [{ provider: "thunderstore", community: "valheim" }] },
        },
      },
    },
  }),
  makeTemplate({
    metadata: { name: "terraria-vanilla" },
    spec: {
      displayName: "Terraria",
      game: "terraria",
      version: "1.4.4",
      description: "2D sandbox with mining and exploration",
      image: "ghcr.io/valgulnecron/gameplane/terraria:1.4.4",
    },
  }),
  makeTemplate({
    metadata: { name: "rust-vanilla" },
    spec: {
      displayName: "Rust",
      game: "rust",
      version: "2026.01",
      description: "Survival multiplayer game",
      image: "ghcr.io/valgulnecron/gameplane/rust:2026.01",
    },
  }),
  makeTemplate({
    metadata: { name: "palworld-default" },
    spec: {
      displayName: "Palworld",
      game: "palworld",
      version: "0.3.5",
      description: "Pokemon-like survival crafting MMO",
      image: "ghcr.io/valgulnecron/gameplane/palworld:0.3.5",
    },
  }),
  makeTemplate({
    metadata: { name: "factorio-vanilla" },
    spec: {
      displayName: "Factorio",
      game: "factorio",
      version: "1.1.105",
      description: "Industrial automation and logistics",
      image: "ghcr.io/valgulnecron/gameplane/factorio:1.1.105",
    },
  }),
  makeTemplate({
    metadata: { name: "cs2-competitive" },
    spec: {
      displayName: "Counter-Strike 2",
      game: "cs2",
      version: "2026.02",
      description: "Competitive tactical first-person shooter",
      image: "ghcr.io/valgulnecron/gameplane/cs2:2026.02",
    },
  }),
  makeTemplate({
    metadata: { name: "ark-ascended" },
    spec: {
      displayName: "ARK: Survival Ascended",
      game: "ark",
      version: "1.30.2",
      description: "Dinosaur survival game",
      image: "ghcr.io/valgulnecron/gameplane/ark:1.30.2",
    },
  }),
];

// ============================================================================
// GameServers: 5 servers with diverse phases and namespaces
// ============================================================================

export const screenshotServers: GameServer[] = [
  makeServer({
    metadata: { name: "test-server-01", namespace: "default" },
    spec: { templateRef: { name: "minecraft-java" } },
    status: {
      phase: "Running",
      agent: { playersOnline: 5, playersMax: 20, lastHeartbeat: "2026-09-02T15:45:30Z" },
      startedAt: "2026-08-28T10:30:00Z",
    },
  }),
  makeServer({
    metadata: { name: "test-server-02", namespace: "default" },
    spec: { templateRef: { name: "valheim-default" } },
    status: {
      phase: "Running",
      agent: { playersOnline: 2, playersMax: 10, lastHeartbeat: "2026-09-02T15:44:15Z" },
      startedAt: "2026-08-01T00:00:00Z",
    },
  }),
  makeServer({
    metadata: { name: "test-server-03", namespace: "gameplane-demo" },
    spec: { templateRef: { name: "terraria-vanilla" } },
    status: {
      phase: "Pending",
      agent: { playersOnline: null, playersMax: 16, lastHeartbeat: "2026-09-02T15:42:00Z" },
      startedAt: "2026-09-02T15:40:00Z",
    },
  }),
  makeServer({
    metadata: { name: "test-server-04", namespace: "gameplane-demo" },
    spec: { templateRef: { name: "rust-vanilla" } },
    status: {
      phase: "Failed",
      agent: { playersOnline: null, playersMax: 128, lastHeartbeat: undefined },
      startedAt: undefined,
    },
  }),
  makeServer({
    metadata: { name: "test-server-05", namespace: "default" },
    spec: { templateRef: { name: "palworld-default" }, suspend: true },
    status: {
      phase: "Suspended",
      idle: { asleep: true, asleepSince: "2026-09-01T00:00:00Z", reason: "No players for 24 hours" },
      agent: { playersOnline: 0, playersMax: 32, lastHeartbeat: "2026-09-01T00:00:00Z" },
      startedAt: "2026-08-15T18:30:00Z",
    },
  }),
];

// ============================================================================
// Cluster Nodes: 3 nodes with varying resource usage
// ============================================================================

export const screenshotNodes = [
  {
    name: "node-01",
    roles: ["control-plane", "worker"],
    status: "Ready" as const,
    startedAt: "2026-08-01T00:00:00Z",
    cpu: { used: 3.2, capacity: 8 },
    memory: { used: 6_400_000_000, capacity: 16_000_000_000 },
    pods: { used: 28, capacity: 110 },
  },
  {
    name: "node-02",
    roles: ["worker"],
    status: "Ready" as const,
    startedAt: "2026-08-10T12:00:00Z",
    cpu: { used: 5.8, capacity: 8 },
    memory: { used: 10_200_000_000, capacity: 16_000_000_000 },
    pods: { used: 35, capacity: 110 },
  },
  {
    name: "node-03",
    roles: ["worker"],
    status: "Ready" as const,
    startedAt: "2026-07-20T08:15:00Z",
    cpu: { used: 1.1, capacity: 8 },
    memory: { used: 3_600_000_000, capacity: 16_000_000_000 },
    pods: { used: 12, capacity: 110 },
  },
];

export function screenshotClusterView(): ClusterView {
  return makeClusterView({
    name: "gameplane-demo",
    version: "v1.31.0",
    ready: 3,
    total: 3,
    nodes: screenshotNodes,
  });
}

// ============================================================================
// Kubernetes Events: realistic lifecycle events for test-server-04 (Failed)
// ============================================================================

export const screenshotEvents: ServerEvent[] = [
  {
    id: "evt-001",
    time: "2026-09-02T15:35:00Z",
    type: "Normal",
    reason: "Scheduled",
    message: 'Successfully assigned gameplane-demo/test-server-04 to node-01',
    source: "default-scheduler",
    object: "test-server-04",
    count: 1,
  },
  {
    id: "evt-002",
    time: "2026-09-02T15:36:15Z",
    type: "Normal",
    reason: "Pulling",
    message: 'Pulling image "ghcr.io/valgulnecron/gameplane/rust:2026.01"',
    source: "kubelet",
    object: "test-server-04",
    count: 1,
  },
  {
    id: "evt-003",
    time: "2026-09-02T15:37:30Z",
    type: "Normal",
    reason: "Pulled",
    message: 'Successfully pulled image "ghcr.io/valgulnecron/gameplane/rust:2026.01" in 1m15s',
    source: "kubelet",
    object: "test-server-04",
    count: 1,
  },
  {
    id: "evt-004",
    time: "2026-09-02T15:37:45Z",
    type: "Normal",
    reason: "Created",
    message: 'Created container rust-server',
    source: "kubelet",
    object: "test-server-04",
    count: 1,
  },
  {
    id: "evt-005",
    time: "2026-09-02T15:37:50Z",
    type: "Normal",
    reason: "Started",
    message: 'Started container rust-server',
    source: "kubelet",
    object: "test-server-04",
    count: 1,
  },
  {
    id: "evt-006",
    time: "2026-09-02T15:40:22Z",
    type: "Warning",
    reason: "ImagePullBackOff",
    message: 'Back-off pulling image "ghcr.io/valgulnecron/gameplane/rust:2026.01"',
    source: "kubelet",
    object: "test-server-04",
    count: 3,
  },
  {
    id: "evt-007",
    time: "2026-09-02T15:41:45Z",
    type: "Warning",
    reason: "CrashLoopBackOff",
    message: 'Back-off restarting failed container rust-server',
    source: "kubelet",
    object: "test-server-04",
    count: 12,
  },
];

// ============================================================================
// Audit Events: 15 diverse audit events across actors and methods
// ============================================================================

export const screenshotAuditEvents: AuditEvent[] = [
  makeAudit({
    id: 101,
    ts: "2026-09-02T14:22:15Z",
    actor: "test-user-01",
    method: "POST",
    path: "/api/v1/servers",
    target: "test-server-01",
    status: 201,
    ip: "<internal>",
  }),
  makeAudit({
    id: 102,
    ts: "2026-09-02T14:25:03Z",
    actor: "admin-demo",
    method: "PUT",
    path: "/api/v1/servers/test-server-01",
    target: "test-server-01",
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 103,
    ts: "2026-09-02T14:31:42Z",
    actor: "operator-01",
    method: "POST",
    path: "/api/v1/servers/test-server-01:start",
    target: "test-server-01",
    status: 202,
    ip: "<internal>",
  }),
  makeAudit({
    id: 104,
    ts: "2026-09-02T14:35:18Z",
    actor: "test-user-01",
    method: "GET",
    path: "/api/v1/servers/test-server-01",
    target: "test-server-01",
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 105,
    ts: "2026-09-02T14:42:09Z",
    actor: "admin-demo",
    method: "POST",
    path: "/api/v1/backups",
    target: "test-server-01",
    status: 201,
    ip: "<internal>",
  }),
  makeAudit({
    id: 106,
    ts: "2026-09-02T15:01:33Z",
    actor: "operator-01",
    method: "POST",
    path: "/api/v1/servers/test-server-02:restart",
    target: "test-server-02",
    status: 202,
    ip: "<internal>",
  }),
  makeAudit({
    id: 107,
    ts: "2026-09-02T15:05:21Z",
    actor: "test-user-01",
    method: "DELETE",
    path: "/api/v1/backups/test-server-01-2026-05-07",
    target: "test-server-01-2026-05-07",
    status: 204,
    ip: "<internal>",
  }),
  makeAudit({
    id: 108,
    ts: "2026-09-02T15:12:44Z",
    actor: "admin-demo",
    method: "PATCH",
    path: "/api/v1/users/2",
    target: "operator-01",
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 109,
    ts: "2026-09-02T15:18:07Z",
    actor: "admin-demo",
    method: "POST",
    path: "/api/v1/admin/config/backups",
    target: undefined,
    status: 204,
    ip: "<internal>",
  }),
  makeAudit({
    id: 110,
    ts: "2026-09-02T15:24:56Z",
    actor: "operator-01",
    method: "GET",
    path: "/api/v1/servers",
    target: undefined,
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 111,
    ts: "2026-09-02T15:31:28Z",
    actor: "test-user-01",
    method: "POST",
    path: "/api/v1/servers/test-server-01/players/kick",
    target: "Player-01",
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 112,
    ts: "2026-09-02T15:38:15Z",
    actor: "admin-demo",
    method: "POST",
    path: "/api/v1/servers",
    target: "test-server-03",
    status: 201,
    ip: "<internal>",
  }),
  makeAudit({
    id: 113,
    ts: "2026-09-02T15:40:02Z",
    actor: "operator-01",
    method: "PUT",
    path: "/api/v1/schedules/test-server-01-daily",
    target: "test-server-01-daily",
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 114,
    ts: "2026-09-02T15:43:19Z",
    actor: "test-user-01",
    method: "GET",
    path: "/api/v1/cluster",
    target: undefined,
    status: 200,
    ip: "<internal>",
  }),
  makeAudit({
    id: 115,
    ts: "2026-09-02T15:45:33Z",
    actor: "admin-demo",
    method: "POST",
    path: "/api/v1/admin/config/auth",
    target: undefined,
    status: 204,
    ip: "<internal>",
  }),
];

// ============================================================================
// Users: diverse roles and demo accounts
// ============================================================================

export const screenshotUsers: User[] = [
  makeUser({
    id: 1,
    username: "admin-demo",
    displayName: "Demo Admin",
    email: "admin@gameplane-demo.local",
    role: "admin",
  }),
  makeUser({
    id: 2,
    username: "operator-01",
    displayName: "Server Operator",
    email: "operator@gameplane-demo.local",
    role: "operator",
  }),
  makeUser({
    id: 3,
    username: "viewer-01",
    displayName: "Demo Viewer",
    email: "viewer@gameplane-demo.local",
    role: "viewer",
  }),
  makeUser({
    id: 4,
    username: "test-user-01",
    displayName: "Test User",
    email: "test@gameplane-demo.local",
    role: "operator",
  }),
];

// ============================================================================
// Schedules and Restores
// ============================================================================

export const screenshotSchedules = [
  makeSchedule({
    metadata: { name: "test-server-01-daily", namespace: "default" },
    spec: {
      serverRef: { name: "test-server-01" },
      schedule: "0 3 * * *",
      retention: { keepLast: 7 },
    },
  }),
  makeSchedule({
    metadata: { name: "test-server-02-weekly", namespace: "default" },
    spec: {
      serverRef: { name: "test-server-02" },
      schedule: "0 2 * * 0",
      retention: { keepLast: 4 },
    },
  }),
];

export const screenshotRestores = [
  makeRestore({
    metadata: { name: "restore-test-server-01-1", namespace: "default" },
    spec: {
      backupRef: { name: "test-server-01-2026-05-07" },
      serverRef: { name: "test-server-01" },
    },
  }),
];

// ============================================================================
// Config / Settings
// ============================================================================

export function screenshotConfig(): AllConfig {
  return makeConfig({
    general: {
      instanceName: "My Gameplane Cluster",
      externalURL: "https://gameplane-demo.local",
      defaultNamespace: "default",
    },
  });
}

// ============================================================================
// Game Server Log Lines (~30 lines, Minecraft-style format)
// ============================================================================

export const screenshotLogLines = [
  "[12:00:01] [Server thread/INFO]: Starting minecraft server version 1.21",
  "[12:00:02] [Server thread/INFO]: Loading properties",
  "[12:00:03] [Server thread/INFO]: Default game type: SURVIVAL",
  "[12:00:04] [Server thread/INFO]: Generating keypair",
  "[12:00:05] [Server thread/INFO]: Starting Minecraft server on 0.0.0.0:25565",
  "[12:00:06] [Server thread/INFO]: Using default channel type",
  "[12:00:07] [Worker-Main-1/INFO]: Preparing level \"world\"",
  "[12:00:08] [Worker-Main-1/INFO]: Preparing spawn area: 0%",
  "[12:00:09] [Worker-Main-1/INFO]: Preparing spawn area: 50%",
  "[12:00:10] [Worker-Main-1/INFO]: Preparing spawn area: 100%",
  "[12:00:11] [Server thread/INFO]: Done (2.534s)! For help, type \"help\"",
  "[12:00:12] [Server thread/INFO]: All players logged in without incident",
  "[12:05:15] [Server thread/INFO]: Player-01 joined the game",
  "[12:05:16] [Server thread/INFO]: Player-01 logged in with entity id 123",
  "[12:06:22] [Server thread/INFO]: Player-02 joined the game",
  "[12:06:23] [Server thread/INFO]: Player-02 logged in with entity id 124",
  "[12:15:30] [Server thread/INFO]: Player-03 joined the game",
  "[12:15:31] [Server thread/INFO]: Player-03 logged in with entity id 125",
  "[12:30:45] [Server thread/WARN]: Memory usage high: 78% (7.8 GB / 10 GB)",
  "[12:45:20] [Server thread/INFO]: Player-01 left the game",
  "[12:45:21] [Server thread/INFO]: Saving and pausing game...",
  "[12:45:22] [Server thread/INFO]: Saving chunks for level 'minecraft:overworld'",
  "[12:45:23] [Server thread/INFO]: Saved the game",
  "[12:45:24] [Server thread/INFO]: Resuming game",
  "[13:00:00] [Server thread/INFO]: CPU usage: 45% | Memory: 68% | Disk: 12%",
  "[13:15:10] [Server thread/INFO]: Automatic save triggered",
  "[13:15:11] [Server thread/INFO]: Saving chunks for level 'minecraft:overworld'",
  "[13:15:12] [Server thread/INFO]: Saved the game",
  "[13:30:05] [Server thread/INFO]: Player-02 left the game",
  "[14:00:00] [Server thread/INFO]: Server running normally with 2 players online",
];

// ============================================================================
// Console Output Lines for RCON/PTY Console WebSocket demonstrations
// ============================================================================

export const screenshotConsoleOutput = [
  "Starting server initialization...",
  "Loading configuration files",
  "[INFO] Server version: Minecraft 1.21",
  "[INFO] Loading world 'world'",
  "[INFO] Preparing spawn area: 0%",
  "[INFO] Preparing spawn area: 50%",
  "[INFO] Preparing spawn area: 100%",
  "[INFO] Done! Server is ready for connections",
  "[WARN] No players connected",
  "[INFO] Player-01 joined the game",
  "[INFO] Player-02 joined the game",
  "[INFO] Running auto-save...",
  "[DEBUG] Saved world in 2.34 seconds",
  "[INFO] Player-03 joined the game",
  "[WARN] High memory usage: 78%",
  "[INFO] Player-01 executed: /say Hello everyone!",
  "[INFO] 3 players online, 0 players idle",
];

// ============================================================================
// Installed Mods for test-server-02 (Valheim)
// ============================================================================

export const screenshotInstalledMods: InstalledMod[] = [
  {
    name: "ValheimPlus.dll",
    size: 1_482_240,
    modTime: "2026-08-30T19:12:00Z",
    meta: {
      provider: "thunderstore",
      projectId: "Grantapher-ValheimPlus",
      projectName: "ValheimPlus",
      versionNumber: "0.9.16.1",
      installedAt: "2026-08-30T19:12:00Z",
    },
  },
  {
    name: "EquipmentAndQuickSlots.dll",
    size: 212_992,
    modTime: "2026-08-30T19:14:00Z",
    meta: {
      provider: "thunderstore",
      projectId: "RandyKnapp-EquipmentAndQuickSlots",
      projectName: "EquipmentAndQuickSlots",
      versionNumber: "2.1.15",
      installedAt: "2026-08-30T19:14:00Z",
    },
  },
  {
    name: "PlantEverything.dll",
    size: 356_352,
    modTime: "2026-09-01T08:05:00Z",
    meta: {
      provider: "thunderstore",
      projectId: "Advize-PlantEverything",
      projectName: "PlantEverything",
      versionNumber: "1.18.3",
      installedAt: "2026-09-01T08:05:00Z",
    },
  },
];

// ============================================================================
// Thunderstore Registry Projects for Valheim mod browser
// ============================================================================

export const screenshotRegistryProjects: RegistryProject[] = [
  {
    id: "denikson-BepInExPack_Valheim",
    slug: "BepInExPack_Valheim",
    title: "BepInExPack_Valheim",
    description: "BepInEx pack for Valheim with mod manager integration",
    author: "denikson",
    downloads: 14_200_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/denikson/BepInExPack_Valheim/",
    provider: "thunderstore",
  },
  {
    id: "ValheimModding-Jotunn",
    slug: "Jotunn",
    title: "Jotunn",
    description: "A library that provides intuitive and modular systems for modding Valheim",
    author: "ValheimModding",
    downloads: 12_800_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/ValheimModding/Jotunn/",
    provider: "thunderstore",
  },
  {
    id: "ValheimModding-HookGenPatcher",
    slug: "HookGenPatcher",
    title: "HookGenPatcher",
    description: "Automatic Unity Networking patching for game object systems",
    author: "ValheimModding",
    downloads: 10_500_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/ValheimModding/HookGenPatcher/",
    provider: "thunderstore",
  },
  {
    id: "Grantapher-ValheimPlus",
    slug: "ValheimPlus",
    title: "ValheimPlus",
    description: "Comprehensive quality-of-life mod with farming, building, and exploration enhancements",
    author: "Grantapher",
    downloads: 9_800_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/Grantapher/ValheimPlus/",
    provider: "thunderstore",
  },
  {
    id: "RandyKnapp-EpicLoot",
    slug: "EpicLoot",
    title: "EpicLoot",
    description: "Adds an advanced loot system with rare item drops and crafting",
    author: "RandyKnapp",
    downloads: 8_200_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/RandyKnapp/EpicLoot/",
    provider: "thunderstore",
  },
  {
    id: "RandyKnapp-EquipmentAndQuickSlots",
    slug: "EquipmentAndQuickSlots",
    title: "EquipmentAndQuickSlots",
    description: "Adds equipment slots and quick-slot hotbar for better inventory management",
    author: "RandyKnapp",
    downloads: 7_100_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/RandyKnapp/EquipmentAndQuickSlots/",
    provider: "thunderstore",
  },
  {
    id: "Advize-PlantEverything",
    slug: "PlantEverything",
    title: "PlantEverything",
    description: "Allows planting of all Valheim plants for better farming",
    author: "Advize",
    downloads: 6_400_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/Advize/PlantEverything/",
    provider: "thunderstore",
  },
  {
    id: "Azumatt-AzuCraftyBoxes",
    slug: "AzuCraftyBoxes",
    title: "AzuCraftyBoxes",
    description: "Adds convenient crafting interface boxes that can be placed anywhere",
    author: "Azumatt",
    downloads: 5_600_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/Azumatt/AzuCraftyBoxes/",
    provider: "thunderstore",
  },
  {
    id: "Smoothbrain-Sailing",
    slug: "Sailing",
    title: "Sailing",
    description: "Overhauls sailing mechanics with smoother controls and navigation features",
    author: "Smoothbrain",
    downloads: 4_300_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/Smoothbrain/Sailing/",
    provider: "thunderstore",
  },
  {
    id: "Smoothbrain-Farming",
    slug: "Farming",
    title: "Farming",
    description: "Expands farming with new crops and enhanced growth mechanics",
    author: "Smoothbrain",
    downloads: 3_800_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/Smoothbrain/Farming/",
    provider: "thunderstore",
  },
  {
    id: "ishid4-BetterArchery",
    slug: "BetterArchery",
    title: "BetterArchery",
    description: "Improves bow mechanics with crosshair and better damage calculations",
    author: "ishid4",
    downloads: 2_200_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/ishid4/BetterArchery/",
    provider: "thunderstore",
  },
  {
    id: "blaxxun-Groups",
    slug: "Groups",
    title: "Groups",
    description: "Adds player groups and cross-server communication features",
    author: "blaxxun",
    downloads: 1_500_000,
    pageUrl: "https://thunderstore.io/c/valheim/p/blaxxun/Groups/",
    provider: "thunderstore",
  },
];

// ============================================================================
// Helper for tests/e2e to conditionally swap handler sets
// ============================================================================

export function getScreenshotData() {
  return {
    templates: screenshotTemplates,
    servers: screenshotServers,
    nodes: screenshotNodes,
    clusterView: screenshotClusterView,
    events: screenshotEvents,
    auditEvents: screenshotAuditEvents,
    users: screenshotUsers,
    schedules: screenshotSchedules,
    restores: screenshotRestores,
    config: screenshotConfig,
    logLines: screenshotLogLines,
    installedMods: screenshotInstalledMods,
    registryProjects: screenshotRegistryProjects,
  };
}
