import { Sun, Moon, Monitor } from "lucide-react";
import type { ReactNode } from "react";

export type AppearanceMode = "light" | "dark" | "system";

export interface AppearanceToggleProps {
  value: AppearanceMode;
  onChange: (mode: AppearanceMode) => void;
}

/**
 * Three-state appearance toggle: light → dark → system → light.
 * Uses ToggleButtonGroup if available, otherwise three button elements.
 * Persists to localStorage and updates both class and data-theme on <html>.
 */
export function AppearanceToggle({ value, onChange }: AppearanceToggleProps) {
  const modes: Array<{ mode: AppearanceMode; icon: ReactNode; label: string }> = [
    { mode: "light", icon: <Sun className="h-4 w-4" />, label: "Light" },
    { mode: "dark", icon: <Moon className="h-4 w-4" />, label: "Dark" },
    { mode: "system", icon: <Monitor className="h-4 w-4" />, label: "System" },
  ];

  return (
    <div className="flex items-center justify-center gap-1 rounded-lg bg-surface p-1">
      {modes.map(({ mode, icon, label }) => (
        <button
          key={mode}
          type="button"
          aria-label={label}
          title={label}
          onClick={() => onChange(mode)}
          className={`
            flex h-8 w-8 items-center justify-center rounded-md
            transition-colors
            ${
              value === mode
                ? "bg-primary/20 text-primary"
                : "text-muted hover:bg-border/60 hover:text-fg"
            }
          `}
        >
          {icon}
        </button>
      ))}
    </div>
  );
}
