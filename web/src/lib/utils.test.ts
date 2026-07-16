import { describe, it, expect, beforeEach, vi, afterEach } from "vitest";
import {
  cn,
  describeStorageProvisioned,
  formatBytes,
  formatCores,
  formatRelative,
  formatRelativeFuture,
  formatUptime,
  parseQuantityToBytes,
} from "./utils";

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

describe("formatRelativeFuture", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-07T12:00:00Z"));
  });
  afterEach(() => vi.useRealTimers());

  it("returns em dash for empty", () => {
    expect(formatRelativeFuture()).toBe("—");
  });
  it("returns input on invalid date", () => {
    expect(formatRelativeFuture("nope")).toBe("nope");
  });
  it("reads past/now as due", () => {
    expect(formatRelativeFuture("2026-05-07T11:59:00Z")).toBe("due");
  });
  it("minutes ahead", () => {
    expect(formatRelativeFuture("2026-05-07T12:45:00Z")).toBe("in 45m");
  });
  it("hours ahead", () => {
    expect(formatRelativeFuture("2026-05-07T14:00:00Z")).toBe("in 2h");
  });
  it("days ahead", () => {
    expect(formatRelativeFuture("2026-05-09T12:00:00Z")).toBe("in 2d");
  });
});

describe("parseQuantityToBytes", () => {
  it("returns 0 for empty/unparseable", () => {
    expect(parseQuantityToBytes()).toBe(0);
    expect(parseQuantityToBytes("")).toBe(0);
    expect(parseQuantityToBytes("garbage")).toBe(0);
  });
  it("parses plain byte counts", () => {
    expect(parseQuantityToBytes("5368709120")).toBe(5368709120);
  });
  it("parses binary-SI suffixes", () => {
    expect(parseQuantityToBytes("5Gi")).toBe(5 * 1024 ** 3);
    expect(parseQuantityToBytes("512Mi")).toBe(512 * 1024 ** 2);
    expect(parseQuantityToBytes("38.2 GiB")).toBeCloseTo(38.2 * 1024 ** 3);
  });
  it("parses decimal-SI suffixes", () => {
    expect(parseQuantityToBytes("5G")).toBe(5 * 1000 ** 3);
    expect(parseQuantityToBytes("500MB")).toBe(500 * 1000 ** 2);
  });
  it("treats a bare B as bytes", () => {
    expect(parseQuantityToBytes("1024 B")).toBe(1024);
  });
  it("returns 0 for an unknown unit", () => {
    expect(parseQuantityToBytes("5X")).toBe(0);
  });
});

describe("formatCores", () => {
  it("renders whole numbers without decimals", () => {
    expect(formatCores(4)).toBe("4");
    expect(formatCores(0)).toBe("0");
  });
  it("renders fractional cores to 2 decimals", () => {
    expect(formatCores(0.646)).toBe("0.65");
  });
  it("trims floating-point noise from summed sub-core readings", () => {
    // 0.646 + 0.646 in IEEE754 is 1.2919999999999998, not 1.292.
    expect(formatCores(0.646 + 0.646)).toBe("1.29");
  });
});

describe("describeStorageProvisioned", () => {
  it("reports provisioned-of-physical when under capacity", () => {
    const r = describeStorageProvisioned(250_000_000_000, 1_000_000_000_000);
    expect(r.overcommitted).toBe(false);
    expect(r.subText).toContain("physical");
    expect(r.subText).not.toContain("overcommitted");
  });
  it("flags overcommit when provisioned exceeds physical capacity", () => {
    // The reported bug: 102 GB provisioned vs 86 GB physical (networked
    // storage doesn't come off node disk).
    const r = describeStorageProvisioned(102_000_000_000, 86_000_000_000);
    expect(r.overcommitted).toBe(true);
    expect(r.subText).toContain("overcommitted");
  });
  it("omits subText when total is unknown", () => {
    const r = describeStorageProvisioned(5_000, undefined);
    expect(r.subText).toBeUndefined();
    expect(r.overcommitted).toBe(false);
  });
  it("defaults used to 0 when undefined", () => {
    const r = describeStorageProvisioned(undefined, 1_000_000_000);
    expect(r.valueText).toBe(formatBytes(0));
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
