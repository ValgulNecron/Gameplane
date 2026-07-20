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

// matchesCategory decides whether a module belongs under a given filter chip.
// Comparison is case-insensitive to agree with categoryFilters, which collapses
// "Survival" and "survival" into a single chip — a case-sensitive match here
// would hide every module that spelled its category differently from the first
// one seen.
export function matchesCategory(
  explicit: string[] | undefined,
  game: string,
  chip: string,
): boolean {
  if (chip === "all") return true;
  const want = chip.toLowerCase();
  return resolveCategories(explicit, game).some((c) => c.toLowerCase() === want);
}

// matchesAnyCategory returns true when a module matches ANY of the selected
// category chips. An empty selection means "no filter" (show everything).
// Comparison is case-insensitive to agree with categoryFilters/matchesCategory.
export function matchesAnyCategory(
  explicit: string[] | undefined,
  game: string,
  selected: Set<string>,
): boolean {
  if (selected.size === 0) return true;
  const wants = new Set([...selected].map((c) => c.toLowerCase()));
  return resolveCategories(explicit, game).some((c) => wants.has(c.toLowerCase()));
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
  const otherLower = OTHER_CATEGORY.toLowerCase();
  const named = present
    .filter((c) => c.toLowerCase() !== otherLower)
    .sort((a, b) => a.localeCompare(b));
  const tail = present.some((c) => c.toLowerCase() === otherLower) ? [OTHER_CATEGORY] : [];
  return ["all", ...named, ...tail];
}
