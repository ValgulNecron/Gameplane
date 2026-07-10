import { FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

export interface RetentionForm {
  keepLast?: number;
  keepHourly?: number;
  keepDaily?: number;
  keepWeekly?: number;
  keepMonthly?: number;
  keepYearly?: number;
}

interface Props {
  value: RetentionForm;
  onChange: (v: RetentionForm) => void;
}

export function RetentionFields({ value, onChange }: Props) {
  const buckets: Array<{
    key: keyof RetentionForm;
    label: string;
  }> = [
    { key: "keepLast", label: "Keep last" },
    { key: "keepHourly", label: "Hourly" },
    { key: "keepDaily", label: "Daily" },
    { key: "keepWeekly", label: "Weekly" },
    { key: "keepMonthly", label: "Monthly" },
    { key: "keepYearly", label: "Yearly" },
  ];

  const handleChange = (key: keyof RetentionForm, input: string) => {
    const numValue = input === "" ? undefined : Number(input);
    onChange({
      ...value,
      [key]: numValue,
    });
  };

  return (
    <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-3">
      {buckets.map(({ key, label }) => (
        <FieldLabel key={key} label={label}>
          <Input
            type="number"
            min={0}
            value={value[key] ?? ""}
            onChange={(e) => handleChange(key, e.target.value)}
            placeholder="0"
            aria-label={label}
          />
        </FieldLabel>
      ))}
    </div>
  );
}

/**
 * Converts a RetentionForm to the payload shape for the API.
 * Returns an object containing only buckets with a value > 0,
 * or undefined if no buckets have values.
 */
export function buildRetention(v: RetentionForm): RetentionForm | undefined {
  const result: RetentionForm = {};
  let hasAny = false;

  if ((v.keepLast ?? 0) > 0) {
    result.keepLast = v.keepLast;
    hasAny = true;
  }
  if ((v.keepHourly ?? 0) > 0) {
    result.keepHourly = v.keepHourly;
    hasAny = true;
  }
  if ((v.keepDaily ?? 0) > 0) {
    result.keepDaily = v.keepDaily;
    hasAny = true;
  }
  if ((v.keepWeekly ?? 0) > 0) {
    result.keepWeekly = v.keepWeekly;
    hasAny = true;
  }
  if ((v.keepMonthly ?? 0) > 0) {
    result.keepMonthly = v.keepMonthly;
    hasAny = true;
  }
  if ((v.keepYearly ?? 0) > 0) {
    result.keepYearly = v.keepYearly;
    hasAny = true;
  }

  return hasAny ? result : undefined;
}
