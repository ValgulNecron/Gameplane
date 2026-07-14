# Multi-Category Modules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn a module's single `category: string` into `categories: []string` end to end, so a game can be both "Survival" and "Sandbox", and give the official modules real categories instead of a regex guess.

**Architecture:** `category` is a single string in three Go types (`GameTemplateSpec`, `ModuleEntry`, and the internal bundle `Metadata`), the API's `CatalogEntry` DTO, and the web types. It becomes a list in all five. The bundle parser keeps accepting a legacy scalar `category:` and normalizes it to a one-element list, so third-party bundles authored before this change keep working. The dashboard's filter chips, which are derived from the distinct values actually present, now flatten a union and match a module if **any** of its categories match the active chip.

**Tech Stack:** Go 1.25 (controller-runtime, controller-tools v0.20.1), sigs.k8s.io/yaml, React 18 + TypeScript strict, Vitest, Pencil (design.pen).

## Global Constraints

- **Phase 1 of** `docs/superpowers/specs/2026-07-14-console-protocols-categories-actions-design.md`. Read the spec's §2 before starting.
- **Do NOT run tests or linters locally.** Push to a branch; GitHub Actions is the source of truth (CLAUDE.md rule 8). A compile check (`go build ./...`, `go vet ./...`, `tsc --noEmit`) is fine and encouraged. Note `go build` skips test files — use `go vet ./...` to compile-check tests.
- **After editing CRD Go types, run `make generate && make manifests`** and commit the regenerated `zz_generated.deepcopy.go`, `operator/config/crd/*.yaml`, `operator/config/rbac/*.yaml`, and `charts/gameplane/crds/*.yaml` in the SAME commit as the source change (CLAUDE.md rule 7). Codegen is not a test — running it locally is required.
- **Sign every commit**: `git -c commit.gpgsign=false commit -s -m "..."` (the `-c` MUST precede the subcommand). Conventional-commit prefixes.
- **Fix, don't silence** (CLAUDE.md rule 4): no `//nolint`, no `eslint-disable`.
- **Go errors wrap with `%w`** (CLAUDE.md rule 6).
- **TypeScript strict, no `any`** (CLAUDE.md rule 5).
- **Never stage the `modules` submodule pointer** unless the task explicitly says to.

### DANGER: `category` is an overloaded word in this repo

There are **two unrelated `category` concepts**. Only the first is in scope.

| In scope (module catalog) | OUT OF SCOPE — do not touch |
|---|---|
| `GameTemplateSpec.Category` | `api/internal/registry/{modrinth,hangar,umod}.go` — `SearchQuery.Category` is a **mod-registry search facet** |
| `ModuleEntry.Category` | `web/src/lib/endpoints.ts:186,197` — the registry search `category` query param |
| `modsrc.Metadata.Category` | `web/src/components/registry-browser.tsx` — registry category chips |
| `handlers.CatalogEntry.Category` | `web/src/routes/tabs/Mods.tsx`, `Modpacks.test.tsx` |
| `web/src/lib/games.ts`, `Modules.tsx`, `CreateServer.tsx` | |

A blind rename across the repo **will break mod search**. Touch only the left column.

### The canonical category vocabulary

Values stay free-form in the CRD (a third-party module can coin a new one with no frontend change). These 11 are the canon the official modules use, documented in `docs/module-authoring.md`:

**Survival · Sandbox · Shooter · Simulation · Building · Adventure · Horror · Co-op · PvP · Modded · Creative**

---

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `operator/api/v1alpha1/gametemplate_types.go` | `GameTemplateSpec.Categories []string` | 1 |
| `operator/api/v1alpha1/modulesource_types.go` | `ModuleEntry.Categories []string` | 1 |
| `operator/api/v1alpha1/zz_generated.deepcopy.go` | GENERATED — `make generate` | 1 |
| `operator/config/crd/*.yaml`, `charts/gameplane/crds/*.yaml` | GENERATED — `make manifests` | 1 |
| `operator/internal/modsrc/bundle.go` | `Metadata.Categories` + legacy-scalar normalization | 2 |
| `operator/internal/modsrc/{oci,dir,upload}.go` | carry the list into `ModuleEntry` | 2 |
| `api/internal/handlers/modules.go` | `CatalogEntry.Categories`; union-merge across sources | 3 |
| `web/src/lib/games.ts` | `resolveCategories`, `categoryFilters` over lists | 4 |
| `web/src/lib/games.test.ts` | NEW — no test file exists for this module today | 4 |
| `design.pen` (Pencil MCP) | filter-chip-row overflow, Modules + Create Server | 5 |
| `web/src/types.ts`, `web/src/routes/{Modules,CreateServer}.tsx` | `categories?: string[]`; any-match filter; chip overflow | 6 |
| `modules/*/{module.yaml,template.yaml}` (submodule) | real categories for all 16 modules | 7 |
| `docs/module-authoring.md` | `categories` field + the canon | 8 |

**Ordering note:** Task 5 (design) MUST precede Task 6 (React). CLAUDE.md rule 1: the dashboard's visual surface starts in `design.pen`, not in code.

**Why the design pass is needed:** module cards do **not** render categories today — `resolveCategory` is used only in filter logic. The visual change is the *filter chip row*: the heuristic currently yields at most 4 chips (`all`, Survival, Sandbox, Shooter), but the 11-value canon will produce ~12. Twelve chips overflow the row on both pages, and the fix (wrap, horizontal scroll, or collapse) is a design decision.

---

## Task 1: CRD types become lists

**Files:**
- Modify: `operator/api/v1alpha1/gametemplate_types.go:24-31`
- Modify: `operator/api/v1alpha1/modulesource_types.go:250-254`
- Generated: `operator/api/v1alpha1/zz_generated.deepcopy.go`, `operator/config/crd/*.yaml`, `charts/gameplane/crds/*.yaml`
- Test: `operator/internal/controller/gametemplate_envtest_test.go`

**Interfaces:**
- Produces: `GameTemplateSpec.Categories []string` and `ModuleEntry.Categories []string`. Tasks 2 and 3 consume these. The old `Category string` fields are **removed**, not deprecated.

- [ ] **Step 1: Write the failing envtest**

Add to `operator/internal/controller/gametemplate_envtest_test.go`. Follow the file's existing pattern for building and applying a GameTemplate (copy the minimal-valid-spec helper it already uses; a GameTemplate needs at least `displayName`, `game`, `version`, and an image — reuse whatever the neighbouring tests construct).

```go
func TestGameTemplateCategoriesRoundTrip(t *testing.T) {
	ctx := context.Background()
	tmpl := minimalGameTemplate("cat-roundtrip")
	tmpl.Spec.Categories = []string{"Sandbox", "Survival", "Building", "Modded", "Creative"}

	if err := k8sClient.Create(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, tmpl) })

	var got gameplanev1alpha1.GameTemplate
	key := client.ObjectKeyFromObject(tmpl)
	if err := k8sClient.Get(ctx, key, &got); err != nil {
		t.Fatalf("get template: %v", err)
	}
	want := []string{"Sandbox", "Survival", "Building", "Modded", "Creative"}
	if !reflect.DeepEqual(got.Spec.Categories, want) {
		t.Errorf("categories = %v, want %v", got.Spec.Categories, want)
	}
}

func TestGameTemplateCategoriesRejectsTooMany(t *testing.T) {
	ctx := context.Background()
	tmpl := minimalGameTemplate("cat-toomany")
	tmpl.Spec.Categories = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} // 9 > MaxItems=8

	err := k8sClient.Create(ctx, tmpl)
	if err == nil {
		_ = k8sClient.Delete(ctx, tmpl)
		t.Fatal("create with 9 categories succeeded, want apiserver rejection")
	}
	if !apierrors.IsInvalid(err) {
		t.Errorf("err = %v, want Invalid", err)
	}
}
```

Ensure the imports include `reflect` and `apierrors "k8s.io/apimachinery/pkg/api/errors"`.

- [ ] **Step 2: Run the test to verify it fails**

Per the global constraints, do **not** run envtest locally. Instead compile-check:

Run: `cd operator && go vet ./...`
Expected: FAIL — `tmpl.Spec.Categories undefined (type GameTemplateSpec has no field or method Categories)`

That compile failure is this step's "red".

- [ ] **Step 3: Change the types**

In `operator/api/v1alpha1/gametemplate_types.go`, replace lines 24-31 entirely:

```go
	// Categories group this game in the dashboard's catalog and Create
	// Server picker (e.g. ["Survival", "Sandbox"]). A game may belong to
	// several at once — Minecraft is reasonably Sandbox, Survival and
	// Creative. The dashboard builds its category filter from the distinct
	// values present across installed templates, so a module introduces a
	// new category simply by naming it here — no frontend change. Empty
	// falls back to a heuristic on the game slug, and finally to "Other".
	// docs/module-authoring.md publishes the canonical vocabulary the
	// official modules use.
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:items:MaxLength=32
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	Categories []string `json:"categories,omitempty"`
```

In `operator/api/v1alpha1/modulesource_types.go`, replace lines 250-254 entirely:

```go
	// Categories are the catalog groupings from module.yaml (e.g.
	// ["Survival", "Sandbox"]). Surfaced so the dashboard can build its
	// module-catalog filter from author-declared values instead of a
	// hardcoded heuristic. A bundle that declares the legacy scalar
	// `category:` is normalized into a one-element list on parse.
	// +kubebuilder:validation:MaxItems=8
	// +kubebuilder:validation:items:MaxLength=32
	// +kubebuilder:validation:items:MinLength=1
	// +optional
	Categories []string `json:"categories,omitempty"`
```

- [ ] **Step 4: Regenerate**

Run: `make generate && make manifests`
Expected: `operator/api/v1alpha1/zz_generated.deepcopy.go` gains a `Categories` copy in both `GameTemplateSpec.DeepCopyInto` and `ModuleEntry.DeepCopyInto`; `operator/config/crd/gameplane.local_gametemplates.yaml` and `..._modulesources.yaml` replace the `category: {type: string}` property with `categories: {type: array, maxItems: 8, items: {type: string, maxLength: 32, minLength: 1}}`; the same lands in `charts/gameplane/crds/`.

Verify: `git diff --stat charts/gameplane/crds/` shows both CRD files changed. If `charts/gameplane/crds/` is untouched, `make manifests` did not sync — do not proceed.

- [ ] **Step 5: Compile-check**

Run: `cd operator && go build ./... && go vet ./...`
Expected: PASS. (Task 2 fixes the `modsrc` callers; if `go vet` still fails there, that is expected — proceed to Task 2 and re-run at its end. It must pass by the end of Task 2.)

Note: `operator/internal/modsrc/{oci,dir,upload}.go` reference `entry.Category` and will now fail to compile. That is Task 2's problem; the two tasks land as one compiling commit only if you prefer — but the cleaner split is: do Task 2 immediately and commit both together, since Task 1 alone leaves the tree broken (see CLAUDE.md rule 11: never commit a known-broken state).

**Therefore: do NOT commit at the end of Task 1. Commit at the end of Task 2.**

---

## Task 2: Bundle metadata accepts a list (and a legacy scalar)

**Files:**
- Modify: `operator/internal/modsrc/bundle.go:35` (the `Metadata` struct) and `:79` (after `yaml.Unmarshal`)
- Modify: `operator/internal/modsrc/oci.go:91`, `dir.go:54`, `upload.go:45`
- Test: `operator/internal/modsrc/bundle_test.go` (new test funcs), `operator/internal/modsrc/dir_test.go:68-69` (existing assertion must be updated)

**Interfaces:**
- Consumes: `gameplanev1alpha1.ModuleEntry.Categories []string` (Task 1).
- Produces: `modsrc.Metadata.Categories []string`, populated from either `categories: [a, b]` or the legacy `category: a`.

- [ ] **Step 1: Write the failing tests**

Add to `operator/internal/modsrc/bundle_test.go`. (If a `Metadata`-parsing helper already exists in that file, reuse it; these tests call `ParseBundle`/the same entry point the existing bundle tests use — match the file's existing convention for constructing a bundle from raw `module.yaml` bytes.)

```go
func TestMetadataCategoriesList(t *testing.T) {
	meta := []byte(`
apiVersion: gameplane.local/module/v1
name: mc
displayName: Minecraft
version: 1.0.0
game: minecraft-java
categories: [Sandbox, Survival, Building]
`)
	m, err := parseMetadata(meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"Sandbox", "Survival", "Building"}
	if !reflect.DeepEqual(m.Categories, want) {
		t.Errorf("Categories = %v, want %v", m.Categories, want)
	}
}

func TestMetadataLegacyScalarCategory(t *testing.T) {
	meta := []byte(`
apiVersion: gameplane.local/module/v1
name: mc
displayName: Minecraft
version: 1.0.0
game: minecraft-java
category: Sandbox
`)
	m, err := parseMetadata(meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(m.Categories, []string{"Sandbox"}) {
		t.Errorf("Categories = %v, want [Sandbox]", m.Categories)
	}
	if m.Category != "" {
		t.Errorf("legacy Category = %q, want cleared after normalization", m.Category)
	}
}

func TestMetadataListWinsOverLegacyScalar(t *testing.T) {
	meta := []byte(`
apiVersion: gameplane.local/module/v1
name: mc
displayName: Minecraft
version: 1.0.0
game: minecraft-java
category: Ignored
categories: [Sandbox, Survival]
`)
	m, err := parseMetadata(meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !reflect.DeepEqual(m.Categories, []string{"Sandbox", "Survival"}) {
		t.Errorf("Categories = %v, want [Sandbox Survival]", m.Categories)
	}
}

func TestMetadataNoCategory(t *testing.T) {
	meta := []byte(`
apiVersion: gameplane.local/module/v1
name: mc
displayName: Minecraft
version: 1.0.0
game: minecraft-java
`)
	m, err := parseMetadata(meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Categories) != 0 {
		t.Errorf("Categories = %v, want empty", m.Categories)
	}
}
```

`parseMetadata` above stands for whatever the file's existing bundle-parse entry point is (the function reached from `bundle.go:79`). If the only entry point is the exported bundle loader, call that instead and read `bundle.Metadata` — do not add a new exported function just for the test.

- [ ] **Step 2: Run to verify it fails**

Run: `cd operator && go vet ./internal/modsrc/`
Expected: FAIL — `m.Categories undefined`.

- [ ] **Step 3: Change `Metadata` and add normalization**

In `operator/internal/modsrc/bundle.go`, replace line 35 with:

```go
	Categories        []string `yaml:"categories,omitempty" json:"categories,omitempty"`
	// Category is the legacy singular form, accepted for back-compat with
	// bundles authored before categories became a list. normalizeCategories
	// folds it into Categories and clears it; nothing else reads it.
	Category          string `yaml:"category,omitempty" json:"category,omitempty"`
```

Add to the same file:

```go
// normalizeCategories folds the legacy scalar `category:` into Categories.
// An explicit `categories:` list always wins; the scalar is cleared either
// way so no caller can read a stale value.
func (m *Metadata) normalizeCategories() {
	if len(m.Categories) == 0 && m.Category != "" {
		m.Categories = []string{m.Category}
	}
	m.Category = ""
}
```

At `bundle.go:79`, immediately after the metadata unmarshal succeeds:

```go
	if err := yaml.Unmarshal(meta, &b.Metadata); err != nil {
		return b, fmt.Errorf("parse module.yaml: %w", err)
	}
	b.Metadata.normalizeCategories()
```

(Preserve the existing error message and wrapping at that line — only the `normalizeCategories()` call is new.)

- [ ] **Step 4: Update the three callers**

`operator/internal/modsrc/oci.go:91`:
```go
	entry.Categories = bundle.Metadata.Categories
```

`operator/internal/modsrc/dir.go:54` — within the `ModuleEntry` literal, replace `Category: meta.Category,` with:
```go
			Categories:    meta.Categories,
```

`operator/internal/modsrc/upload.go:45` — same replacement:
```go
			Categories:    meta.Categories,
```

- [ ] **Step 5: Update the existing dir_test assertion**

`operator/internal/modsrc/dir_test.go:68-69` currently asserts `mc.Category != "Sandbox"`. Replace with:

```go
	if !reflect.DeepEqual(mc.Categories, []string{"Sandbox"}) {
		t.Errorf("mc categories = %v, want [Sandbox]", mc.Categories)
	}
```

Check the fixture that test reads: if it writes a `module.yaml` containing `category: Sandbox`, leave it — it now exercises the legacy path, which is exactly what we want covered. Add `reflect` to the imports if absent.

- [ ] **Step 6: Compile-check the whole operator**

Run: `cd operator && go build ./... && go vet ./...`
Expected: PASS with no references to `.Category` remaining outside `bundle.go`.

Verify: `grep -rn --include="*.go" "\.Category\b" operator/` returns only `bundle.go` (the legacy field and its normalizer).

- [ ] **Step 7: Commit Tasks 1 + 2 together**

```bash
git add operator/api/v1alpha1/ operator/config/ charts/gameplane/crds/ operator/internal/modsrc/
git -c commit.gpgsign=false commit -s -m "feat(operator): modules declare multiple categories

GameTemplateSpec.Category and ModuleEntry.Category become Categories
[]string (MaxItems=8, each 1-32 chars). The bundle parser still accepts
a legacy scalar 'category:' and normalizes it to a one-element list, so
bundles authored before this change keep working.

Includes regenerated deepcopy + CRD manifests."
```

---

## Task 3: API catalog merges categories as a union

**Files:**
- Modify: `api/internal/handlers/modules.go:72` (the `CatalogEntry` DTO) and `:320-322` (the merge)
- Test: `api/internal/handlers/modules_unit_test.go`

**Interfaces:**
- Consumes: `ModuleEntry.Categories` (Task 1) via the unstructured `status.modules` slice.
- Produces: `CatalogEntry.Categories []string` on the wire as `"categories": ["Survival","Sandbox"]`. Task 6's `web/src/types.ts` mirrors this.

**Why a union, not first-wins:** the same module can be served by several ModuleSources. Today the merge takes the first non-empty `category`. With a list, taking only the first source's list would silently drop categories the other source declares. Dedupe case-insensitively (first spelling wins) so `Survival` and `survival` do not both become chips.

- [ ] **Step 1: Write the failing test**

Add to `api/internal/handlers/modules_unit_test.go`:

```go
func TestMergeCategoriesUnionsAcrossSources(t *testing.T) {
	got := mergeCategories([]string{"Survival", "Sandbox"}, []string{"sandbox", "Co-op"})
	want := []string{"Survival", "Sandbox", "Co-op"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeCategories = %v, want %v (case-insensitive dedupe, first spelling wins, order stable)", got, want)
	}
}

func TestMergeCategoriesEmptyInputs(t *testing.T) {
	if got := mergeCategories(nil, nil); len(got) != 0 {
		t.Errorf("mergeCategories(nil, nil) = %v, want empty", got)
	}
	if got := mergeCategories(nil, []string{"PvP"}); !reflect.DeepEqual(got, []string{"PvP"}) {
		t.Errorf("mergeCategories(nil, [PvP]) = %v, want [PvP]", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd api && go vet ./internal/handlers/`
Expected: FAIL — `undefined: mergeCategories`.

- [ ] **Step 3: Implement**

In `api/internal/handlers/modules.go`, change line 72 from `Category string` to:

```go
	Categories       []string    `json:"categories,omitempty"`
```

Add near the existing `mergeVersions` helper (keep it beside its sibling — same responsibility, same file region):

```go
// mergeCategories unions a module's categories across the ModuleSources
// serving it. Dedupe is case-insensitive so "Survival" and "survival" do
// not become two chips; the first spelling seen wins, and input order is
// preserved so the dashboard's chip list is stable across reloads.
func mergeCategories(dst, src []string) []string {
	seen := make(map[string]struct{}, len(dst)+len(src))
	out := make([]string, 0, len(dst)+len(src))
	for _, list := range [][]string{dst, src} {
		for _, c := range list {
			c = strings.TrimSpace(c)
			if c == "" {
				continue
			}
			k := strings.ToLower(c)
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}
```

Replace `modules.go:320-322` (the `if c, _ := m["category"]...` block) with:

```go
			cats, _, _ := unstructured.NestedStringSlice(m, "categories")
			e.Categories = mergeCategories(e.Categories, cats)
```

`strings` and `unstructured` are already imported in this file (`unstructured.NestedStringSlice` is used a few lines below for `versions`).

- [ ] **Step 4: Compile-check**

Run: `cd api && go build ./... && go vet ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/handlers/modules.go api/internal/handlers/modules_unit_test.go
git -c commit.gpgsign=false commit -s -m "feat(api): union module categories across sources

CatalogEntry.Category becomes Categories []string. When several
ModuleSources serve the same module, their category lists are unioned
(case-insensitive dedupe, first spelling wins) rather than the old
first-non-empty-wins, which would silently drop the other source's
categories."
```

---

## Task 4: Web category helpers operate on lists

**Files:**
- Modify: `web/src/lib/games.ts` (whole file)
- Create: `web/src/lib/games.test.ts` — **no test file exists for this module today**

**Interfaces:**
- Produces, for Task 6:
  - `resolveCategories(explicit: string[] | undefined, game: string): string[]` — declared categories, or `[gameCategory(game)]` when none are declared.
  - `categoryFilters(categories: string[][]): string[]` — **note the signature change**: it now takes one array *per module* and flattens them. Returns `["all", ...sorted named..., "Other"?]`.
  - `gameCategory(game: string): string` — unchanged heuristic, still exported.
  - `OTHER_CATEGORY` — unchanged.

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/games.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { OTHER_CATEGORY, categoryFilters, gameCategory, resolveCategories } from "./games";

describe("resolveCategories", () => {
  it("returns the declared categories", () => {
    expect(resolveCategories(["Sandbox", "Survival"], "minecraft-java")).toEqual([
      "Sandbox",
      "Survival",
    ]);
  });

  it("falls back to the game-slug heuristic when none are declared", () => {
    expect(resolveCategories(undefined, "valheim")).toEqual(["Survival"]);
    expect(resolveCategories([], "minecraft-java")).toEqual(["Sandbox"]);
  });

  it("falls back to Other for an unknown game", () => {
    expect(resolveCategories(undefined, "some-unknown-game")).toEqual([OTHER_CATEGORY]);
  });

  it("drops blank entries and trims", () => {
    expect(resolveCategories(["  Sandbox  ", "", "   "], "minecraft-java")).toEqual(["Sandbox"]);
  });

  it("falls back to the heuristic when every declared entry is blank", () => {
    expect(resolveCategories(["", "  "], "valheim")).toEqual(["Survival"]);
  });
});

describe("categoryFilters", () => {
  it("flattens per-module lists, sorts named categories, and puts Other last", () => {
    expect(
      categoryFilters([
        ["Sandbox", "Survival"],
        ["Survival", "Co-op"],
        [OTHER_CATEGORY],
      ]),
    ).toEqual(["all", "Co-op", "Sandbox", "Survival", OTHER_CATEGORY]);
  });

  it("omits Other when no module falls into it", () => {
    expect(categoryFilters([["Shooter"], ["PvP"]])).toEqual(["all", "PvP", "Shooter"]);
  });

  it("returns just all for an empty catalog", () => {
    expect(categoryFilters([])).toEqual(["all"]);
  });

  it("dedupes case-insensitively, keeping the first spelling", () => {
    expect(categoryFilters([["Survival"], ["survival"]])).toEqual(["all", "Survival"]);
  });
});

describe("gameCategory", () => {
  it("still classifies known slugs", () => {
    expect(gameCategory("valheim")).toBe("Survival");
    expect(gameCategory("minecraft-java")).toBe("Sandbox");
    expect(gameCategory("cs2")).toBe("Shooter");
    expect(gameCategory("nothing-like-this")).toBe(OTHER_CATEGORY);
  });
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx tsc --noEmit`
Expected: FAIL — `Module './games' has no exported member 'resolveCategories'`.

- [ ] **Step 3: Rewrite `web/src/lib/games.ts`**

Replace the whole file:

```ts
// Category grouping for the module catalog + Create-Server template picker.
// A module may declare several categories (GameTemplate.spec.categories /
// CatalogEntry.categories) — Minecraft is reasonably Sandbox, Survival and
// Creative at once. When a module declares none we fall back to a
// best-effort heuristic on the game slug, so third-party modules that
// predate the field still group sensibly. The dashboard builds its filter
// chips from the categories actually present rather than a fixed list.

export const OTHER_CATEGORY = "Other";

// gameCategory is the heuristic fallback used when a module declares no
// explicit category. Unknown games map to "Other".
export function gameCategory(game: string): string {
  const g = game.toLowerCase();
  if (/valheim|palworld|ark|rust|conan|7.?days|dayz/.test(g)) return "Survival";
  if (/minecraft|terraria|factorio|satisfactory|stardew/.test(g)) return "Sandbox";
  if (/cs2|cs.?go|csgo|tf2|valorant|insurgency|squad|left4dead/.test(g)) return "Shooter";
  return OTHER_CATEGORY;
}

// resolveCategories returns a module's declared categories, or a
// single-element heuristic fallback when it declares none usable.
export function resolveCategories(explicit: string[] | undefined, game: string): string[] {
  const declared = (explicit ?? []).map((c) => c.trim()).filter((c) => c.length > 0);
  return declared.length > 0 ? declared : [gameCategory(game)];
}

// categoryFilters builds the ordered chip list from the resolved categories
// of every item in the catalog — one string[] per module. "all" first, then
// the distinct named categories sorted alphabetically, then "Other" last
// (only if some module actually falls into it). Dedupe is case-insensitive
// so "Survival" and "survival" do not become two chips; the first spelling
// seen wins, matching the API's merge.
export function categoryFilters(categories: string[][]): string[] {
  const bySlug = new Map<string, string>();
  for (const list of categories) {
    for (const c of list) {
      const key = c.toLowerCase();
      if (!bySlug.has(key)) bySlug.set(key, c);
    }
  }
  const present = [...bySlug.values()];
  const named = present.filter((c) => c !== OTHER_CATEGORY).sort((a, b) => a.localeCompare(b));
  const tail = present.includes(OTHER_CATEGORY) ? [OTHER_CATEGORY] : [];
  return ["all", ...named, ...tail];
}
```

Note `resolveCategory` (singular) is **gone**. Task 6 updates its two callers; nothing else imports it (verified: only `Modules.tsx:16` and `CreateServer.tsx:21`).

- [ ] **Step 4: Run the tests**

Run: `cd web && npx tsc --noEmit`
Expected: FAIL still — `Modules.tsx` and `CreateServer.tsx` import the now-deleted `resolveCategory`. That is expected; Task 6 fixes it. Do not "fix" it by re-adding `resolveCategory`.

Because the tree does not compile at the end of this task, **do not commit yet.** Commit at the end of Task 6 (CLAUDE.md rule 11: never commit a known-broken state).

---

## Task 5: Design pass — filter-chip-row overflow (Pencil)

**Files:**
- Modify: `design.pen` via the **Pencil MCP server only**. Never `Read`, `Grep`, `cat`, `sed`, or `rm` this file — it is encrypted (CLAUDE.md rule 2).

**Interfaces:**
- Produces: an agreed visual treatment for a ~12-chip filter row, which Task 6 translates to React.

**The problem to solve:** the Modules page and the Create Server page each render a horizontal row of category filter chips. Today the heuristic yields at most 4 (`all`, Survival, Sandbox, Shooter). With the 11-value canon, the row becomes ~12 chips and overflows on narrower viewports.

- [ ] **Step 1: Open the document and orient**

- `mcp__pencil__get_editor_state` with `include_schema: true` (required before any other Pencil call).
- `mcp__pencil__open_document` on the repo's `design.pen`.
- `mcp__pencil__batch_get` to locate the **Modules** and **Create Server** screens and their existing chip-row frames.
- `mcp__pencil__get_screenshot` of both screens as they stand.

**Prerequisite:** Pencil MCP reaches the repo's `design.pen` only when the file is open in the Pencil GUI. If `open_document` returns an empty document, STOP and ask the user to open `design.pen` in Pencil and reconnect `/mcp` — do NOT write to an empty in-memory document, which has previously overwritten `design.pen` and wiped it.

- [ ] **Step 2: Design the overflow treatment**

Produce one treatment applied consistently to both screens. Candidates, in the order I'd try them:
1. **Wrap to two rows** — simplest, no interaction, but doubles the header height.
2. **Horizontal scroll with fade-out edges** — preserves height, familiar, but hides chips behind a scroll affordance.
3. **Show top N + "More" popover** — tidiest header, adds a click to reach a long-tail category.

Match the existing lunaris conventions already in the file (tokens, not hex; the dark theme override is `c:Mode:Dark`). Check both light and dark.

- [ ] **Step 3: Screenshot and verify**

`mcp__pencil__get_screenshot` of both edited screens, in **both** light and dark themes. Confirm tokens resolve and the row does not clip at the frame's width.

- [ ] **Step 4: Ask the user to save**

**Pencil does NOT auto-save.** Immediately after the edits, ask the user to press Ctrl/Cmd-S in the Pencil GUI. Do not wait for a flush and do not assume the edit persisted.

- [ ] **Step 5: Verify the diff is additive, then commit**

Run: `git diff --stat design.pen`
Expected: `design.pen` changed. If the diff shows a large *deletion*, the in-memory document was empty and has clobbered the file — recover with `git checkout HEAD -- design.pen` and start over.

```bash
git add design.pen
git -c commit.gpgsign=false commit -s -m "design: category filter chip-row overflow on Modules + Create Server

The 11-value category canon grows the filter row from ~4 chips to ~12,
which overflows on narrow viewports. <describe the chosen treatment>."
```

Include the Pencil node id(s) touched in the eventual PR description.

---

## Task 6: Web types + routes filter on any matching category

**Files:**
- Modify: `web/src/types.ts:293` (`GameTemplate.spec`), `:819` (`CatalogEntry`)
- Modify: `web/src/routes/Modules.tsx:16,61,71` and the chip row at `:159-169`
- Modify: `web/src/routes/CreateServer.tsx:21,389,399` and the chip row at `:424-426`
- Test: `web/src/routes/Modules.test.tsx:121-123`, `web/src/routes/CreateServer.test.tsx:250-283`

**Interfaces:**
- Consumes: `resolveCategories`, `categoryFilters` (Task 4); the chip-overflow treatment (Task 5); `"categories"` on the wire (Task 3).

- [ ] **Step 1: Update the existing tests to the new shape (they are the failing test)**

`web/src/routes/Modules.test.tsx:121-123` — replace the two fixture lines and extend the case to prove multi-category membership:

```tsx
  it("filters the catalog by game category", async () => {
    const minecraftEntry = { ...MINECRAFT, categories: ["Sandbox", "Survival"] };
    const valheimEntry = { ...VALHEIM_INSTALLED, game: "valheim", categories: ["Survival"] };
```

Keep the rest of the existing test body, then add a new case below it asserting that a module in two categories appears under **both** chips:

```tsx
  it("shows a multi-category module under each of its categories", async () => {
    const minecraftEntry = { ...MINECRAFT, categories: ["Sandbox", "Survival"] };
    const valheimEntry = { ...VALHEIM_INSTALLED, game: "valheim", categories: ["Survival"] };
    // Mock the catalog with both entries exactly as the sibling test above does,
    // then:
    await user.click(screen.getByRole("button", { name: "Sandbox" }));
    expect(screen.getByText(/Minecraft/)).toBeInTheDocument();
    expect(screen.queryByText(/Valheim/)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Survival" }));
    expect(screen.getByText(/Minecraft/)).toBeInTheDocument();
    expect(screen.getByText(/Valheim/)).toBeInTheDocument();
  });
```

Copy the catalog-mocking preamble verbatim from the sibling test rather than inventing one — `Modules.test.tsx` mocks the catalog through `msw`, and the exact handler shape must match.

`web/src/routes/CreateServer.test.tsx:256,266` — change `category: "Sandbox"` to `categories: ["Sandbox"]` and `category: "Survival"` to `categories: ["Survival"]`. The assertion at `:283` needs no change.

- [ ] **Step 2: Run to verify it fails**

Run: `cd web && npx tsc --noEmit`
Expected: FAIL — `Object literal may only specify known properties, and 'categories' does not exist in type ...` (plus the leftover `resolveCategory` import errors from Task 4).

- [ ] **Step 3: Update the types**

`web/src/types.ts:293`, inside `GameTemplate.spec`:
```ts
    categories?: string[];
```

`web/src/types.ts:819`, inside `CatalogEntry`:
```ts
  categories?: string[];
```

- [ ] **Step 4: Update `Modules.tsx`**

Line 16:
```tsx
import { resolveCategories, categoryFilters } from "@/lib/games";
```

Lines 60-63:
```tsx
  const catChips = useMemo(
    () => categoryFilters((data?.items ?? []).map((e) => resolveCategories(e.categories, e.game ?? ""))),
    [data],
  );
```

Line 71 — a module matches if **any** of its categories match the chip:
```tsx
    if (activeCat !== "all" && !resolveCategories(e.categories, e.game ?? "").includes(activeCat))
      return false;
```

Apply the Task 5 chip-overflow treatment to the chip row at lines 159-169.

- [ ] **Step 5: Update `CreateServer.tsx`**

Line 21:
```tsx
import { resolveCategories, categoryFilters } from "@/lib/games";
```

Lines 388-391:
```tsx
  const templateCategories = useMemo(
    () =>
      categoryFilters((data?.items ?? []).map((t) => resolveCategories(t.spec.categories, t.spec.game))),
    [data],
  );
```

Line 399:
```tsx
    if (
      activeCat !== "all" &&
      !resolveCategories(t.spec.categories, t.spec.game).includes(activeCat)
    )
      return false;
```

Apply the same chip-overflow treatment to the chip row at lines 424-426.

- [ ] **Step 6: Compile-check**

Run: `cd web && npx tsc --noEmit`
Expected: PASS, no errors.

Verify: `grep -rn "resolveCategory\b" web/src` returns nothing (the singular helper is fully gone).

- [ ] **Step 7: Commit Tasks 4 + 6 together**

```bash
git add web/src/lib/games.ts web/src/lib/games.test.ts web/src/types.ts \
        web/src/routes/Modules.tsx web/src/routes/CreateServer.tsx \
        web/src/routes/Modules.test.tsx web/src/routes/CreateServer.test.tsx
git -c commit.gpgsign=false commit -s -m "feat(web): filter the catalog on multiple categories per module

resolveCategory (singular) becomes resolveCategories, and categoryFilters
now flattens one list per module. A module matches a chip if ANY of its
categories match, so Minecraft appears under both Sandbox and Survival.
Adds games.test.ts, which did not exist."
```

---

## Task 7: Give the 16 official modules real categories

**Files:**
- Modify (in the `gameplane-module` repo, checked out at `modules/`): `modules/*/module.yaml` and `modules/*/template.yaml`, 16 of each.
- Modify (in THIS repo, as a separate commit): the `modules` submodule pointer.

**Interfaces:**
- Consumes: the `categories` list field (Tasks 1, 2).

**Two repos.** `modules/` is a git submodule pointing at `gameplane-module`. Commit there first, push, get it merged, then bump the pointer here. Note `modules/` currently has an **untracked `__pycache__/` directory** — do not commit it; leave it alone.

No module declares a category today (verified: `grep -rln "^category:" modules/*/module.yaml` is empty), so every edit is an addition.

- [ ] **Step 1: Add `categories` to each `module.yaml`**

Insert a `categories:` key after `game:` in each. Values (from the spec's §2 table):

| module.yaml | categories |
|---|---|
| `minecraft-java` | `[Sandbox, Survival, Building, Modded, Creative]` |
| `valheim` | `[Survival, Co-op, Building]` |
| `terraria` | `[Sandbox, Survival, Adventure, Modded]` |
| `factorio` | `[Simulation, Building, Sandbox, Modded, Co-op]` |
| `palworld` | `[Survival, Sandbox, Co-op]` |
| `7-days-to-die` | `[Survival, Horror, Co-op, Shooter]` |
| `rust` | `[Survival, Shooter, PvP]` |
| `dayz` | `[Survival, Shooter, PvP, Horror]` |
| `dont-starve-together` | `[Survival, Co-op]` |
| `garrys-mod` | `[Sandbox, Modded, Creative]` |
| `satisfactory` | `[Simulation, Building, Sandbox, Co-op]` |
| `enshrouded` | `[Survival, Co-op, Building, Adventure]` |
| `cs2` | `[Shooter, PvP]` |
| `project-zomboid` | `[Survival, Horror, Co-op]` |
| `v-rising` | `[Survival, PvP, Co-op]` |
| `ark-survival-ascended` | `[Survival, PvP, Co-op, Modded]` |

e.g. `modules/minecraft-java/module.yaml`:
```yaml
game: minecraft-java
categories: [Sandbox, Survival, Building, Modded, Creative]
summary: Vanilla / Paper / Spigot / ...
```

- [ ] **Step 2: Mirror into each `template.yaml`**

Each `template.yaml` has a `spec:` block. Add the SAME list under `spec.categories`, beside the existing `spec.icon` / `spec.accentColor`:

```yaml
spec:
  icon: icon.png
  categories: [Sandbox, Survival, Building, Modded, Creative]
  accentColor: "#5b9a3e"
```

`module.yaml` feeds `ModuleEntry.Categories` (the Modules catalog); `template.yaml` feeds `GameTemplateSpec.Categories` (the Create Server picker). Both are needed — they are read by different code paths. They must agree.

- [ ] **Step 3: Bump each module's version**

Each `module.yaml` has a `version:`. Bump the patch version (e.g. minecraft-java `2.7.1` → `2.7.2`) so the operator sees new content behind the tag.

- [ ] **Step 4: Commit in the module repo**

```bash
cd modules
git checkout -b feat/multi-category
git add '*/module.yaml' '*/template.yaml'
git -c commit.gpgsign=false commit -s -m "feat: declare categories on all 16 modules

Each module now declares its catalog categories explicitly rather than
relying on the dashboard's game-slug regex heuristic, and may declare
several — Minecraft is Sandbox, Survival, Building, Modded and Creative."
git push -u origin feat/multi-category
```

Open a PR in `gameplane-module`, let its CI run, merge, delete the branch.

- [ ] **Step 5: Bump the submodule pointer here**

Only after the module PR is merged:

```bash
cd /home/valgul/project/kubernetes-game-dashboard
git -C modules checkout main && git -C modules pull
git add modules
git -c commit.gpgsign=false commit -s -m "chore(modules): bump submodule — categories on all 16 modules"
```

---

## Task 8: Document the field and the canon

**Files:**
- Modify: `docs/module-authoring.md:44`, `:58-63`, `:350`, `:354-355`

- [ ] **Step 1: Update the `module.yaml` schema example (line 44)**

```yaml
categories: [Sandbox, Survival]            # optional, catalog groupings; a module may have several
```

- [ ] **Step 2: Replace the field-rules paragraph (lines 58-63)**

```markdown
- `categories` groups the module in the dashboard's Modules catalog. A module
  may belong to several at once — Minecraft is reasonably Sandbox, Survival,
  Building, Modded and Creative. The dashboard builds its filter chips from
  the distinct values across installed/available modules, so naming a new
  category here creates its filter button with no frontend change; a module
  appears under every chip it declares. Values are free-form, but the official
  modules stick to this canon:

  **Survival · Sandbox · Shooter · Simulation · Building · Adventure · Horror ·
  Co-op · PvP · Modded · Creative**

  Chips dedupe case-insensitively, so `Survival` and `survival` do not both
  appear. When `categories` is omitted, the dashboard falls back to a
  best-effort heuristic on the `game` slug, and finally to "Other".

  The singular `category: Sandbox` is still accepted for bundles authored
  before this field became a list; it is normalized to `[Sandbox]` on parse.
  New modules should use `categories`.
```

- [ ] **Step 3: Update the GameTemplate branding section (lines 350, 354-355)**

```yaml
spec:
  icon: icon.png            # bundle file or URL/data-URI shown in the catalog
  categories: [Sandbox]     # optional, catalog groupings (mirrors module.yaml)
  accentColor: "#5b9a3e"    # CSS hex; tints this game's icon + accents
```

```markdown
`categories` mirrors the `module.yaml` field and drives the Create-Server
template picker's category chips. Optional; the same fallback and legacy-scalar
handling apply. The two lists should agree — `module.yaml` feeds the Modules
catalog, `template.yaml` feeds the Create-Server picker, and they are read by
different code paths.
```

- [ ] **Step 4: Commit**

```bash
git add docs/module-authoring.md
git -c commit.gpgsign=false commit -s -m "docs: document categories as a list + the canonical vocabulary"
```

---

## Task 9: Push, open the PR, watch CI

- [ ] **Step 1: Push the branch**

```bash
git push -u origin feat/multi-category-modules
```

- [ ] **Step 2: Open the PR**

```bash
gh pr create --title "feat: modules declare multiple categories" --body "$(cat <<'EOF'
Phase 1 of docs/superpowers/specs/2026-07-14-console-protocols-categories-actions-design.md.

`category: string` becomes `categories: []string` across the CRDs, the bundle
parser, the API catalog DTO, and the dashboard. A module may now belong to
several categories at once and appears under each one's filter chip.

- Legacy scalar `category:` in a bundle is normalized to a one-element list, so
  third-party bundles keep working.
- The API unions categories across the ModuleSources serving a module
  (case-insensitive dedupe) rather than taking the first non-empty one.
- All 16 official modules now declare real categories instead of relying on the
  dashboard's game-slug regex heuristic.
- The filter chip row grows from ~4 chips to ~12, so it gained an overflow
  treatment — designed in `design.pen` first (Pencil node ids: <fill in>).

Unrelated `category` concepts (mod-registry search facets in
`api/internal/registry/` and `registry-browser.tsx`) are deliberately untouched.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

- [ ] **Step 3: Watch CI to green**

```bash
gh run watch
```

CI is the source of truth. Fix failures with follow-up commits on the branch — never `--amend` a pushed commit, never `--no-verify`.

Expect the two envtest jobs (`go operator`, `go api`) to be the ones that catch a bad CRD.

- [ ] **Step 4: Merge once ALL checks are green, then delete the branch**

`main` has a ruleset that blocks a plain merge, so use `--admin` directly:

```bash
gh pr merge --admin --merge
git push origin --delete feat/multi-category-modules
git branch -d feat/multi-category-modules
```

Wait for **all** checks including the flaky e2e ones. Never merge on substantive-green-only.

---

## Self-Review

**Spec coverage (§2 of the design doc):**

| Spec requirement | Task |
|---|---|
| `GameTemplateSpec` → `Categories []string`, MaxItems=8, MaxLength=32 | 1 |
| `ModuleEntry` → `Categories []string` | 1 |
| `modsrc.Metadata.Categories` + legacy scalar accepted | 2 |
| `modsrc/{oci,dir,upload}.go` carry the list | 2 |
| `CatalogEntry.Categories`; merge becomes a case-insensitive union | 3 |
| `web/src/types.ts` `categories?: string[]` | 6 |
| `resolveCategories` fallback to `[gameCategory(game)]` | 4 |
| Chips from the flattened union; **any**-match filter | 4, 6 |
| Free-form values + canon in `docs/module-authoring.md` | 8 |
| All 16 modules' category assignments | 7 |
| `make generate && make manifests`, committed with the source | 1 |
| Chip-overflow design pass before React | 5 |
| Regex heuristic retained for undeclared third-party modules | 4 |

No gaps.

**Correction to the spec:** §2 states "A module card that previously showed at most one category chip may now show up to five." That is wrong — module **cards render no categories at all** today; `resolveCategory` is used only in filter logic. The real visual change is the **filter chip row** growing from ~4 chips to ~12. Task 5 is scoped to that. Adding category chips to the cards themselves is deliberately **not** in this plan (YAGNI — it is a separate enhancement, not a requirement of making categories plural).

**Known-broken windows (deliberate):** Task 1 alone leaves the operator uncompilable (modsrc still references `.Category`) and Task 4 alone leaves the web uncompilable (routes still import `resolveCategory`). Both are commit-paired — 1+2 commit together, 4+6 commit together — so no commit is ever a broken state (CLAUDE.md rule 11).
