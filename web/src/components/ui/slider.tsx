import { cn } from "@/lib/utils";

interface SliderProps {
  value: number;
  min: number;
  max: number;
  step?: number;
  onValueChange: (v: number) => void;
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
  className?: string;
}

export function Slider({
  value,
  min,
  max,
  step = 1,
  onValueChange,
  disabled,
  id,
  className,
  ...rest
}: SliderProps) {
  return (
    <input
      id={id}
      type="range"
      role="slider"
      aria-label={rest["aria-label"]}
      min={min}
      max={max}
      step={step}
      value={value}
      disabled={disabled}
      onChange={(e) => onValueChange(Number(e.target.value))}
      className={cn(
        "h-2 w-full cursor-pointer appearance-none rounded-full bg-border accent-primary disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
    />
  );
}
