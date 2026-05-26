import { cn } from "@/lib/utils";

export interface TabBarItem<K extends string> {
  key: K;
  label: string;
  count?: number;
}

interface TabBarProps<K extends string> {
  items: ReadonlyArray<TabBarItem<K>>;
  value: K;
  onChange: (key: K) => void;
}

// Pill-style segmented control. The Backups page uses a different
// (underline) tab treatment and intentionally does not use this.
export function TabBar<K extends string>({ items, value, onChange }: TabBarProps<K>) {
  return (
    <div className="inline-flex gap-1 rounded-md border border-border bg-card p-1">
      {items.map((t) => {
        const active = value === t.key;
        return (
          <button
            key={t.key}
            type="button"
            onClick={() => onChange(t.key)}
            className={cn(
              "rounded px-3 py-1.5 text-xs transition-colors",
              active ? "bg-primary/15 text-primary" : "text-muted hover:text-fg",
            )}
          >
            {t.label}
            {typeof t.count === "number" && (
              <span
                className={cn(
                  "ml-2 rounded px-1.5 text-[10px] font-mono",
                  active ? "bg-primary/20" : "bg-border/60",
                )}
              >
                {t.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
