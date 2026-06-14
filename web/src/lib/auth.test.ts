import { describe, it, expect } from "vitest";
import { hasRole, can } from "./auth";
import type { User } from "@/types";

const u = (role: User["role"]): User => ({
  id: 1,
  username: "x",
  displayName: "X",
  email: "x@x",
  role,
});

const withPerms = (perms: Record<string, string[]>): User => ({
  ...u("custom"),
  permissions: perms,
});

describe("hasRole", () => {
  it("returns false when user is undefined", () => {
    expect(hasRole(undefined, ["admin"])).toBe(false);
  });
  it("matches an allowed role", () => {
    expect(hasRole(u("admin"), ["admin", "operator"])).toBe(true);
  });
  it("rejects a non-allowed role", () => {
    expect(hasRole(u("viewer"), ["admin", "operator"])).toBe(false);
  });
});

describe("can", () => {
  it("is false for an undefined user or no permissions", () => {
    expect(can(undefined, "servers:read")).toBe(false);
    expect(can(u("viewer"), "servers:read")).toBe(false);
  });
  it("grants any permission to a cluster-wide wildcard", () => {
    const admin = withPerms({ "*": ["*"] });
    expect(can(admin, "users:manage")).toBe(true);
    expect(can(admin, "anything:at:all")).toBe(true);
  });
  it("grants a specific cluster-wide permission", () => {
    const op = withPerms({ "*": ["servers:write", "servers:read"] });
    expect(can(op, "servers:write")).toBe(true);
    expect(can(op, "users:manage")).toBe(false);
  });
  it("honors a namespace-scoped grant only in that namespace", () => {
    const nsOp = withPerms({ "team-a": ["servers:write"] });
    expect(can(nsOp, "servers:write", "team-a")).toBe(true);
    expect(can(nsOp, "servers:write", "team-b")).toBe(false);
    // No cluster-wide grant: the bare check (no ns) is false.
    expect(can(nsOp, "servers:write")).toBe(false);
  });
});
