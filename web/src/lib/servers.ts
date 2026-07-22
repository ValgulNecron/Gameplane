// Fleet aggregation shared by the Servers list (/servers) and the
// Dashboard overview (/). Kept in lib so the counting logic is tested once
// and reused, rather than living inside a route component.

import type { GameServer, GameServerPhase } from "@/types";

export interface ServerCounts {
  running: number;
  stopped: number;
  players: number;
  playersMax: number;
}

// countByState drives the Servers page's tab counts and stat cards.
// "stopped" folds in Suspended and Failed (anything not actively running).
// Player totals clamp at zero: null/undefined means "unknown" and a legacy
// -1 sentinel must never drag the sum below zero.
export function countByState(items: GameServer[]): ServerCounts {
  const result: ServerCounts = { running: 0, stopped: 0, players: 0, playersMax: 0 };
  for (const gs of items) {
    const p: GameServerPhase = gs.status?.phase ?? "Pending";
    if (p === "Running") result.running += 1;
    if (p === "Stopped" || p === "Suspended" || p === "Failed") result.stopped += 1;
    result.players += Math.max(0, gs.status?.agent?.playersOnline ?? 0);
    result.playersMax += gs.status?.agent?.playersMax ?? 0;
  }
  return result;
}

// Phase buckets for the dashboard's fleet-status widget. Failed is broken
// out from stopped (for the distribution bar), and `attention` collects the
// servers an operator should look at — Failed phase or a stale agent
// heartbeat (status.agent.stale set by the API when the heartbeat is old).
export interface PhaseGroups {
  total: number;
  running: number;
  stopped: number; // Stopped or Suspended
  failed: number;
  other: number; // Pending/Starting/Stopping/unknown
  attention: GameServer[];
}

export function phaseGroups(items: GameServer[]): PhaseGroups {
  const g: PhaseGroups = {
    total: items.length,
    running: 0,
    stopped: 0,
    failed: 0,
    other: 0,
    attention: [],
  };
  for (const gs of items) {
    const p: GameServerPhase = gs.status?.phase ?? "Pending";
    if (p === "Running") g.running += 1;
    else if (p === "Failed") g.failed += 1;
    else if (p === "Stopped" || p === "Suspended") g.stopped += 1;
    else g.other += 1;
    // An expected-down server (Stopped, or Suspended — which also covers
    // idle auto-sleep) has zero replicas and therefore no heartbeat by
    // definition; a stale agent there is not a fault and must not flood
    // "Needs attention" every time a server sleeps overnight.
    const down = p === "Stopped" || p === "Suspended";
    if (p === "Failed" || (gs.status?.agent?.stale && !down)) g.attention.push(gs);
  }
  return g;
}
