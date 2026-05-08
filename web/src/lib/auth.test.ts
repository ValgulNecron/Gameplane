import { describe, it, expect } from "vitest";
import { hasRole } from "./auth";
import type { User } from "@/types";

const u = (role: User["role"]): User => ({
  id: 1,
  username: "x",
  displayName: "X",
  email: "x@x",
  role,
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
