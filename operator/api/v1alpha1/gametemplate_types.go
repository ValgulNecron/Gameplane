package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GameTemplateSpec defines a reusable blueprint for a game server.
// Users instantiate a GameTemplate by creating a GameServer that
// references it.
// +kubebuilder:validation:XValidation:rule="!has(self.consoleMode) || self.consoleMode != 'rcon' || (has(self.rcon) && (!has(self.rcon.protocol) || self.rcon.protocol != 'none'))",message="consoleMode 'rcon' requires spec.rcon with a protocol other than 'none'"
type GameTemplateSpec struct {
	// DisplayName is a human-friendly label shown in the dashboard.
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// Game is the canonical game identifier (e.g. "minecraft-java",
	// "valheim", "factorio"). Used as a grouping key in the UI catalog.
	// +kubebuilder:validation:MinLength=1
	Game string `json:"game"`

	// Version is the template revision (e.g. "1.0.0"). Bump when changing
	// defaults in ways that existing GameServers should opt into.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`

	// Icon is an optional URL or data URI shown in the catalog.
	// +optional
	Icon string `json:"icon,omitempty"`

	// AccentColor is an optional brand color (CSS hex, e.g. "#3b82f6")
	// the dashboard uses to tint this game's icon and accents. When
	// empty the dashboard falls back to a neutral default. This replaces
	// the previously hardcoded per-game color palette in the web app, so
	// new games carry their own color without a frontend change.
	// +kubebuilder:validation:Pattern=`^#[0-9a-fA-F]{6}$`
	// +optional
	AccentColor string `json:"accentColor,omitempty"`

	// Description is free-form markdown describing the template.
	// +optional
	Description string `json:"description,omitempty"`

	// Image is the default container image (e.g.
	// "itzg/minecraft-server:2025.1.0"). Used as the fallback when the
	// server selects no version (see Versions) and sets no spec.image
	// override. Can be overridden by GameServer.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Versions is an optional catalog of selectable game versions surfaced
	// in the Create Server wizard. Each entry maps a user choice to a
	// concrete image (and optional per-version env / mod loader). When empty
	// there is no version choice: a server runs spec.image (override) or this
	// template's Image, exactly as before. At most one entry should set
	// default=true; otherwise the first entry is the wizard's default.
	// +kubebuilder:validation:MaxItems=64
	// +optional
	Versions []GameVersion `json:"versions,omitempty"`

	// Command / Args override the container image entrypoint when set.
	// +optional
	Command []string `json:"command,omitempty"`
	// +optional
	Args []string `json:"args,omitempty"`

	// Env is the default environment for the game container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Ports declares exposed ports on the game container.
	// +optional
	Ports []GamePort `json:"ports,omitempty"`

	// Storage describes the default persistent storage layout.
	// +optional
	Storage GameStorageSpec `json:"storage,omitempty"`

	// Resources are the default compute resources for the game container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// RCON declares the remote-console protocol used by this game, if any.
	// +optional
	RCON *RCONSpec `json:"rcon,omitempty"`

	// ConsoleMode controls how the dashboard's Console tab attaches to
	// the running game.
	//
	//   - "rcon": send line-based RCON commands (default when RCON is set
	//     and its protocol is not "none").
	//   - "pty":  attach to the game container's stdin/stdout via the
	//     Kubernetes pod-attach API. Requires the container to be started
	//     with tty=true and stdin=true (the operator sets these). Switching
	//     to pty after the pod exists requires a pod recreate.
	//   - "none": disable the Console tab for this game.
	//
	// When unset, the operator defaults to "rcon" if RCON.Protocol is set
	// to anything other than "none", and "none" otherwise.
	// +kubebuilder:validation:Enum=rcon;pty;none
	// +optional
	ConsoleMode string `json:"consoleMode,omitempty"`

	// LogPath is the file holding the game's primary log, as seen from
	// inside the pod (e.g. "/data/logs/latest.log"). It must live under
	// the shared data volume (Storage.MountPath) so the agent sidecar
	// can tail it for the dashboard's Logs tab. Leave empty for games
	// that only log to stdout — the Logs tab is then unavailable.
	// +optional
	LogPath string `json:"logPath,omitempty"`

	// Probes are the default readiness/liveness/startup probes for the
	// game container. Operator supplies sane defaults when unset.
	// +optional
	Probes *GameProbesSpec `json:"probes,omitempty"`

	// ConfigSchema declares user-tunable fields surfaced in the Create
	// Server wizard. The operator resolves GameServer.spec.config
	// against this schema (applying defaults, validating types/enums)
	// and sets each resolved value as an env var on the game container.
	// Fields with target=file are consumed by ConfigFiles templates
	// instead of becoming env vars.
	// +optional
	ConfigSchema []ConfigField `json:"configSchema,omitempty"`

	// ConfigFiles declares files the operator renders from the resolved
	// config values and places under Storage.MountPath before the game
	// starts. Rendered contents are stored in an owned `<server>-files`
	// Secret (they may embed password values) and copied onto the data
	// volume by an init container on every pod start — manual edits to
	// these paths via the Files tab are overwritten on restart.
	// +kubebuilder:validation:MaxItems=32
	// +optional
	ConfigFiles []ConfigFile `json:"configFiles,omitempty"`

	// Agent tunes the sidecar deployed alongside the game container.
	// +optional
	Agent *AgentSpec `json:"agent,omitempty"`

	// Capabilities declares the game-specific console commands behind
	// agent features (player moderation, backup quiesce). The operator
	// serializes this onto the agent sidecar, which interprets it at
	// runtime — modules add full feature support without agent code
	// changes. All commands run over the template's RCON connection,
	// so they require rcon.protocol != none.
	// +optional
	Capabilities *CapabilitiesSpec `json:"capabilities,omitempty"`
}

// GameVersion is one selectable entry in a template's version catalog.
// Selecting it (via GameServer.spec.version) pins the container image,
// appends this entry's env, and — when Loader names a capabilities.mods
// loader — provisions and mounts that loader's per-(version+loader) mod
// volume.
type GameVersion struct {
	// ID is the stable selector stored in GameServer.spec.version. It is
	// also folded into the per-version mod volume/PVC names, so keep it short
	// and DNS-ish (dots and hyphens allowed; sanitized to a DNS label for
	// volume names). E.g. "1.21.4-paper", "tmodloader-latest".
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=40
	ID string `json:"id"`

	// DisplayName labels the entry in the version picker, e.g.
	// "1.21.4 (Paper)".
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	DisplayName string `json:"displayName"`

	// Image is the full container reference for this version. For
	// env-versioned games (e.g. itzg/minecraft-server, which picks software
	// via TYPE/VERSION env) this is usually one pinned image shared across
	// entries, differentiated only by Loader/Env. For tag-versioned games
	// (e.g. Terraria) each entry pins a distinct tag.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// Loader is the mod-loader / server-type key (e.g. "paper", "forge",
	// "fabric", "vanilla", "bepinex", "tmodloader"). It keys into
	// capabilities.mods.loaders to select this combo's mod volume. Empty
	// means this version has no loader dimension (mods, if any, live on the
	// shared data volume).
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=40
	// +optional
	Loader string `json:"loader,omitempty"`

	// Env is appended when this version is selected (e.g. TYPE=PAPER,
	// VERSION=1.21.4 for itzg). It is applied after the template's Env and
	// before GameServer.spec.env, so an explicit user override still wins.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Default marks the wizard's pre-selected entry. At most one entry should
	// set it; if none (or several) do, the first entry is the default.
	// +optional
	Default bool `json:"default,omitempty"`

	// GameVersion is the upstream game-version token the dashboard passes to
	// an external mod registry to filter results to this version (e.g.
	// Modrinth's "game_versions" facet), e.g. "1.21.4". It is distinct from
	// ID (a Kestrel selector like "1.21.4-paper" that's too lossy to parse
	// reliably). Optional; when unset the registry search sends no version
	// facet and returns mods for all versions. Ignored by registries with no
	// version dimension (e.g. Thunderstore) and when the template declares no
	// mods.registry.
	// +kubebuilder:validation:MaxLength=32
	// +optional
	GameVersion string `json:"gameVersion,omitempty"`
}

// CapabilitiesSpec declares the per-game command surface the agent
// interprets.
type CapabilitiesSpec struct {
	// Players configures the moderation actions on the Players tab.
	// +optional
	Players *PlayerActionsSpec `json:"players,omitempty"`

	// Quiesce configures how in-flight game state is flushed to disk
	// before a backup snapshot (and resumed afterwards).
	// +optional
	Quiesce *QuiesceSpec `json:"quiesce,omitempty"`

	// Actions declares named operator actions surfaced as buttons on the
	// server detail page. Each runs a templated console command over the
	// template's RCON connection, so they require rcon.protocol != none.
	// +kubebuilder:validation:MaxItems=32
	// +optional
	Actions []ServerActionSpec `json:"actions,omitempty"`

	// Status declares game-specific live metrics shown on the Overview
	// tab, each read via an RCON command parsed by a named-group regex.
	// Like actions, they require rcon.protocol != none.
	// +optional
	Status *StatusSpec `json:"status,omitempty"`

	// Mods declares how the dashboard manages this game's mods/plugins.
	// The dashboard is generic — it lists, installs, and removes mods by
	// calling the agent; this block tells the agent where mods live and
	// how installs are permitted. Mods are files under a directory on the
	// game's data volume, so this needs no RCON.
	// +optional
	Mods *ModsSpec `json:"mods,omitempty"`
}

// ModsSpec declares the mod/plugin directory and install policy the
// agent enforces. A template uses EITHER the single shared Path (legacy,
// one mods dir on the data volume) OR Loaders (a per-(version+loader) mod
// volume selected by the active GameVersion.loader). Listing and removal
// operate on the resolved directory; Install adds a mod by downloading it
// there.
// +kubebuilder:validation:XValidation:rule="has(self.path) || (has(self.loaders) && size(self.loaders) > 0)",message="mods requires either path or at least one loaders entry"
type ModsSpec struct {
	// Path is the single shared mods directory, relative to
	// storage.mountPath (e.g. "mods" for Forge/Fabric, "plugins" for
	// Bukkit/Paper). Used when Loaders is empty. Absolute paths and ".."
	// segments are rejected.
	// +kubebuilder:validation:MaxLength=256
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('/') && !self.contains('..')",message="path must be relative to the data mount and must not contain '..'"
	// +optional
	Path string `json:"path,omitempty"`

	// Loaders maps a loader id (from GameVersion.loader) to its mods
	// directory. When the active version's loader has an entry here, the
	// operator provisions a per-(version+loader) PVC, mounts it at
	// storage.mountPath/<path> on the game and agent containers, and points
	// the agent's mod manager at it — so each version+loader keeps its own
	// mod set. When empty, Path is the single shared mods dir (legacy).
	// MaxProperties bounds the per-entry CEL validation cost (the apiserver
	// rejects the CRD otherwise).
	// +kubebuilder:validation:MaxProperties=16
	// +optional
	Loaders map[string]ModLoaderSpec `json:"loaders,omitempty"`

	// Extensions optionally restricts which files in Path are treated as
	// mods (e.g. [".jar"]). Empty lists every file. Used with Path; per-
	// loader extensions are set on each ModLoaderSpec instead.
	// +optional
	Extensions []string `json:"extensions,omitempty"`

	// Extract is the resolved per-loader Extract flag (the operator copies
	// it from the active loader). When true the agent unpacks archive mods
	// into per-mod folders. Not normally set by template authors at this
	// level — set it on the loader entry instead.
	// +optional
	Extract bool `json:"extract,omitempty"`

	// Install, when set, lets the dashboard add new mods by downloading
	// them into the resolved mods directory. When unset, only listing and
	// removal are offered. Shared across all loaders.
	// +optional
	Install *ModInstallSpec `json:"install,omitempty"`

	// Registry, when set, lets the dashboard browse and search an external
	// mod registry for this game (in addition to install-by-URL). Kestrel
	// ships the provider engines and a generic browse UI; the module selects
	// a provider here and the agent's Install downloads the chosen file —
	// so the registry's CDN must also be in Install.AllowedHosts. Omit for
	// URL-only games (e.g. tModLoader, whose mods live on Steam Workshop).
	// +optional
	Registry *ModRegistrySpec `json:"registry,omitempty"`
}

// ModRegistrySpec lists the built-in external mod registries the dashboard
// can browse for this game. The engines are generic Kestrel code; this
// block is the per-game configuration that drives them. Loader filtering
// reuses the active version's loader id verbatim and version filtering uses
// the active GameVersion.gameVersion token — so no mappings live here.
type ModRegistrySpec struct {
	// Providers is the ordered list of registries to offer. The dashboard
	// shows a provider switch when there's more than one; the first is the
	// default. A provider whose engine needs unmet config (e.g. a
	// CurseForge API key) is hidden until configured.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=8
	Providers []ModProvider `json:"providers"`
}

// ModProvider configures one registry engine for a game.
type ModProvider struct {
	// Provider names the built-in registry engine: "modrinth" (Minecraft
	// mods/plugins, keyless), "thunderstore" (BepInEx games, keyless,
	// per-community), "curseforge" (Minecraft mods/modpacks, needs an API
	// key), or "hangar" (PaperMC plugins, keyless).
	// +kubebuilder:validation:Enum=modrinth;thunderstore;curseforge;hangar
	Provider string `json:"provider"`

	// Community is the Thunderstore community slug whose package index to
	// search, e.g. "valheim". Required by the thunderstore provider;
	// ignored by others.
	// +kubebuilder:validation:MaxLength=64
	// +optional
	Community string `json:"community,omitempty"`

	// Modpacks, when set, surfaces a Modpacks browser for this provider and
	// declares how installing one is applied. A modpack is selected as a
	// whole (not added to the mods dir like a single mod), so install
	// either pins it via a game-image env (RefEnv) or resolves and installs
	// its dependency mods. Omit for providers without modpacks.
	// +optional
	Modpacks *ModpackSpec `json:"modpacks,omitempty"`
}

// ModpackSpec declares how a chosen modpack is installed for a game.
type ModpackSpec struct {
	// RefEnv, when set, is the game-image env the operator points at the
	// chosen modpack reference (slug/URL) — e.g. "MODRINTH_MODPACK" for the
	// itzg image. Installing then patches GameServer.spec.env and restarts;
	// one modpack is active per server. When empty, installing instead
	// resolves and installs the modpack's dependency mods (e.g. a
	// Thunderstore/BepInEx pack).
	// +kubebuilder:validation:MaxLength=64
	// +optional
	RefEnv string `json:"refEnv,omitempty"`

	// Env are additional fixed env applied alongside RefEnv when a modpack
	// is active, e.g. {TYPE: MODRINTH} to switch the itzg image into
	// Modrinth-modpack mode. Bounded to keep the CRD small.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// ModLoaderSpec is the mods directory for one loader / server-type,
// selected by the active GameVersion.loader. The operator provisions a
// per-(version+loader) PVC and mounts it at storage.mountPath/<Path> on
// both the game container (where the image reads mods) and the agent.
type ModLoaderSpec struct {
	// Path is this loader's mods/plugins directory, relative to
	// storage.mountPath (e.g. "plugins", "mods", "bepinex/plugins",
	// "ModPacks"). Absolute paths and ".." segments are rejected. MaxLength
	// is kept small to bound the per-entry CEL validation cost (this rule
	// lives inside the loaders map, so its cost is multiplied per entry).
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('/') && !self.contains('..')",message="path must be relative to the data mount and must not contain '..'"
	Path string `json:"path"`

	// DisplayName labels this volume in the Mods tab, e.g. "Plugins (Paper)".
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Extensions optionally restricts which files are treated as mods for
	// this loader (e.g. [".jar"], [".dll"]). Empty lists every file.
	// +optional
	Extensions []string `json:"extensions,omitempty"`

	// Extract, when true, tells the agent to treat downloaded mods as
	// archives (e.g. Thunderstore .zip): each install unpacks into its own
	// folder under the mods dir so the loader (e.g. BepInEx, which scans
	// recursively) finds the contained files. Listing/removal then operate
	// on those per-mod folders. Use for loaders distributed as archives.
	// +optional
	Extract bool `json:"extract,omitempty"`
}

// ModInstallSpec configures installing a mod by downloading it from a
// URL into the mods directory.
type ModInstallSpec struct {
	// AllowedHosts is the allowlist of hosts the agent may download from:
	// an exact hostname ("cdn.modrinth.com") or a leading-dot suffix
	// (".modrinth.com") matching that host and its subdomains. Downloads
	// from any other host — and any that resolve to a private, loopback,
	// or link-local address — are refused (SSRF guard).
	// +kubebuilder:validation:MinItems=1
	AllowedHosts []string `json:"allowedHosts"`

	// MaxSizeMB caps the download size in mebibytes. Defaults to 256.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4096
	// +optional
	MaxSizeMB int32 `json:"maxSizeMB,omitempty"`
}

// ServerActionSpec declares a named operator action surfaced as a button
// on the server detail page. Running it renders Command (a Go
// text/template) with the collected parameters and sends the result over
// the template's RCON connection.
type ServerActionSpec struct {
	// ID is a stable identifier, unique within the template's actions.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=63
	ID string `json:"id"`

	// DisplayName is the button label.
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// Description explains the action; shown in the confirmation dialog.
	// +optional
	Description string `json:"description,omitempty"`

	// Icon is an optional lucide-react icon name (e.g. "megaphone").
	// +optional
	Icon string `json:"icon,omitempty"`

	// Command is a Go text/template rendered with .Params (each declared
	// parameter name mapped to its resolved value) and sent over RCON,
	// e.g. "say {{.Params.message}}". Parameter values are sanitized for
	// console-injection before rendering.
	// +kubebuilder:validation:MinLength=1
	Command string `json:"command"`

	// Params declares inputs collected from the user before running.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Params []ActionParamSpec `json:"params,omitempty"`

	// Confirm, when true, makes the dashboard require explicit
	// confirmation before running the action.
	// +optional
	Confirm bool `json:"confirm,omitempty"`

	// Danger marks a destructive action so the dashboard styles it
	// distinctly (e.g. a red button).
	// +optional
	Danger bool `json:"danger,omitempty"`
}

// ActionParamSpec is a single input collected before running an action.
type ActionParamSpec struct {
	// Name is the parameter identifier referenced in the command
	// template as {{.Params.<name>}}.
	// +kubebuilder:validation:Pattern=`^[a-zA-Z_][a-zA-Z0-9_]*$`
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// DisplayName labels the input in the UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description explains the input.
	// +optional
	Description string `json:"description,omitempty"`

	// Type controls the input widget and validation.
	// +kubebuilder:validation:Enum=string;int;bool;enum
	// +kubebuilder:default=string
	Type string `json:"type"`

	// Default is the pre-filled value (as a string).
	// +optional
	Default string `json:"default,omitempty"`

	// Enum restricts valid values when Type=enum.
	// +optional
	Enum []string `json:"enum,omitempty"`

	// Required, when true, blocks running until the input is set.
	// +optional
	Required bool `json:"required,omitempty"`
}

// StatusSpec declares game-specific live metrics for the Overview tab.
type StatusSpec struct {
	// Metrics are the per-game readouts. Each runs an RCON command and
	// extracts a value via a named-group regex.
	// +kubebuilder:validation:MaxItems=16
	// +optional
	Metrics []StatusMetricSpec `json:"metrics,omitempty"`
}

// StatusMetricSpec reads one live metric from an RCON command's output.
type StatusMetricSpec struct {
	// ID is a stable identifier, unique within the template's metrics.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=63
	ID string `json:"id"`

	// DisplayName labels the metric in the UI.
	// +kubebuilder:validation:MinLength=1
	DisplayName string `json:"displayName"`

	// Command is the RCON command whose output is parsed.
	// +kubebuilder:validation:MinLength=1
	Command string `json:"command"`

	// Regex extracts the value via the named group "value", e.g.
	// `TPS: (?P<value>[0-9.]+)`.
	// +kubebuilder:validation:MinLength=1
	Regex string `json:"regex"`

	// Unit is an optional suffix shown after the value (e.g. "ms", "TPS").
	// +optional
	Unit string `json:"unit,omitempty"`
}

// PlayerActionsSpec maps moderation actions to console commands. Each
// command is a Go text/template rendered with .Player and .Reason
// (reason may be empty — guard with {{if .Reason}}). Unset actions are
// reported as unsupported.
type PlayerActionsSpec struct {
	// Kick disconnects a player, e.g.
	// "kick {{.Player}}{{if .Reason}} {{.Reason}}{{end}}".
	// +optional
	Kick string `json:"kick,omitempty"`

	// Ban bans a player.
	// +optional
	Ban string `json:"ban,omitempty"`

	// Unban lifts a ban, e.g. "pardon {{.Player}}".
	// +optional
	Unban string `json:"unban,omitempty"`

	// BanList configures reading the current ban list.
	// +optional
	BanList *BanListSpec `json:"banList,omitempty"`
}

// BanListSpec reads and parses the game's ban list.
type BanListSpec struct {
	// Command prints the ban list (e.g. "banlist players").
	// +kubebuilder:validation:MinLength=1
	Command string `json:"command"`

	// EntryRegex matches one banned player per output line, using the
	// named groups "name" (required), "source" and "reason" (optional).
	// +kubebuilder:validation:MinLength=1
	EntryRegex string `json:"entryRegex"`
}

// QuiesceSpec declares the command sequences that pause and resume
// game writes around a backup.
type QuiesceSpec struct {
	// Quiesce runs before the snapshot, in order (e.g. ["save-off",
	// "save-all flush"]). Any command error triggers a best-effort
	// Unquiesce.
	// +kubebuilder:validation:MinItems=1
	Quiesce []string `json:"quiesce"`

	// Unquiesce runs after the snapshot (e.g. ["save-on"]).
	// +kubebuilder:validation:MinItems=1
	Unquiesce []string `json:"unquiesce"`

	// FailurePattern, when it matches a quiesce command's output
	// (case-insensitive regex), treats the step as failed even though
	// the command itself returned successfully.
	// +optional
	FailurePattern string `json:"failurePattern,omitempty"`
}

// GamePort is a single exposed port.
type GamePort struct {
	// Name is a DNS-label port identifier (e.g. "game", "query", "rcon").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ContainerPort is the port the game listens on inside the pod.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ContainerPort int32 `json:"containerPort"`

	// Protocol is one of TCP, UDP. Defaults to TCP.
	// +kubebuilder:default=TCP
	// +kubebuilder:validation:Enum=TCP;UDP
	// +optional
	Protocol corev1.Protocol `json:"protocol,omitempty"`

	// Advertise controls whether this port is exposed to users via
	// GameServer.Spec.Networking. RCON and query ports are typically
	// not advertised publicly.
	// +kubebuilder:default=true
	// +optional
	Advertise bool `json:"advertise,omitempty"`
}

// GameStorageSpec describes the persistent storage layout for a game.
type GameStorageSpec struct {
	// Size is the default PVC size (e.g. "10Gi").
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClassName, when set, pins the PVC to a specific StorageClass.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// MountPath is where the game persistent volume is mounted inside
	// the game container (e.g. "/data").
	// +kubebuilder:default=/data
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// RCONSpec describes the remote-console protocol used by the game.
type RCONSpec struct {
	// Protocol is the wire protocol. "source" is the Valve/Minecraft RCON
	// protocol; other games use different mechanisms (telnet, HTTP, etc.)
	// and are implemented by module-specific code paths.
	// +kubebuilder:default=source
	// +kubebuilder:validation:Enum=source;telnet;http;none
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Port is the TCP port RCON listens on inside the pod.
	// +optional
	Port int32 `json:"port,omitempty"`

	// PasswordSecretRef references a Secret+key containing the RCON
	// password. If unset, the operator generates a password and stores
	// it in an auto-managed Secret (<gameserver>-rcon, key "password").
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`

	// PasswordEnv is the environment variable the game container reads
	// the RCON password from (e.g. "RCON_PASSWORD" for itzg/minecraft).
	// When set, the operator injects the resolved password into the game
	// container via this env var and mounts the same value for the agent
	// sidecar, so the dashboard console can authenticate. Leave empty for
	// games that take their RCON password some other way.
	// +optional
	PasswordEnv string `json:"passwordEnv,omitempty"`
}

// GameProbesSpec are the default probes for the game container.
type GameProbesSpec struct {
	// +optional
	Readiness *corev1.Probe `json:"readiness,omitempty"`
	// +optional
	Liveness *corev1.Probe `json:"liveness,omitempty"`
	// +optional
	Startup *corev1.Probe `json:"startup,omitempty"`
}

// AgentSpec tunes the Kestrel sidecar that runs alongside the game.
type AgentSpec struct {
	// Image overrides the default agent image. Normally set by the
	// operator to the image matching its own build.
	// +optional
	Image string `json:"image,omitempty"`

	// Resources overrides agent resources.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ConfigField is a single user-tunable setting surfaced in the wizard.
type ConfigField struct {
	// Name is the field identifier (also used as an env var when
	// Target is "env").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// DisplayName is shown in the UI.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Description explains the field to end users.
	// +optional
	Description string `json:"description,omitempty"`

	// Type controls the input widget in the wizard. Values of password
	// fields are stored in a per-GameServer Secret and injected via
	// SecretKeyRef, never inline in the pod spec.
	// +kubebuilder:validation:Enum=string;int;bool;enum;password
	// +kubebuilder:default=string
	Type string `json:"type"`

	// Default is the default value (as a string) rendered in the wizard.
	// +optional
	Default string `json:"default,omitempty"`

	// Enum restricts valid values when Type=enum.
	// +optional
	Enum []string `json:"enum,omitempty"`

	// Required, when true, blocks wizard submission if unset.
	// +optional
	Required bool `json:"required,omitempty"`

	// Target controls where the value is applied. "env" (default) sets
	// an env var on the game container; "file" makes the value available
	// to spec.configFiles templates instead of the environment.
	// +kubebuilder:validation:Enum=env;file
	// +kubebuilder:default=env
	// +optional
	Target string `json:"target,omitempty"`
}

// ConfigFile is a single operator-rendered file on the game's data
// volume.
type ConfigFile struct {
	// Path is where the rendered file lands, relative to
	// Storage.MountPath (e.g. "serverconfig.txt", "cfg/server.cfg").
	// Absolute paths and ".." segments are rejected.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=512
	// +kubebuilder:validation:XValidation:rule="!self.startsWith('/') && !self.contains('..')",message="path must be relative to the data mount and must not contain '..'"
	Path string `json:"path"`

	// Template is a Go text/template rendered with `.Values` (every
	// configSchema field name mapped to its resolved value, "" when an
	// optional field is unset) and `.Server` (`.Name`, `.Namespace`).
	// Rendering uses missingkey=error: referencing a key outside the
	// schema fails the GameServer.
	// +kubebuilder:validation:MinLength=1
	Template string `json:"template"`
}

// SecretKeySelector references a key inside a namespaced Secret. Mirrors
// corev1.SecretKeySelector but narrowed to the fields we support.
type SecretKeySelector struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// GameTemplateStatus is intentionally minimal — templates are mostly
// static configuration. The operator populates ObservedGeneration so
// controllers reading templates can detect spec updates.
type GameTemplateStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// InUseCount is the number of GameServers currently referencing
	// this template.
	// +optional
	InUseCount int32 `json:"inUseCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=gtmpl;gametmpl
// +kubebuilder:printcolumn:name="Game",type=string,JSONPath=`.spec.game`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="In Use",type=integer,JSONPath=`.status.inUseCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:subresource:status

// GameTemplate is a cluster-scoped blueprint for a specific game. A
// GameTemplate is distributed as part of a Kestrel module (OCI artifact)
// and instantiated via GameServer.
type GameTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameTemplateSpec   `json:"spec,omitempty"`
	Status GameTemplateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GameTemplateList is a list of GameTemplates.
type GameTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameTemplate{}, &GameTemplateList{})
}
