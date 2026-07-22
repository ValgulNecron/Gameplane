import { describe, it, expect } from "vitest";
import { countByState, phaseGroups } from "@/lib/servers";
import { makeServer } from "@/test/factories";

describe("countByState", () => {
  it("counts running and stopped (incl. suspended/failed) and sums players", () => {
    const c = countByState([
      makeServer({ status: { phase: "Running", agent: { playersOnline: 3, playersMax: 20 } } }),
      makeServer({ status: { phase: "Suspended", agent: { playersOnline: 0, playersMax: 10 } } }),
      makeServer({ status: { phase: "Failed" } }),
      makeServer({ status: { phase: "Stopped" } }),
    ]);
    expect(c.running).toBe(1);
    expect(c.stopped).toBe(3); // Suspended + Failed + Stopped
    expect(c.players).toBe(3);
    expect(c.playersMax).toBe(30);
  });

  it("clamps unknown/sentinel player counts so the total never goes negative", () => {
    const c = countByState([
      makeServer({ status: { phase: "Running", agent: { playersOnline: -1 } } }),
      makeServer({ status: { phase: "Running", agent: { playersOnline: null } } }),
    ]);
    expect(c.players).toBe(0);
  });

  it("treats a missing phase as Pending (neither running nor stopped)", () => {
    const c = countByState([makeServer({ status: {} })]);
    expect(c.running).toBe(0);
    expect(c.stopped).toBe(0);
  });
});

describe("phaseGroups", () => {
  it("breaks Failed out of stopped and buckets the rest", () => {
    const g = phaseGroups([
      makeServer({ status: { phase: "Running" } }),
      makeServer({ status: { phase: "Stopped" } }),
      makeServer({ status: { phase: "Suspended" } }),
      makeServer({ status: { phase: "Failed" } }),
      makeServer({ status: { phase: "Starting" } }),
    ]);
    expect(g.total).toBe(5);
    expect(g.running).toBe(1);
    expect(g.stopped).toBe(2); // Stopped + Suspended
    expect(g.failed).toBe(1);
    expect(g.other).toBe(1); // Starting
  });

  it("flags Failed servers and stale agents as needing attention", () => {
    const g = phaseGroups([
      makeServer({ metadata: { name: "ok" }, status: { phase: "Running" } }),
      makeServer({ metadata: { name: "broken" }, status: { phase: "Failed" } }),
      makeServer({ metadata: { name: "stale" }, status: { phase: "Running", agent: { stale: true } } }),
    ]);
    const names = g.attention.map((s) => s.metadata.name).sort();
    expect(names).toEqual(["broken", "stale"]);
  });

  it("does not flag a stopped/asleep server's stale agent — no replicas means no heartbeat by design", () => {
    const g = phaseGroups([
      makeServer({ metadata: { name: "stopped" }, status: { phase: "Stopped", agent: { stale: true } } }),
      makeServer({
        metadata: { name: "asleep" },
        status: { phase: "Suspended", agent: { stale: true }, idle: { asleep: true } },
      }),
    ]);
    expect(g.attention).toHaveLength(0);
  });
});
