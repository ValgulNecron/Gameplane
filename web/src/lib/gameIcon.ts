// Pure helpers for GameIcon's per-template abbreviation (Q7), kept free of
// React so they're easy to unit test and so any screen holding a full
// GameTemplate[] (from Templates.list()) can compute codes once and hand
// them down, rather than GameIcon guessing from a single name in isolation.
import type { GameTemplate } from "@/types";

export interface GameAbbreviationInput {
  /** The identifier the tile is keyed by — typically GameTemplate.metadata.name. */
  name: string;
  creationTimestamp?: string;
}

// assignGameCodes returns a Map from template name -> a unique 2-letter
// uppercase code (or, once every letter-pair for a name is taken, a
// first-letter+digit code). Rule (Q7, decisions.md):
//   first letter of the name + the first subsequent letter (in order of
//   appearance) that isn't already claimed by an earlier-installed
//   template (creationTimestamp ascending — the oldest template keeps its
//   preferred code); if every pair for a name is exhausted, fall back to
//   first letter + digit (2, 3, …).
// Non-letter characters (hyphens, digits) are ignored when picking letters.
// Templates without a creationTimestamp are treated as installed after
// every template that has one (so a template with real provenance always
// keeps priority over one with unknown install order), and ties/missing
// timestamps break by input array order.
export function assignGameCodes(templates: GameAbbreviationInput[]): Map<string, string> {
  const sorted = templates
    .map((t, index) => ({ ...t, index }))
    .sort((a, b) => {
      const ta = a.creationTimestamp ? Date.parse(a.creationTimestamp) : Number.POSITIVE_INFINITY;
      const tb = b.creationTimestamp ? Date.parse(b.creationTimestamp) : Number.POSITIVE_INFINITY;
      if (ta !== tb) return ta - tb;
      return a.index - b.index; // stable tiebreaker for equal/missing timestamps
    });

  const used = new Set<string>();
  const codes = new Map<string, string>();

  for (const t of sorted) {
    const letters = t.name.replace(/[^a-zA-Z]/g, "");
    const first = (letters[0] ?? "?").toUpperCase();
    let code: string | undefined;

    for (let i = 1; i < letters.length; i++) {
      const candidate = first + letters[i].toUpperCase();
      if (!used.has(candidate)) {
        code = candidate;
        break;
      }
    }

    if (!code) {
      let digit = 2;
      let candidate = `${first}${digit}`;
      while (used.has(candidate)) {
        digit += 1;
        candidate = `${first}${digit}`;
      }
      code = candidate;
    }

    used.add(code);
    codes.set(t.name, code);
  }

  return codes;
}

// Convenience wrapper for callers holding GameTemplate[] (e.g.
// Templates.list().items) rather than the bare {name, creationTimestamp}
// shape assignGameCodes expects.
export function assignGameCodesForTemplates(templates: GameTemplate[]): Map<string, string> {
  return assignGameCodes(
    templates.map((t) => ({
      name: t.metadata.name,
      creationTimestamp: t.metadata.creationTimestamp,
    })),
  );
}
