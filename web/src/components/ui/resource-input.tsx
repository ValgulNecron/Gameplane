import { useState, useMemo } from "react";
import { Slider } from "./slider";
import { Input } from "./input";
import { Select, type SelectOption } from "./select";
import {
  parseCpuQuantity,
  formatCpuQuantity,
  convertCpu,
  cpuCores,
  parseMemQuantity,
  formatMemQuantity,
  convertMem,
  memBytes,
  type CpuAmount,
  type MemAmount,
  type CpuUnit,
  type MemUnit,
} from "@/lib/quantity";

export interface ResourceInputProps {
  kind: "cpu" | "memory";
  value: string; // canonical k8s quantity (e.g., "2", "500m", "4Gi", "512Mi")
  onChange: (qty: string) => void; // emits canonical k8s quantity string
  min?: number; // slider min in base units (cores for cpu, GiB for memory)
  max?: number; // slider max in base units
  step?: number; // slider step in base units
  disabled?: boolean;
  id?: string;
}

const CPU_DEFAULTS = { min: 0.5, max: 16, step: 0.5 };
const MEM_DEFAULTS = { min: 0.5, max: 64, step: 0.5 };

const CPU_UNIT_OPTIONS: SelectOption[] = [
  { value: "cores", label: "cores" },
  { value: "m", label: "mCPU" },
];

const MEM_UNIT_OPTIONS: SelectOption[] = [
  { value: "Ki", label: "KiB" },
  { value: "Mi", label: "MiB" },
  { value: "Gi", label: "GiB" },
  { value: "Ti", label: "TiB" },
];

/**
 * ResourceInput: a reusable control combining slider + numeric input + unit dropdown
 * for selecting CPU and memory resources. All three controls stay in sync.
 *
 * The slider's axis is always the base unit (cores for CPU, GiB for memory).
 * The input shows the value in the currently selected display unit.
 * Interactions are always converted back to canonical k8s quantity strings.
 */
export function ResourceInput({
  kind,
  value,
  onChange,
  min: userMin,
  max: userMax,
  step: userStep,
  disabled = false,
  id,
}: ResourceInputProps) {
  const defaults = kind === "cpu" ? CPU_DEFAULTS : MEM_DEFAULTS;
  const minBase = userMin ?? defaults.min;
  const maxBase = userMax ?? defaults.max;
  const stepBase = userStep ?? defaults.step;

  // Parse the canonical value to get the base magnitude and infer a natural display unit.
  const parsed = useMemo(() => {
    if (kind === "cpu") {
      const parsed = parseCpuQuantity(value);
      if (!parsed) return { baseValue: minBase, displayAmount: { value: minBase, unit: "cores" as CpuUnit } };
      const cores = cpuCores(parsed);
      return { baseValue: cores, displayAmount: parsed };
    } else {
      const parsed = parseMemQuantity(value);
      if (!parsed) return { baseValue: minBase, displayAmount: { value: minBase, unit: "Gi" as MemUnit } };
      const gib = memBytes(parsed) / (1024 ** 3);
      return { baseValue: gib, displayAmount: parsed };
    }
  }, [value, kind, minBase]);

  // Internal display unit state: initialize from the parsed value.
  const [displayUnit, setDisplayUnit] = useState<CpuUnit | MemUnit>(parsed.displayAmount.unit);

  // Current base value (clamped to [min, max]).
  const baseValue = Math.max(minBase, Math.min(maxBase, parsed.baseValue));

  // Compute the input value in the current display unit.
  const inputValue = useMemo(() => {
    if (kind === "cpu") {
      const cpuAmount = parsed.displayAmount as CpuAmount;
      const converted = convertCpu(cpuAmount, displayUnit as CpuUnit);
      return String(converted.value);
    } else {
      const memAmount = parsed.displayAmount as MemAmount;
      const converted = convertMem(memAmount, displayUnit as MemUnit);
      return String(converted.value);
    }
  }, [parsed.displayAmount, displayUnit, kind]);

  // Emit a canonical k8s quantity string.
  const emitQuantity = (base: number) => {
    const clamped = Math.max(minBase, Math.min(maxBase, base));
    if (kind === "cpu") {
      const amount: CpuAmount = { value: clamped, unit: "cores" };
      onChange(formatCpuQuantity(amount));
    } else {
      const amount: MemAmount = { value: clamped, unit: "Gi" };
      onChange(formatMemQuantity(amount));
    }
  };

  // Slider change: emit based on base value.
  const handleSliderChange = (newBase: number) => {
    emitQuantity(newBase);
  };

  // Input change: parse in display unit, convert to base, emit.
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const text = e.target.value;
    if (!text) return; // Ignore empty input during typing.

    const num = Number(text);
    if (!Number.isFinite(num)) return; // Ignore non-numeric input.

    if (kind === "cpu") {
      const amount: CpuAmount = { value: num, unit: displayUnit as CpuUnit };
      const cores = cpuCores(amount);
      emitQuantity(cores);
    } else {
      const amount: MemAmount = { value: num, unit: displayUnit as MemUnit };
      const gib = memBytes(amount) / (1024 ** 3);
      emitQuantity(gib);
    }
  };

  // Input blur: snap to clamped value.
  const handleInputBlur = () => {
    emitQuantity(baseValue);
  };

  // Unit dropdown change: convert the current amount to the new display unit (no base change).
  const handleUnitChange = (newUnit: string) => {
    if (kind === "cpu") {
      const cpuAmount = parsed.displayAmount as CpuAmount;
      const converted = convertCpu(cpuAmount, newUnit as CpuUnit);
      setDisplayUnit(converted.unit);
    } else {
      const memAmount = parsed.displayAmount as MemAmount;
      const converted = convertMem(memAmount, newUnit as MemUnit);
      setDisplayUnit(converted.unit);
    }
  };

  const ariaLabel = kind === "cpu" ? "CPU cores" : "Memory (GiB)";
  const unitOptions = kind === "cpu" ? CPU_UNIT_OPTIONS : MEM_UNIT_OPTIONS;
  const inputId = id ? `${id}-input` : undefined;
  const selectId = id ? `${id}-unit` : undefined;

  return (
    <div className="flex items-center gap-2 sm:gap-3">
      <Slider
        id={id}
        value={baseValue}
        min={minBase}
        max={maxBase}
        step={stepBase}
        onValueChange={handleSliderChange}
        disabled={disabled}
        aria-label={ariaLabel}
        className="flex-1"
      />
      <Input
        id={inputId}
        type="number"
        value={inputValue}
        onChange={handleInputChange}
        onBlur={handleInputBlur}
        disabled={disabled}
        step={stepBase}
        className="w-16"
        aria-label={`${ariaLabel} value`}
      />
      <Select
        id={selectId}
        options={unitOptions}
        value={displayUnit}
        onValueChange={handleUnitChange}
        disabled={disabled}
        className="w-24"
        aria-label={`${ariaLabel} unit`}
      />
    </div>
  );
}
