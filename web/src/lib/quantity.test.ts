import { describe, expect, it } from "vitest";
import {
  type CpuAmount,
  type MemAmount,
  convertCpu,
  convertMem,
  cpuCores,
  formatCpuQuantity,
  formatMemQuantity,
  memBytes,
  parseCpuQuantity,
  parseMemQuantity,
} from "./quantity";

describe("parseCpuQuantity", () => {
  it("parses whole cores", () => {
    expect(parseCpuQuantity("2")).toEqual({ value: 2, unit: "cores" });
    expect(parseCpuQuantity("1")).toEqual({ value: 1, unit: "cores" });
  });

  it("parses fractional cores", () => {
    expect(parseCpuQuantity("0.5")).toEqual({ value: 0.5, unit: "cores" });
    expect(parseCpuQuantity("1.5")).toEqual({ value: 1.5, unit: "cores" });
  });

  it("parses millicores", () => {
    expect(parseCpuQuantity("500m")).toEqual({ value: 500, unit: "m" });
    expect(parseCpuQuantity("1000m")).toEqual({ value: 1000, unit: "m" });
  });

  it("returns null for empty or undefined", () => {
    expect(parseCpuQuantity("")).toBeNull();
    expect(parseCpuQuantity(undefined)).toBeNull();
  });

  it("returns null for invalid quantities", () => {
    expect(parseCpuQuantity("abc")).toBeNull();
    expect(parseCpuQuantity("50G")).toBeNull();
    expect(parseCpuQuantity("50Gi")).toBeNull();
    expect(parseCpuQuantity("-5")).toBeNull();
  });
});

describe("parseMemQuantity", () => {
  it("parses binary Kubernetes units", () => {
    expect(parseMemQuantity("4Gi")).toEqual({ value: 4, unit: "Gi" });
    expect(parseMemQuantity("512Mi")).toEqual({ value: 512, unit: "Mi" });
    expect(parseMemQuantity("1Ti")).toEqual({ value: 1, unit: "Ti" });
    expect(parseMemQuantity("256Ki")).toEqual({ value: 256, unit: "Ki" });
  });

  it("parses fractional binary units", () => {
    expect(parseMemQuantity("0.5Gi")).toEqual({ value: 0.5, unit: "Gi" });
    expect(parseMemQuantity("1.5Gi")).toEqual({ value: 1.5, unit: "Gi" });
  });

  it("normalizes decimal SI units to binary", () => {
    // 4G = 4*10^9 bytes ≈ 3.73 Gi, should pick Gi as the natural unit
    const result = parseMemQuantity("4G");
    expect(result?.unit).toBe("Gi");
    expect(result?.value).toBeCloseTo(3.725290298461914, 1);
  });

  it("normalizes plain byte counts to the best binary unit", () => {
    // 1048576 bytes = 1 Mi
    const result = parseMemQuantity("1048576");
    expect(result).toEqual({ value: 1, unit: "Mi" });
  });

  it("prefers larger units when possible", () => {
    // 4096 Mi = 4 Gi; should return Gi
    const result = parseMemQuantity("4096Mi");
    expect(result).toEqual({ value: 4, unit: "Gi" });
  });

  it("handles quantities smaller than 1 Ki", () => {
    // 512 bytes; Ki is 1024, so this is < 1 Ki
    const result = parseMemQuantity("512");
    expect(result?.unit).toBe("Ki");
    expect(result?.value).toBeCloseTo(0.5, 5);
  });

  it("returns null for empty or undefined", () => {
    expect(parseMemQuantity("")).toBeNull();
    expect(parseMemQuantity(undefined)).toBeNull();
  });

  it("returns null for invalid quantities", () => {
    expect(parseMemQuantity("abc")).toBeNull();
    expect(parseMemQuantity("5Xi")).toBeNull(); // unknown suffix
    expect(parseMemQuantity("-5Gi")).toBeNull();
  });
});

describe("formatCpuQuantity", () => {
  it("formats whole cores as plain integers", () => {
    expect(formatCpuQuantity({ value: 2, unit: "cores" })).toBe("2");
    expect(formatCpuQuantity({ value: 1, unit: "cores" })).toBe("1");
  });

  it("formats fractional cores as millicores", () => {
    expect(formatCpuQuantity({ value: 0.5, unit: "cores" })).toBe("500m");
    expect(formatCpuQuantity({ value: 1.5, unit: "cores" })).toBe("1500m");
  });

  it("formats millicores as-is", () => {
    expect(formatCpuQuantity({ value: 500, unit: "m" })).toBe("500m");
    expect(formatCpuQuantity({ value: 1000, unit: "m" })).toBe("1000m");
  });

  it("rounds fractional millicores", () => {
    expect(formatCpuQuantity({ value: 500.4, unit: "m" })).toBe("500m");
    expect(formatCpuQuantity({ value: 500.6, unit: "m" })).toBe("501m");
  });
});

describe("formatMemQuantity", () => {
  it("formats binary units correctly", () => {
    expect(formatMemQuantity({ value: 4, unit: "Gi" })).toBe("4Gi");
    expect(formatMemQuantity({ value: 512, unit: "Mi" })).toBe("512Mi");
    expect(formatMemQuantity({ value: 1, unit: "Ti" })).toBe("1Ti");
  });

  it("steps fractional values down to a whole smaller unit (lossless)", () => {
    expect(formatMemQuantity({ value: 1.5, unit: "Gi" })).toBe("1536Mi");
    expect(formatMemQuantity({ value: 0.5, unit: "Mi" })).toBe("512Ki");
    expect(formatMemQuantity({ value: 2.5, unit: "Gi" })).toBe("2560Mi");
  });
});

describe("convertCpu", () => {
  it("converts cores to millicores", () => {
    expect(convertCpu({ value: 2, unit: "cores" }, "m")).toEqual({
      value: 2000,
      unit: "m",
    });
    expect(convertCpu({ value: 0.5, unit: "cores" }, "m")).toEqual({
      value: 500,
      unit: "m",
    });
  });

  it("converts millicores to cores", () => {
    expect(convertCpu({ value: 1000, unit: "m" }, "cores")).toEqual({
      value: 1,
      unit: "cores",
    });
    expect(convertCpu({ value: 500, unit: "m" }, "cores")).toEqual({
      value: 0.5,
      unit: "cores",
    });
  });

  it("returns the same unit unchanged", () => {
    expect(convertCpu({ value: 2, unit: "cores" }, "cores")).toEqual({
      value: 2,
      unit: "cores",
    });
    expect(convertCpu({ value: 500, unit: "m" }, "m")).toEqual({
      value: 500,
      unit: "m",
    });
  });
});

describe("convertMem", () => {
  it("converts Gi to Mi", () => {
    expect(convertMem({ value: 4, unit: "Gi" }, "Mi")).toEqual({
      value: 4096,
      unit: "Mi",
    });
  });

  it("converts Mi to Gi", () => {
    expect(convertMem({ value: 4096, unit: "Mi" }, "Gi")).toEqual({
      value: 4,
      unit: "Gi",
    });
  });

  it("converts Gi to Ki", () => {
    expect(convertMem({ value: 1, unit: "Gi" }, "Ki")).toEqual({
      value: 1048576,
      unit: "Ki",
    });
  });

  it("converts between any units", () => {
    expect(convertMem({ value: 1, unit: "Ti" }, "Mi")).toEqual({
      value: 1048576,
      unit: "Mi",
    });
  });

  it("returns the same unit unchanged", () => {
    expect(convertMem({ value: 4, unit: "Gi" }, "Gi")).toEqual({
      value: 4,
      unit: "Gi",
    });
  });

  it("handles fractional conversions", () => {
    const result = convertMem({ value: 512, unit: "Mi" }, "Gi");
    expect(result.value).toBeCloseTo(0.5, 10);
    expect(result.unit).toBe("Gi");
  });
});

describe("cpuCores", () => {
  it("returns the core count from cores unit", () => {
    expect(cpuCores({ value: 2, unit: "cores" })).toBe(2);
    expect(cpuCores({ value: 0.5, unit: "cores" })).toBe(0.5);
  });

  it("converts millicores to cores", () => {
    expect(cpuCores({ value: 1000, unit: "m" })).toBe(1);
    expect(cpuCores({ value: 500, unit: "m" })).toBe(0.5);
  });
});

describe("memBytes", () => {
  it("returns the byte count from binary units", () => {
    expect(memBytes({ value: 4, unit: "Gi" })).toBe(4 * 1024 ** 3);
    expect(memBytes({ value: 512, unit: "Mi" })).toBe(512 * 1024 ** 2);
    expect(memBytes({ value: 1, unit: "Ti" })).toBe(1024 ** 4);
  });

  it("handles fractional values", () => {
    expect(memBytes({ value: 0.5, unit: "Gi" })).toBe(0.5 * 1024 ** 3);
  });
});

describe("round-trip preservation", () => {
  it("preserves CPU cores in round-trip", () => {
    const original = "2";
    const parsed = parseCpuQuantity(original);
    expect(parsed).not.toBeNull();
    const formatted = formatCpuQuantity(parsed!);
    expect(formatted).toBe(original);
  });

  it("preserves CPU millicores in round-trip", () => {
    const original = "500m";
    const parsed = parseCpuQuantity(original);
    expect(parsed).not.toBeNull();
    const formatted = formatCpuQuantity(parsed!);
    expect(formatted).toBe(original);
  });

  it("preserves memory Gi in round-trip", () => {
    const original = "4Gi";
    const parsed = parseMemQuantity(original);
    expect(parsed).not.toBeNull();
    const formatted = formatMemQuantity(parsed!);
    expect(formatted).toBe(original);
  });

  it("preserves memory Mi in round-trip", () => {
    const original = "512Mi";
    const parsed = parseMemQuantity(original);
    expect(parsed).not.toBeNull();
    const formatted = formatMemQuantity(parsed!);
    expect(formatted).toBe(original);
  });

  it("round-trips through unit conversion", () => {
    const original: CpuAmount = { value: 2, unit: "cores" };
    const converted = convertCpu(original, "m");
    const backConverted = convertCpu(converted, "cores");
    expect(backConverted.value).toBe(original.value);
    expect(backConverted.unit).toBe(original.unit);
  });

  it("round-trips memory through unit conversion", () => {
    const original: MemAmount = { value: 4, unit: "Gi" };
    const converted = convertMem(original, "Mi");
    const backConverted = convertMem(converted, "Gi");
    expect(backConverted.value).toBe(original.value);
    expect(backConverted.unit).toBe(original.unit);
  });
});

describe("integration with Resources.tsx patterns", () => {
  it("emulates cpuToQty behavior", () => {
    // From Resources.tsx: .5 cores -> "500m", whole cores -> "<n>"
    expect(formatCpuQuantity({ value: 0.5, unit: "cores" })).toBe("500m");
    expect(formatCpuQuantity({ value: 2, unit: "cores" })).toBe("2");
  });

  it("emulates memToQty behavior", () => {
    // From Resources.tsx: whole Gi -> "<n>Gi", fractional -> "Mi"
    expect(formatMemQuantity({ value: 4, unit: "Gi" })).toBe("4Gi");
    // 0.5 Gi = 512 Mi, so parseMemQuantity would pick Mi
    expect(parseMemQuantity("512Mi")).toEqual({ value: 512, unit: "Mi" });
  });

  it("handles the typical Minecraft resource range", () => {
    // Typical Minecraft: 2-4 cores, 4-8 Gi memory
    expect(parseCpuQuantity("2")).toEqual({ value: 2, unit: "cores" });
    expect(parseCpuQuantity("4")).toEqual({ value: 4, unit: "cores" });
    expect(parseMemQuantity("4Gi")).toEqual({ value: 4, unit: "Gi" });
    expect(parseMemQuantity("8Gi")).toEqual({ value: 8, unit: "Gi" });
  });
});
