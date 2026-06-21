// Game categorization for UI filtering. GameTemplate/CatalogEntry carry a
// `game` slug but no category field, so this is a best-effort heuristic used
// by the Create-Server template picker and the Modules catalog filter.
// Unknown games map to "Other" and only appear under "All".

export const GAME_CATEGORIES = ["all", "Survival", "Sandbox", "Shooter"] as const;
export type GameCategory = (typeof GAME_CATEGORIES)[number];

export function gameCategory(game: string): string {
  const g = game.toLowerCase();
  if (/valheim|palworld|ark|rust|conan|7.?days|dayz/.test(g)) return "Survival";
  if (/minecraft|terraria|factorio|satisfactory|stardew/.test(g)) return "Sandbox";
  if (/cs2|cs.?go|csgo|tf2|valorant|insurgency|squad|left4dead/.test(g)) return "Shooter";
  return "Other";
}
