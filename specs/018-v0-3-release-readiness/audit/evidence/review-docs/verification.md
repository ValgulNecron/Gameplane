# T045 docs chunk: tier-up verification (opus)

Verifier input: `notes.md` in this directory, plus `../review-modules/notes.md` and `../review-website/notes.md` (sonnet reviewer). The candidates are C-docs-01..06, C-modules-01 and C-website-01..03.

Method: I read every cited location and checked it against the code, config or data it describes. That meant `operator/api/v1alpha1/*_types.go`, the generated CRD in `charts/gameplane/crds/`, `charts/gameplane/values.yaml` and templates, `go.work`, the `go.mod` files, `Makefile`, `CHANGELOG.md`, the git history of the `modules/` and `website/` submodules (`website` origin/main was fetched and is identical to the pinned `d84d449`), `gh release list`, and the kubelab baseline snapshot (`audit/evidence/baseline/*.json`, captured 2026-09-23).

For C-modules-01 I read the vendored `sigs.k8s.io/yaml@v1.6.0` source. I also ran one throwaway Go program in the session scratchpad. It unmarshals the 10 cited `template.yaml` files into the real `v1alpha1.GameTemplate` type. It is not a test or lint suite, and no repo file was changed.

None of the kept candidates duplicates an already-tracked item. The tracked `values.yaml:473` module-source ref (`v0.2.0-beta.6`) is related to C-docs-01 and C-website-01/02, but it is a different defect. That item is about which catalog installs pull. These candidates are about the prose counts.

Severity follows research R3. Every kept item here is documentation or wording, so all are S4, the same as the sibling `review-test-e2e/verification.md`. Within S4, fix C-docs-03 first, because its copy-paste example is rejected by the API server.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| C-docs-01 | kept | S4 | Confirmed: `modules/` has 30 module directories, and `docs/game-coverage.md` has 30 rows (2 covered-in-ci, 26 blocked-doc, 2 out-of-scope). README.md:71, 85, 142, 178 and 259 all say 16. comparison-sources.md:69 says 16. roadmap.md:266 says 2/**12**/2 and names game-coverage.md as canonical, which contradicts it. Location corrected: I dropped roadmap.md:164. It sits under "Wake-on-connect … (shipped v0.2.0-beta.8)", and "the other 14" was accurate for that release, whose default catalog (gameplane-module tag `v0.2.0-beta.6`) has exactly 16 modules. The other 14 modules came after beta.8: nuclear-option on 2026-08-23 and 13 more in `d029fee` on 2026-09-11. README.md:178 and :259 describe the repo contents, so they are wrong today whatever the release. Documentation only, so downgraded from S3 to S4. |
| C-modules-01 | rejected | n/a | Refuted. `module_controller.go:188` calls `sigs.k8s.io/yaml.Unmarshal` into the typed struct. That library does not go through a plain YAML→JSON→`encoding/json` path. `convertToJSONableObject` (`sigs.k8s.io/yaml@v1.6.0/yaml.go:181`, default case `:329-356`) walks the target struct and turns YAML int, float and bool scalars into strings when the Go field is a `string`. The path `GameTemplate` → `GameTemplateSpec` → `[]ConfigField` → `Default string` has no custom `UnmarshalJSON` that would turn this off. Checked three ways: (1) a scratch program with the real `v1alpha1.GameTemplate` parsed all 10 files and returned `Default="70"`, `"64"`, `"10"`, `"8"`, `"16"`, `"100"`, `"64"`, `"100"`, `"24"` and `"50"`; (2) the kubelab baseline (`crd-modules.json`, `crd-gametemplates.json`) shows all 10 Modules `Ready` and all 10 GameTemplates materialised; (3) the other parsers also accept a number: the API upload uses `map[string]any` (`module_upload.go:277-285`), the `gp-module` preview uses `fmt.Sprintf("%v")` (`preview.go:171-173`), and the `gp-module` validator reads the node's scalar text (`rules.go:285-289`). What remains is style only: the editor-only JSON Schema (`modules/.schema/gametemplate.schema.json:160-163`, used only through `yaml-language-server` comments) types `default` as a string, and `docs/module-authoring.md:632-636` quotes it. |
| C-website-01 | kept | S4 | Confirmed with context. `website/src/pages/games.astro:9-108` lists 16 games, and the prose says "Sixteen" at `:111`, `:119` and `:148`. The 16 are exactly the gameplane-module `v0.2.0-beta.6` catalog that released beta.8 installs by default. So the live site is accurate for the released product, and the 14 other modules are unreleased (added after beta.8). The v0.3 release will ship 30, and FR-013 plus spec.md:165 require the website to match product behaviour, so this must change when v0.3 is published, together with the tracked `values.yaml:473` ref bump. Not earlier. Documentation only, so downgraded from S3 to S4. |
| C-docs-02 | kept | S4 | Confirmed. `docs/security.md:281-282` says "`capture.enabled: false` (default is true)". The real default is `false`: `values.yaml:527`, and `operator.yaml:283-291` renders `--capture-enabled=false` unless the value is set. `docs/install.md:216` and `docs/architecture.md:304` agree with `false`. The sentence contradicts itself and sits in the PodSecurity `restricted` trade-off discussion. The downgrade from S3 to S4 is because the product default is the safe one. Only the doc misstates it. |
| C-docs-03 | kept | S4 | Confirmed. The GameServer examples at `docs/tunnels.md:125`, `:166` and `:213` use `spec.template: minecraft-java`. The CRD (`charts/gameplane/crds/gameplane.local_gameservers.yaml`) has no `template` property and lists `required: [templateRef]`, with `templateRef.required: [name]` (Go: `gameserver_types.go:35-37`). Every other field in the three examples (`expose`, and `tunnel.{enabled,provider,credentialsSecretRef,frp.{serverAddr,serverPort,remotePorts},tailscale,playit}`) is valid against the CRD. Applying any of the three verbatim fails. This is documentation, so S4 by R3 rather than the suggested S3. It is the first S4 to fix, and the workaround is `docs/networking.md:64-65`'s `templateRef.name`. |
| C-website-02 | kept | S4 | Confirmed. `website/src/content/docs/comparison.mdx:14` says "16 official modules". It is the same situation as C-website-01: accurate for the released beta.8 catalog, stale for v0.3. It is a separate hardcoded copy, so it needs its own edit, but it can be fixed in the same website change as C-website-01. |
| C-docs-05 | kept | S4 | Confirmed, with one correction: **6** workspace modules are left out, not 5. `go.work` has 15 `use` entries. `docs/dependencies.md:4-7` covers 9 and leaves out `sentinel`, `capture-sidecar`, `gameproto`, `svcutil`, `tunnel` and also `gp-module`. `sentinel` gets a passing mention only, in the client-go alignment note at `:363`. Direct deps with no justification anywhere: `capture-sidecar` (`gopacket/gopacket`, `packetcap/go-pcap`, `golang.org/x/net`), `sentinel` (`k8s.io/apimachinery`, `k8s.io/client-go`, plus in-repo `gameproto`) and `gp-module` (`gopkg.in/yaml.v3`, `k8s.io/apimachinery`). `gameproto`, `svcutil` and `tunnel` have no direct third-party deps, but they are also missing the "0 deps" row that `netguard` and `gameaction` get. Documentation only, so downgraded from S3 to S4. |
| C-website-03 | kept | S4 | Confirmed, and wider than reported. `website/src/config.ts:13` has `VERSION = "v0.2.0-beta.7"`, but `v0.2.0-beta.8` is a published GitHub pre-release (2026-08-22). The old constant feeds the homepage install snippet (`index.astro:14`, `chartVersion`), the hero badge (`index.astro:77-78`), the footer, the FAQ and the roadmap heading. `getting-started.mdx:24` hardcodes `--version 0.2.0-beta.7`. `changelog.mdx:12` stops at beta.7 and has no beta.8 entry. Unlike C-website-01, this is stale now. |
| C-docs-06 | kept | S4 | Confirmed, with one correction: **6** modules are missing (the list also leaves out `gp-module`). `docs/contributing.md:74-83` gives `go test` lines for 8 Go modules plus `web`. `Makefile:35` `GO_MODULES` has 14, so `gameproto`, `gp-module`, `sentinel`, `capture-sidecar`, `svcutil` and `tunnel` are missing. `make test` just above the list does cover every module, so the damage is limited to the per-component list. |
| C-docs-04 | kept | S4 | Confirmed, and there are more stale citations. `docs/comparison-sources.md:31` (`CLAUDE.md:368`) and `:40` (`CLAUDE.md:372`) point past the end of the 311-line CLAUDE.md. CLAUDE.md was 828 lines when the citations were written on 2026-09-02. New aspect: `:94` cites "CLAUDE.md:9–20 (Repo map)", but those lines are now the Start-of-Session check (the repo map starts at `:52`). Also `:40` cites `values.yaml:58` as "tunnel.enabled configuration toggle", but `:58` is `operator.sentinelImage`, and the chart has no `tunnel.enabled` key at all. Tunnels are enabled per GameServer (`spec.networking.tunnel.enabled`), and the chart only has `operator.tunnelImages` (`:84-87`). |

### C-docs-01

**Location (corrected):** `README.md:71`, `README.md:85`, `README.md:142`, `README.md:178`, `README.md:259`; `docs/comparison-sources.md:69`; `docs/roadmap.md:266`. (`docs/roadmap.md:164` is left out because it is release-scoped. Update it at the same time if the maintainer wants the shipped-feature notes to read as current.)

Repro (by reading the files):
1. `ls -d modules/*/ | wc -l` returns 30, and every directory has a `module.yaml`. `docs/game-coverage.md:7-36` has 30 rows: 2 `covered-in-ci`, 2 `out-of-scope-by-design` and 26 `blocked-doc`.
2. `grep -n "16" README.md`: lines 71, 85, 142, 178 and 259 each call the catalog 16 templates or games. Line 178 annotates the `modules/` directory itself ("16 games shipped"), and line 259 describes what `make dev-up` pushes ("16 games at last count"). Both describe the repo contents, which are 30 today.
3. `docs/comparison-sources.md:69`, the evidence for README row (f), says "16 ready-to-use templates in gameplane-module repository" with "Checked on 2026-09-02". On that date gameplane-module already had 17 modules, because nuclear-option was added on 2026-08-23.
4. `docs/roadmap.md:262-270` says "Every Gameplane-shipped game module now has a recorded … status. See docs/game-coverage.md for the canonical per-module status", then gives 2 / **12** / 2. The canonical table gives 2 / 26 / 2.
5. Context: `git -C modules ls-tree -d --name-only v0.2.0-beta.6` lists 16 modules, and the modules added later are `f0fa82d` (2026-08-23) and `d029fee` (2026-09-11). The counts matched the beta.8 release and went stale when master's catalog grew to 30.

**Expected:** Counts that match `modules/` and `docs/game-coverage.md`: 30 modules, split 2 covered + 26 blocked-doc + 2 out-of-scope. Or wording that avoids a hardcoded number.

**Actual:** Seven sentences give 16 (or 2/12/2), which is about half of the real catalog. Three of them are in the README comparison table and its evidence file, which exist to compare catalog size against Pterodactyl, AMP and Agones.

### C-website-01

**Location:** `website/src/pages/games.astro:9-108` (the `games` array, 16 entries), plus the prose "Sixteen" / "these sixteen" at `games.astro:111`, `:119` and `:148`.

Repro (by reading the files):
1. `grep -c "title:" website/src/pages/games.astro` returns 16. `grep -n -i sixteen website/src/pages/games.astro` returns lines 111, 119 and 148.
2. Compare with `ls -d modules/*/`, which gives 30. Missing from the page: ark-survival-evolved, arma-reforger, beammp, euro-truck-simulator-2, farming-simulator-25, fivem, hell-let-loose, left-4-dead-2, mount-and-blade-2-bannerlord, nuclear-option, squad, team-fortress-2, the-isle and tmodloader.
3. Context: the 16 on the page are exactly the gameplane-module `v0.2.0-beta.6` tag, which is the chart default the released beta.8 installs. The live site is therefore accurate for today's release. It becomes wrong when v0.3 ships the 30-module catalog.

**Expected:** At v0.3 publication, the games page lists all 30 shipped modules (or says "N+ games" with a correct N), in the same change as the tracked `values.yaml` module-source ref bump.

**Actual:** The website repo's `main` (`d84d449`, same as the pinned submodule) lists 16 and says "Sixteen game servers ship out of the box".

### C-docs-02

**Location:** `docs/security.md:281-282`.

Repro (by reading the files):
1. `docs/security.md:281-282`: "**Disable capture** — leave the cluster's capture feature disabled via Helm value `capture.enabled: false` (default is true)."
2. `charts/gameplane/values.yaml:526-527`: `capture:` / `enabled: false`, with the comment "Off by default".
3. `charts/gameplane/templates/operator.yaml:283-291`: the operator gets `--capture-enabled=false` unless `.Values.capture.enabled` is true. `helm template charts/gameplane | grep capture-enabled` shows `--capture-enabled=false` with default values.
4. `docs/install.md:216` and `docs/architecture.md:304` both say the default is false or disabled.

**Expected:** "(default is false)", or no parenthetical at all, since option 1 then just means "keep the default".

**Actual:** The security doc claims the capture feature, whose sidecar needs `allowPrivilegeEscalation: true`, is on by default. That contradicts the chart and two other docs.

### C-docs-03

**Location:** `docs/tunnels.md:125`, `:166`, `:213`.

Repro (live, on any cluster with the Gameplane CRDs installed, such as kubelab):
1. Copy the frp example from `docs/tunnels.md:118-139` into `gs.yaml`. It has `spec.template: minecraft-java`.
2. Run `kubectl apply --dry-run=server -f gs.yaml`.
3. The API server rejects it. With kubectl's default strict field validation you get an `unknown field "spec.template"` error. With validation relaxed, the unknown field is pruned and you get `spec.templateRef: Required value`. Either way no GameServer is created. The Tailscale (`:166`) and playit (`:213`) examples fail the same way.
4. Offline check: in `charts/gameplane/crds/gameplane.local_gameservers.yaml`, `spec.properties` has no `template`, and `spec.required` is `[templateRef]` (Go: `operator/api/v1alpha1/gameserver_types.go:35-37`, no `omitempty`).

**Expected:** `spec.templateRef.name: minecraft-java`, as in `docs/networking.md:64-65`.

**Actual:** All three tunnel walkthroughs give a manifest the API server rejects.

### C-website-02

**Location:** `website/src/content/docs/comparison.mdx:14`.

Repro (by reading the files):
1. `sed -n 14p website/src/content/docs/comparison.mdx` shows `| Game catalog | 16 official modules; open template format | …`.
2. `ls -d modules/*/ | wc -l` gives 30. As with C-website-01, 16 is right for the released beta.8 default catalog and wrong for v0.3.

**Expected:** The v0.3 catalog size, updated in the same website change as C-website-01.

**Actual:** "16 official modules".

### C-docs-05

**Location:** `docs/dependencies.md:4-7` (scope statement), plus no per-module sections for the modules it leaves out.

Repro (by reading the files):
1. `docs/dependencies.md:4-7`: "It covers the 9 Go modules that share `go.work` (`netguard`, `gameaction`, `operator`, `api`, `agent`, `audit-syslog-bridge`, `telemetry-receiver`, `mcp-server`, `test/e2e`)".
2. `go.work` lists 15 modules. The six missing are `capture-sidecar`, `gameproto`, `gp-module`, `sentinel`, `svcutil` and `tunnel`.
3. `grep -n -i "capture-sidecar\|gopacket\|go-pcap\|gameproto\|svcutil\|tunnel\|gp-module" docs/dependencies.md` finds nothing. `sentinel` appears only at `:363`, in the client-go version-alignment note.
4. Direct `require` entries (non-indirect) that have no justification:
   - `capture-sidecar/go.mod`: `github.com/gopacket/gopacket`, `github.com/packetcap/go-pcap`, `golang.org/x/net`
   - `sentinel/go.mod`: `k8s.io/apimachinery`, `k8s.io/client-go`
   - `gp-module/go.mod`: `gopkg.in/yaml.v3`, `k8s.io/apimachinery`

**Expected:** Every `go.work` module is inventoried. Zero-dependency ones get the same "no third-party deps" row that `netguard` and `gameaction` have.

**Actual:** Six modules are missing, including the raw packet-capture library that runs with `CAP_NET_RAW`.

### C-website-03

**Location (corrected and extended):** `website/src/config.ts:13`; `website/src/content/docs/getting-started.mdx:24`; `website/src/content/docs/changelog.mdx:12` (latest entry is beta.7). `VERSION` is also rendered at `website/src/pages/index.astro:14` (homepage install `chartVersion`), `:77-78`, `website/src/components/Footer.astro:71`, `website/src/content/docs/faq.mdx:24` and `website/src/content/docs/roadmap.mdx:13`.

Repro (by reading the files):
1. `gh release list -R ValgulNecron/Gameplane -L 2` shows `v0.2.0-beta.8`, Pre-release, 2026-08-22. `CHANGELOG.md:56` has `## [0.2.0-beta.8] — 2026-08-22`.
2. `grep -rn "VERSION\|beta\.7" website/src` shows the `beta.7` constant and its render sites listed above, plus the hardcoded `--version 0.2.0-beta.7` in getting-started.
3. `git -C website fetch && git -C website show origin/main:src/config.ts | grep VERSION` gives `v0.2.0-beta.7`, so the website repo itself has not moved on.

**Expected:** The site shows and installs the latest published release (beta.8 now, v0.3.0 at release), and the changelog page includes it.

**Actual:** The homepage install command, getting-started command, badge, footer, FAQ and roadmap all say beta.7, and the changelog page stops at beta.7.

### C-docs-06

**Location:** `docs/contributing.md:74-83`.

Repro (by reading the files):
1. `docs/contributing.md:74-83` lists `cd <m> && go test ./...` for `netguard`, `gameaction`, `operator`, `api`, `agent`, `audit-syslog-bridge`, `telemetry-receiver` and `mcp-server`, then `cd web && npm test`.
2. `Makefile:35` `GO_MODULES` also includes `gameproto`, `gp-module`, `sentinel`, `capture-sidecar`, `svcutil` and `tunnel`.

**Expected:** The per-component list covers every Go module in `GO_MODULES`, or points to `make test` as the complete list.

**Actual:** Six modules are missing from the per-component list. The aggregate `make test` in the same section does cover them.

### C-docs-04

**Location (extended):** `docs/comparison-sources.md:31`, `:40`, `:94`.

Repro (by reading the files):
1. `wc -l CLAUDE.md` gives 311.
2. `docs/comparison-sources.md:31` cites `CLAUDE.md:368`, and `:40` cites `CLAUDE.md:372`. Both are past the end of the file. At the 2026-09-02 "Checked on" date CLAUDE.md had 828 lines, and it was trimmed later.
3. `docs/comparison-sources.md:94` cites "CLAUDE.md:9–20 (Repo map)". `sed -n 9,20p CLAUDE.md` is now the Start-of-Session dependency check. `## Repository Map` is at `CLAUDE.md:52`.
4. `docs/comparison-sources.md:40-42` also cites `charts/gameplane/values.yaml:58` as "tunnel.enabled configuration toggle". `values.yaml:58` is `sentinelImage`, and `grep -n "^tunnel" charts/gameplane/values.yaml` finds nothing. Tunnels are turned on per GameServer (`spec.networking.tunnel.enabled`), and the chart only carries `operator.tunnelImages` (`values.yaml:84-87`).

**Expected:** Every evidence citation in this audit-trail file resolves to the claim it supports, preferably by section anchor instead of a line number that drifts.

**Actual:** Three CLAUDE.md citations and one values.yaml citation no longer point at the evidence. The values.yaml one also describes a Helm key that does not exist.
