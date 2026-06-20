import { useEffect, useState, type ReactNode } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";

import type { RegistryProject } from "@/types";
import { Servers } from "@/lib/endpoints";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { APIError } from "@/lib/api";

const PAGE = 24;

export type RegistrySort = "downloads" | "updated" | "newest";

const SORTS: { value: RegistrySort; label: string }[] = [
  { value: "downloads", label: "Downloads" },
  { value: "updated", label: "Recently updated" },
  { value: "newest", label: "Newest" },
];

// RegistryBrowser is the shared full browser used by the Mods Browse mode
// and the Modpacks tab: a debounced search, a sort control, optional
// category chips, a paged result grid (load-more), and a default "popular"
// listing when the search is empty. Each result is rendered by renderItem,
// so the caller owns the per-card action (install a mod vs apply a modpack).
export function RegistryBrowser({
  name,
  type,
  categories,
  renderItem,
}: {
  name: string;
  type?: "mod" | "modpack";
  categories?: { value: string; label: string }[];
  renderItem: (project: RegistryProject) => ReactNode;
}) {
  const [term, setTerm] = useState("");
  const [debounced, setDebounced] = useState("");
  const [sort, setSort] = useState<RegistrySort>("downloads");
  const [category, setCategory] = useState("");

  useEffect(() => {
    const t = setTimeout(() => setDebounced(term.trim()), 300);
    return () => clearTimeout(t);
  }, [term]);

  const q = useInfiniteQuery({
    queryKey: ["registry", name, type ?? "mod", debounced, sort, category],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      Servers.searchRegistry(name, {
        q: debounced,
        type,
        // A search term ranks by relevance; an empty browse uses the chosen sort.
        sort: debounced ? undefined : sort,
        category,
        limit: PAGE,
        offset: pageParam,
      }),
    getNextPageParam: (last, pages) => (last.length === PAGE ? pages.length * PAGE : undefined),
  });

  const items = q.data?.pages.flat() ?? [];
  const unavailable = q.error instanceof APIError && q.error.status === 501;

  return (
    <div className="flex min-h-0 flex-col gap-3">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
          <Input
            autoFocus
            className="pl-8"
            placeholder={type === "modpack" ? "Search modpacks…" : "Search mods…"}
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            spellCheck={false}
          />
        </div>
        <select
          aria-label="Sort"
          value={sort}
          onChange={(e) => setSort(e.target.value as RegistrySort)}
          disabled={!!debounced}
          title={debounced ? "Sorted by relevance while searching" : undefined}
          className="h-9 rounded border border-border bg-surface px-2 text-xs disabled:opacity-50"
        >
          {SORTS.map((s) => (
            <option key={s.value} value={s.value}>
              Sort: {s.label}
            </option>
          ))}
        </select>
      </div>

      {categories && categories.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {[{ value: "", label: "All" }, ...categories].map((c) => {
            const active = category === c.value;
            return (
              <button
                key={c.value || "all"}
                type="button"
                onClick={() => setCategory(c.value)}
                aria-pressed={active}
                className={
                  active
                    ? "rounded-full bg-primary px-2.5 py-0.5 text-[11px] font-medium text-primary-foreground"
                    : "rounded-full border border-border px-2.5 py-0.5 text-[11px] text-muted hover:text-fg"
                }
              >
                {c.label}
              </button>
            );
          })}
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-auto">
        {unavailable ? (
          <p className="text-xs text-muted">In-app browse isn’t available for this server.</p>
        ) : q.isLoading ? (
          <p className="text-xs text-muted">Loading…</p>
        ) : q.isError ? (
          <p className="text-xs text-danger">{errText(q.error)}</p>
        ) : items.length === 0 ? (
          <p className="text-xs text-muted">No results.</p>
        ) : (
          <>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {items.map((p) => (
                <div key={`${p.provider}:${p.id}`}>{renderItem(p)}</div>
              ))}
            </div>
            {q.hasNextPage && (
              <div className="flex justify-center pt-3">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void q.fetchNextPage()}
                  disabled={q.isFetchingNextPage}
                >
                  {q.isFetchingNextPage ? "Loading…" : "Load more"}
                </Button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function errText(err: unknown): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string };
      if (parsed.error) return parsed.error;
    } catch {
      // fall through
    }
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "request failed";
}

// compactNum formats large download counts (e.g. 1.2M).
export function compactNum(n: number): string {
  return new Intl.NumberFormat(undefined, { notation: "compact", maximumFractionDigits: 1 }).format(n);
}

// RegistryIcon shows a registry icon with a graceful fallback when it's
// missing or the third-party CDN image fails to load.
export function RegistryIcon({ url, fallback }: { url?: string; fallback: ReactNode }) {
  const [failed, setFailed] = useState(false);
  if (!url || failed) return <>{fallback}</>;
  return (
    <img
      src={url}
      alt=""
      loading="lazy"
      className="h-9 w-9 shrink-0 rounded object-cover"
      onError={() => setFailed(true)}
    />
  );
}
