import type { GameTemplate } from "@/types";

export type ConsoleMode = "rcon" | "pty" | "none";

// resolveConsoleMode mirrors operator.controller.EffectiveConsoleMode so
// the dashboard agrees with the server on which transport to open (and
// whether to show the Console tab at all). Keep in sync with the operator
// helper if its rules change.
export function resolveConsoleMode(tmpl: GameTemplate | undefined): ConsoleMode {
  if (!tmpl) return "rcon";
  if (tmpl.spec.consoleMode) return tmpl.spec.consoleMode;
  const proto = tmpl.spec.rcon?.protocol;
  if (proto && proto !== "none") return "rcon";
  return "none";
}

// rconAvailable reports whether the template exposes a live RCON
// connection — required for module actions and status metrics, which run
// over RCON.
export function rconAvailable(tmpl: GameTemplate | undefined): boolean {
  const proto = tmpl?.spec.rcon?.protocol;
  return !!proto && proto !== "none";
}
