// Category grouping for the module catalog + Create-Server template picker.
// Modules may declare their own category (GameTemplate.spec.category /
// CatalogEntry.category). When a module declares none we fall back to a
// best-effort heuristic on the game slug, so existing modules still group
// sensibly. The dashboard builds its filter chips from the categories
// actually present rather than a fixed list.

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

// resolveCategory returns the module's declared category, or the heuristic
// fallback when it declares none.
export function resolveCategory(explicit: string | undefined, game: string): string {
  const c = explicit?.trim();
  return c ? c : gameCategory(game);
}

// categoryFilters builds the ordered chip list from the resolved categories of
// the given items: "all" first, then the distinct named categories sorted
// alphabetically, then "Other" last (only if present).
export function categoryFilters(categories: string[]): string[] {
  const present = new Set(categories);
  const named = [...present].filter((c) => c !== OTHER_CATEGORY).sort();
  const tail = present.has(OTHER_CATEGORY) ? [OTHER_CATEGORY] : [];
  return ["all", ...named, ...tail];
}
