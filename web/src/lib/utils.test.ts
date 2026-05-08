import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import { cn, formatBytes, formatRelative, formatUptime } from "./utils";

describe("cn", () => {
  it("merges tailwind classes deduplicating conflicts", () => {
    expect(cn("p-2", "p-4")).toBe("p-4");
  });
  it("filters falsy values", () => {
    expect(cn("a", false, undefined, "b")).toBe("a b");
  });
});

describe("formatBytes", () => {
  it("returns 0 B for zero", () => {
    expect(formatBytes(0)).toBe("0 B");
  });
  it("uses bytes for small values", () => {
    expect(formatBytes(500)).toBe("500 B");
  });
  it("uses KB", () => {
    expect(formatBytes(2048)).toBe("2.0 KB");
  });
  it("uses MB", () => {
    expect(formatBytes(5 * 1024 * 1024)).toBe("5.0 MB");
  });
  it("hides decimals for >=10 of a unit", () => {
    expect(formatBytes(20 * 1024 * 1024)).toBe("20 MB");
  });
  it("caps at PB", () => {
    const huge = 1024 ** 6 * 3;
    expect(formatBytes(huge)).toContain("PB");
  });
});

describe("formatRelative", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-07T12:00:00Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("returns em dash for empty", () => {
    expect(formatRelative()).toBe("—");
    expect(formatRelative("")).toBe("—");
  });
  it("returns input on invalid date", () => {
    expect(formatRelative("not-a-date")).toBe("not-a-date");
  });
  it("seconds", () => {
    expect(formatRelative("2026-05-07T11:59:30Z")).toBe("30s ago");
  });
  it("minutes", () => {
    expect(formatRelative("2026-05-07T11:55:00Z")).toBe("5m ago");
  });
  it("hours", () => {
    expect(formatRelative("2026-05-07T09:00:00Z")).toBe("3h ago");
  });
  it("days", () => {
    expect(formatRelative("2026-05-04T12:00:00Z")).toBe("3d ago");
  });
});

describe("formatUptime", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-07T12:00:00Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("returns dash for empty", () => {
    expect(formatUptime()).toBe("—");
  });
  it("returns dash for invalid", () => {
    expect(formatUptime("nope")).toBe("—");
  });
  it("minutes only", () => {
    expect(formatUptime("2026-05-07T11:30:00Z")).toBe("30m");
  });
  it("hours and minutes", () => {
    expect(formatUptime("2026-05-07T09:23:00Z")).toBe("2h 37m");
  });
  it("days and hours", () => {
    expect(formatUptime("2026-05-05T09:00:00Z")).toBe("2d 3h");
  });
});
