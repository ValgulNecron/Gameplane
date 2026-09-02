# Comparison Table Sources

This file documents the sources and verification dates for every claim in the README comparison table. Each product has a section below. Entries are organized by row (dimension a–i) and include the source URL, date checked, and what was verified.

**Last Updated**: 2026-09-02

---

## Gameplane

**License**: GNU Affero General Public License v3.0 or later (AGPL-3.0-or-later)  
**Documentation Root**: https://valgulnecron.github.io/gameplane-website/  
**Repository**: https://github.com/ValgulNecron/Gameplane

<a id="gameplane-row-a"></a>
### Row (a): Deployment/runtime model

**Source ID**: G-a  
**Evidence**: operator/api/v1alpha1/cluster_types.go, operator/api/v1alpha1/gameserver_types.go, CLAUDE.md repo map  
**Checked on**: 2026-09-02  
**What was verified**: Gameplane uses Kubernetes CRDs (GameServer, GameTemplate, Backup, etc.) and controller-runtime operator for reconciliation. Scales from single-node k3s to multi-node clusters.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane

<a id="gameplane-row-b"></a>
### Row (b): Scaling & auto-sleep

**Source ID**: G-b  
**Evidence**: operator/api/v1alpha1/gameserver_types.go (spec.idle), operator/internal/controller/gameserver_controller.go (idle reconciliation), docs/architecture.md, sentinel/README.md  
**Checked on**: 2026-09-02  
**What was verified**: GameServer CRD includes idle.enabled, idle.idleAfter, idle.wakeWindow fields. Sentinel component (optional) listens on advertised ports during idle state. Minecraft and Terraria use protocol parsing (gameproto/) for wake detection; others use packet heuristics.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/operator/api/v1alpha1

<a id="gameplane-row-c"></a>
### Row (c): Inbound connectivity (NAT traversal, relay)

**Source ID**: G-c  
**Evidence**: tunnel/ module (relay supervisor), docs/tunnels.md, charts/gameplane/values.yaml (tunnel.enabled)  
**Checked on**: 2026-09-02  
**What was verified**: Tunnel component (optional) manages frp, Tailscale, playit relay clients. playit mappings are user-managed via playit.gg account. Integration is opt-in via helm values.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/tunnel

<a id="gameplane-row-d"></a>
### Row (d): Backup and restore

**Source ID**: G-d  
**Evidence**: operator/api/v1alpha1/backup_types.go, operator/api/v1alpha1/restore_types.go, docs/architecture.md (backup section)  
**Checked on**: 2026-09-02  
**What was verified**: Backup CRD uses Restic snapshots stored in S3-compatible storage. BackupSchedule CRD supports cron scheduling. Restore CRD handles one-click restore operations via reconciler.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/operator/api/v1alpha1

<a id="gameplane-row-e"></a>
### Row (e): Access control & authentication

**Source ID**: G-e  
**Evidence**: api/internal/auth/ (password.go, oidc.go), api/internal/rbac/rbac.go, docs/security.md, docs/oidc.md  
**Checked on**: 2026-09-02  
**What was verified**: API supports local argon2id password auth and OIDC (Keycloak, Google, GitHub tested). Built-in roles: admin, operator, viewer. Custom roles supported via role CRD and group-based OIDC mapping.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/api/internal/auth

<a id="gameplane-row-f"></a>
### Row (f): Game template distribution

**Source ID**: G-f  
**Evidence**: operator/api/v1alpha1/module_types.go, operator/api/v1alpha1/modulesource_types.go, docs/module-authoring.md (OCI bundle format)  
**Checked on**: 2026-09-02  
**What was verified**: ModuleSource CRD supports git, http, oci, local, upload sources. Modules are OCI bundles with optional cosign signature verification. 16 templates shipped: minecraft-java, minecraft-vanilla, valheim, terraria, rust, palworld, and 10 others.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/modules

<a id="gameplane-row-g"></a>
### Row (g): Multi-tenancy & multi-cluster

**Source ID**: G-g  
**Evidence**: operator/api/v1alpha1/cluster_types.go (Cluster CRD), docs/architecture.md (multi-cluster section), api/internal/handlers/cluster.go (cluster reconciliation), README.md:35  
**Checked on**: 2026-09-02  
**What was verified**: Cluster CRD allows remote cluster registration and monitoring from a central dashboard. WebSocket console/log streaming is scoped to the local control-plane cluster only (not proxied across cluster boundaries).  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/tree/master/operator/api/v1alpha1

<a id="gameplane-row-h"></a>
### Row (h): Licensing

**Source ID**: G-h  
**Evidence**: LICENSE file (repo root), CLAUDE.md (Stack reference section)  
**Checked on**: 2026-09-02  
**What was verified**: Repository is licensed under AGPL-3.0-or-later (Affero GPL v3). All source code and derivatives must be open-source and share improvements.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane/blob/master/LICENSE

<a id="gameplane-row-i"></a>
### Row (i): Target operator scope (self-hosted vs. managed SaaS)

**Source ID**: G-i  
**Evidence**: README.md (project description), docs/install.md (Helm chart installation), docs/architecture.md  
**Checked on**: 2026-09-02  
**What was verified**: Gameplane is self-hosted only. Runs on Kubernetes clusters (k3s, kubeadm, EKS, GKE, AKS, etc.). No managed SaaS offering is provided by the maintainers.  
**Last-known URL**: https://github.com/ValgulNecron/Gameplane

---

## Pterodactyl

**License**: MIT  
**Documentation Root**: https://pterodactyl.io/  
**Repository**: https://github.com/pterodactyl/panel; https://github.com/pterodactyl/wings

<a id="pterodactyl-row-a"></a>
### Row (a): Deployment/runtime model

**Source ID**: P-a  
**URL**: https://pterodactyl.io/panel/1.0/getting_started.html; https://pterodactyl.io/project/introduction.html  
**Checked on**: 2026-09-02  
**What was verified**: Documentation states Panel is designed for self-hosted deployment on your own web server with multiple dependencies (web server, PHP, MySQL/MariaDB, Redis, systemd worker). Wings is "the next generation server control plane" written in Go, managing Docker containers. Panel and Wings are separate repositories working together: Panel is the user-facing management interface; Wings is the backend control system.  
**Last-known URL**: https://pterodactyl.io/ (docs root)

<a id="pterodactyl-row-b"></a>
### Row (b): Scaling & auto-sleep

**Source ID**: P-b  
**URL**: https://pterodactyl.io/community/config/nodes/add_node.html; https://pterodactyl.io/community/tutorials/artisan.html  
**Checked on**: 2026-09-02  
**What was verified**: Node documentation shows Pterodactyl supports multiple nodes with configurable total memory/disk and overallocation percentages. Artisan CLI includes `p:server:bulk-power` command for start/stop/kill/restart across servers/nodes. Search results describe cron-based scheduling for power actions and automatic restarts, but no dedicated idle/auto-sleep feature is documented. Schedules fire tasks at defined cron intervals.  
**Last-known URL**: https://pterodactyl.io/panel/1.0/ (panel docs entry)

<a id="pterodactyl-row-c"></a>
### Row (c): Inbound connectivity (NAT traversal, relay)

**Source ID**: P-c  
**URL**: https://pterodactyl.io/wings/1.0/installing.html  
**Checked on**: 2026-09-02  
**What was verified**: Wings documentation discusses NAT considerations (internal IP used for systems behind NAT, localhost refers to container not node). Minecraft documentation mentions proxy server setup. Rathole (a reverse proxy for NAT traversal written in Rust) is listed as an available egg in eggs.pterodactyl.io but is not an integrated feature—users must deploy and configure it themselves. No integrated relay sidecars (frp, Tailscale, playit) found in official documentation.  
**Last-known URL**: https://pterodactyl.io/wings/1.0/ (wings docs root)

<a id="pterodactyl-row-d"></a>
### Row (d): Backup and restore

**Source ID**: P-d  
**URL**: https://pterodactyl.io/panel/1.0/additional_configuration.html  
**Checked on**: 2026-09-02  
**What was verified**: Panel documentation states Pterodactyl allows users to create server backups. Two storage drivers are supported: Wings (local, default: `APP_BACKUP_DRIVER=wings`) and S3-compatible (AWS S3 or compatible). Multipart upload is supported for S3. Users can create backups on-demand and via cron-based schedules. Existing backups remain accessible after switching storage methods (if S3 credentials are retained). Restore is performed through the panel interface.  
**Last-known URL**: https://pterodactyl.io/panel/1.0/ (panel docs root)

<a id="pterodactyl-row-e"></a>
### Row (e): Access control & authentication

**Source ID**: P-e  
**URL**: https://pterodactyl.io/panel/1.0/additional_configuration.html; https://pterodactyl.io/community/tutorials/artisan.html  
**Checked on**: 2026-09-02  
**What was verified**: Panel documentation shows 2FA toggle to require 2FA for all accounts or admin-only; can be disabled via Artisan CLI (`p:user:disable2fa`). APP_KEY is used as encryption key for API keys and other secure data. Artisan CLI includes user creation and deletion commands. Subusers are referenced in search results but no dedicated permissions documentation page was located on pterodactyl.io. Custom roles or granular permission system not documented.  
**Last-known URL**: https://pterodactyl.io/panel/1.0/ (panel docs root)

<a id="pterodactyl-row-f"></a>
### Row (f): Game template distribution

**Source ID**: P-f  
**URL**: https://pterodactyl.io/project/terms.html; https://eggs.pterodactyl.io/; https://pterodactyl.io/community/config/eggs/creating_a_custom_egg.html  
**Checked on**: 2026-09-02  
**What was verified**: Terminology documentation defines Nest as "usually used for a specific game or service (Minecraft, Teamspeak, Terraria)" containing many eggs; Egg as "configuration of a specific type of game (Vanilla, Spigot, Bungeecord for Minecraft)". Yolks is "curated collection of core docker images." eggs.pterodactyl.io repository hosts community eggs for games, apps, and programming languages (Minecraft Fabric, Terraria, Minetest, Hytale, etc.). Documentation includes guide for creating custom eggs (valid docker image + start command).  
**Last-known URL**: https://eggs.pterodactyl.io/ (eggs repository)

<a id="pterodactyl-row-g"></a>
### Row (g): Multi-tenancy & multi-cluster

**Source ID**: P-g  
**URL**: https://pterodactyl.io/community/config/nodes/add_node.html; https://pterodactyl.io/project/terms.html  
**Checked on**: 2026-09-02  
**What was verified**: Terminology defines Node as "physical machine running Wings instance"; Servers are "created on nodes, you can have multiple servers per node." Node configuration guide shows adding nodes to a Panel through admin interface with FQDN, daemon port, resource allocation. Documentation describes single Panel managing multiple nodes but does not document remote cluster functionality or cross-node console/log streaming. Architecture suggests single-panel, multi-node within one administrative domain.  
**Last-known URL**: https://pterodactyl.io/panel/1.0/ (panel docs root)

<a id="pterodactyl-row-h"></a>
### Row (h): Licensing

**Source ID**: P-h  
**URL**: https://github.com/pterodactyl/panel; https://github.com/pterodactyl/wings  
**Checked on**: 2026-09-02  
**What was verified**: Panel repository states "Code released under the MIT License." Wings documentation confirms "operates under an MIT license." MIT is a permissive open-source license allowing significant freedom to use, modify, and distribute software with license and copyright notice included.  
**Last-known URL**: https://github.com/pterodactyl/panel (MIT license in repository)

<a id="pterodactyl-row-i"></a>
### Row (i): Target operator scope (self-hosted vs. managed SaaS)

**Source ID**: P-i  
**URL**: https://pterodactyl.io/panel/1.0/getting_started.html; https://pterodactyl.io/wings/1.0/installing.html  
**Checked on**: 2026-09-02  
**What was verified**: Panel documentation states "Pterodactyl Panel is designed to run on your own web server. You will need to have root access to your server in order to run and use this panel." Wings documentation lists supported systems (Ubuntu, RHEL/Rocky, Debian) and explicitly states Windows is unsupported; requires a Linux system capable of running Docker containers. No managed hosting offering is mentioned or linked in official documentation.  
**Last-known URL**: https://pterodactyl.io/ (docs root)

---

## CubeCoders AMP

**License**: Proprietary (subscription-based)  
**Documentation Root**: https://discourse.cubecoders.com/  
**Repository**: Proprietary/unavailable

<a id="cubecoders-row-a"></a>
### Row (a): Deployment/runtime model

**Source ID**: C-a  
**URL**: https://cubecoders.com/AMPInstall, https://discourse.cubecoders.com/t/end-of-support-announcements-for-linux-and-windows-versions/3620  
**Checked on**: 2026-09-02  
**What was verified**: Official CubeCoders pages confirm AMP is a web-based control panel supporting Windows (native) and Linux (Debian 10+, x86_64/aarch64) deployment. Installation pages document platform support.  
**Last-known URL**: https://cubecoders.com/AMP

<a id="cubecoders-row-b"></a>
### Row (b): Scaling & auto-sleep

**Source ID**: C-b  
**URL**: https://discourse.cubecoders.com/t/instance-automatic-sleep/10790, https://discourse.cubecoders.com/t/multiple-worker-amp-servers/12437  
**Checked on**: 2026-09-02  
**What was verified**: Official CubeCoders support forum confirms AMP has "Instance Automatic Sleep" feature (topic 10790). Multi-server architecture documented in forum discussions (topic 12437) showing controller managing instances on multiple physical servers.  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-c"></a>
### Row (c): Inbound connectivity (NAT traversal, relay)

**Source ID**: C-c  
**URL**: https://discourse.cubecoders.com/t/port-forwarding/27899, https://discourse.cubecoders.com/t/how-to-connect-to-amp-remotely/3731  
**Checked on**: 2026-09-02  
**What was verified**: CubeCoders support forums contain discussions of manual port forwarding and remote connectivity setup. No official documentation found stating whether AMP includes built-in relay tunnels, NAT traversal, or automatic port mapping features. Support threads indicate users manually configure port forwarding.  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-d"></a>
### Row (d): Backup and restore

**Source ID**: C-d  
**URL**: https://discourse.cubecoders.com/t/how-do-i-do-a-backup-and-restore-with-amp/11937  
**Checked on**: 2026-09-02  
**What was verified**: CubeCoders support forum documents "how do I do a backup and restore with AMP" (topic 11937) and discussion of restoring backups across different server configurations (topic 2865). Official support confirms backup/restore is a core feature.  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-e"></a>
### Row (e): Access control & authentication

**Source ID**: C-e  
**URL**: https://discourse.cubecoders.com/t/editions-comparison-sheet/2247, https://discourse.cubecoders.com/t/managing-user-permissions-in-amp/2301  
**Checked on**: 2026-09-02  
**What was verified**: Official editions comparison (topic 2247) states Advanced Edition includes "OIDC single-sign-on" and unlimited panel users. Managing permissions documentation (topic 2301) confirms role-based access control system. Search results indicate OIDC support is a distinguishing feature (first mainstream panel to support it per first search).  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-f"></a>
### Row (f): Game template distribution

**Source ID**: C-f  
**URL**: https://discourse.cubecoders.com/t/server-templates/4109, https://discourse.cubecoders.com/t/export-amp-template/23661  
**Checked on**: 2026-09-02  
**What was verified**: Support forum discussions confirm AMP has server templates for dozens of games. Examples include Rust, Minecraft, Arma Reforger, Satisfactory, Assetto Corsa. Templates are exportable and customizable; API supports ApplyTemplate calls for instance redeployment.  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-g"></a>
### Row (g): Multi-tenancy & multi-cluster

**Source ID**: C-g  
**URL**: https://discourse.cubecoders.com/t/multi-tenant-hosting-per-user-instance-limits-resource-quotas-in-amp/40891, https://discourse.cubecoders.com/t/multiple-worker-amp-servers/12437  
**Checked on**: 2026-09-02  
**What was verified**: Support forum (June 2026) documents multi-tenant use case where users create and manage their own instances with role-based visibility (cannot see others' instances). Controller architecture supports instances on separate physical servers. No mention of cross-cluster console/log streaming found.  
**Last-known URL**: https://www.cubecoders.com/AMP

<a id="cubecoders-row-h"></a>
### Row (h): Licensing

**Source ID**: C-h  
**URL**: https://discourse.cubecoders.com/t/editions-comparison-sheet/2247, https://cubecoders.com/AMPTermsOfSale  
**Checked on**: 2026-09-02  
**What was verified**: Official editions comparison topic confirms pricing in GBP and USD for four tiers with corresponding instance limits. Terms of Sale page indicates subscription-based licensing model (unless perpetual license purchased). Licenses issued per-instance with subscription requirement to continue use.  
**Last-known URL**: https://www.cubecoders.com/

<a id="cubecoders-row-i"></a>
### Row (i): Target operator scope (self-hosted vs. managed SaaS)

**Source ID**: C-i  
**URL**: https://cubecoders.com/AMPInstall, https://discourse.cubecoders.com/t/end-of-support-announcements-for-linux-and-windows-versions/3620  
**Checked on**: 2026-09-02  
**What was verified**: Official installation documentation confirms AMP is self-installed on user hardware (Windows or Linux). Support forums document installation requirements and platform prerequisites. No official reference to a managed/hosted SaaS version found.  
**Last-known URL**: https://www.cubecoders.com/AMP

---

## Agones

**License**: Apache License 2.0  
**Documentation Root**: https://agones.dev/site/docs/  
**Repository**: https://github.com/agones-dev/agones

<a id="agones-row-a"></a>
### Row (a): Deployment/runtime model

**Source ID**: A-a  
**URL**: https://agones.dev/site/docs/reference/fleet/  
**Checked on**: 2026-09-02  
**What was verified**: Agones is built as a Kubernetes-native library extending Kubernetes with GameServer and Fleet CRDs. Installation documentation confirms deployment across GKE (Google Kubernetes Engine), EKS (Amazon Elastic Kubernetes Service), AKS (Azure Kubernetes Service), and OKE (Oracle Kubernetes Engine), as well as on-premises and local clusters.  
**Last-known URL**: https://agones.dev/site/docs/reference/fleet/

<a id="agones-row-b"></a>
### Row (b): Scaling & auto-sleep

**Source ID**: A-b  
**URL**: https://agones.dev/site/docs/advanced/scheduling-and-autoscaling/  
**Checked on**: 2026-09-02  
**What was verified**: Agones provides fleet autoscaling through buffer-based strategy (maintaining ready server pools) and webhook-driven strategy (custom autoscaling logic). Advanced feature (Beta) includes CounterPolicy and SchedulePolicy for prioritization during allocation. GameServer state documentation shows states (Ready, Allocated, Reserved, Unhealthy) but no idle or sleep state. Scale-down removes servers from the cluster rather than suspending them.  
**Last-known URL**: https://agones.dev/site/docs/advanced/scheduling-and-autoscaling/

<a id="agones-row-c"></a>
### Row (c): Inbound connectivity (NAT traversal, relay)

**Source ID**: A-c  
**URL**: https://agones.dev/site/docs/reference/gameserver/  
**Checked on**: 2026-09-02  
**What was verified**: GameServer Specification defines port policies: Dynamic (random free hostPort), Static (user-defined port), and Passthrough (containerPort equals hostPort). HostPort routing is implemented via iptables/ipvs kernel routing. No relay or NAT traversal features are documented in the official Agones documentation across Overview, Guides, Advanced topics, or Reference sections.  
**Last-known URL**: https://agones.dev/site/docs/reference/gameserver/

<a id="agones-row-d"></a>
### Row (d): Backup and restore

**Source ID**: A-d  
**URL**: source URL unavailable (checked 2026-09-02)  
**Checked on**: 2026-09-02  
**What was verified**: Agones documentation covers Installation, Getting Started, Guides (SDKs, health checking, metrics), Integration Patterns, Advanced Topics (scheduling, multi-cluster allocation, allocator service), and Reference (CRDs). No documentation section addresses game server backup, snapshots, or restore capabilities. Agones manages GameServer lifecycle (Ready, Allocated, Reserved, Unhealthy, Terminating states) but does not provide persistent state backup or recovery mechanisms. Not applicable: Agones is a Kubernetes operator library orchestrating ephemeral game server instances; persistent state management (if needed) is the responsibility of the game server application and its own storage integration.  
**Last-known URL**: none

<a id="agones-row-e"></a>
### Row (e): Access control & authentication

**Source ID**: A-e  
**URL**: https://agones.dev/site/docs/advanced/allocator-service/  
**Checked on**: 2026-09-02  
**What was verified**: Agones provides no dashboard or user management system. The allocator service (gRPC and REST API for external game server allocation) uses mutual TLS (mTLS) certificate-based authentication between clusters, not user dashboard authentication. Kubernetes RBAC controls API access to GameServer and Fleet CRDs for cluster operators. Not applicable: Agones is a Kubernetes operator library; access control is achieved through Kubernetes RBAC and infrastructure credentials (certificates, keys), not user authentication.  
**Last-known URL**: https://agones.dev/site/docs/advanced/allocator-service/

<a id="agones-row-f"></a>
### Row (f): Game template distribution

**Source ID**: A-f  
**URL**: source URL unavailable (checked 2026-09-02)  
**Checked on**: 2026-09-02  
**What was verified**: Agones documentation covers GameServer/Fleet CRD specifications, client SDKs for game server lifecycle management, deployment on various cloud providers and on-premises, fleet management, autoscaling, multi-cluster allocation, health checking, metrics, and high availability. No module system, template distribution mechanism, or game template registry exists. Agones operates at the infrastructure layer (orchestrating containerized game server instances) and does not provide a game template or module distribution system. Not applicable: Agones is a Kubernetes operator library, not a control panel with game distribution features.  
**Last-known URL**: none

<a id="agones-row-g"></a>
### Row (g): Multi-tenancy & multi-cluster

**Source ID**: A-g  
**URL**: https://agones.dev/site/docs/advanced/multi-cluster-allocation/  
**Checked on**: 2026-09-02  
**What was verified**: Agones supports multi-cluster allocation using GameServerAllocationPolicy CRD which defines redirect rules and cluster priorities. Game servers are allocated from clusters with the lowest priority number; if unavailable, allocation routes to the next-lowest priority cluster. Clusters communicate via the agones-allocator service (gRPC and REST) authenticated with mutual TLS certificates and encryption (Kubernetes secrets store `allocator-tls`, `allocator-client-ca`, and optional `allocator-tls-ca`). Modern recommendation (per documentation) is to use a Service Mesh (Istio, Linkerd) for more powerful multi-cluster routing.  
**Last-known URL**: https://agones.dev/site/docs/advanced/multi-cluster-allocation/

<a id="agones-row-h"></a>
### Row (h): Licensing

**Source ID**: A-h  
**URL**: https://github.com/agones-dev/agones  
**Checked on**: 2026-09-02  
**What was verified**: Agones GitHub repository confirms Apache 2.0 license (SPDX: Apache-2.0). Verified on official repository at github.com/agones-dev/agones; license file and README confirm this permissive open-source license.  
**Last-known URL**: https://github.com/agones-dev/agones

<a id="agones-row-i"></a>
### Row (i): Target operator scope (self-hosted vs. managed SaaS)

**Source ID**: A-i  
**URL**: https://agones.dev/site/docs/installation/  
**Checked on**: 2026-09-02  
**What was verified**: Agones installation documentation explicitly states: "Agones is built with both cloud and on-premises infrastructure in mind, and can run anywhere Kubernetes can run — in the cloud, on premise, on your local machine or anywhere else." Supported deployment targets: GKE (via direct creation and Terraform), EKS, AKS, OKE, Minikube (local development), and bare Kubernetes. No managed SaaS offering or hosted control plane documented. Agones is self-hosted operator software only.  
**Last-known URL**: https://agones.dev/site/docs/installation/
