import { useState, useRef, type JSX, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Search, Server } from "lucide-react";
import { Servers } from "@/lib/endpoints";
import { cn } from "@/lib/utils";

/**
 * GlobalSearch provides a server search dropdown with keyboard navigation.
 * Ported from AppLayout.tsx with HeroUI components.
 * Design export: IdaU7
 */
export function GlobalSearch(): JSX.Element {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();

  const { data } = useQuery({
    queryKey: ["servers"],
    queryFn: () => Servers.list(),
    staleTime: 10_000,
  });

  const query = q.trim().toLowerCase();
  const matches =
    query.length > 0
      ? (data?.items ?? [])
          .filter((s) => s.metadata.name.toLowerCase().includes(query))
          .slice(0, 6)
      : [];

  // Note: selectedIndex is reset to -1 directly in the input's onChange
  // handler below (and in navigateToServer) whenever the query text
  // changes — not in a useEffect here, since `matches` is a fresh array
  // every render and setState-in-effect off that would cascade renders.

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!open || matches.length === 0) {
      if (e.key === "Enter") {
        setOpen(true);
      }
      return;
    }

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex((prev) =>
          prev < matches.length - 1 ? prev + 1 : prev
        );
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1));
        break;
      case "Enter":
        e.preventDefault();
        if (selectedIndex >= 0 && selectedIndex < matches.length) {
          void navigateToServer(matches[selectedIndex].metadata.name);
        } else if (matches.length > 0) {
          void navigateToServer(matches[0].metadata.name);
        }
        break;
      case "Escape":
        e.preventDefault();
        setOpen(false);
        break;
    }
  };

  const navigateToServer = async (name: string) => {
    setOpen(false);
    setQ("");
    setSelectedIndex(-1);
    await navigate({ to: "/servers/$name", params: { name } });
  };

  return (
    <div className="relative hidden w-72 md:block">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
        <input
          ref={inputRef}
          type="search"
          aria-label="Search servers"
          placeholder="Search servers…"
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setOpen(e.target.value.length > 0);
            setSelectedIndex(-1);
          }}
          onFocus={() => {
            if (q.length > 0) setOpen(true);
          }}
          // Delay so a result click registers before the dropdown unmounts.
          onBlur={() => setTimeout(() => setOpen(false), 120)}
          onKeyDown={handleKeyDown}
          className="h-9 w-full rounded-md border border-border bg-surface pl-9 pr-3 text-sm text-fg placeholder:text-muted focus:border-primary focus:outline-hidden"
          data-testid="search-input"
        />
      </div>

      {open && query.length > 0 && (
        <ul className="absolute z-20 mt-1 w-full overflow-hidden rounded-md border border-border bg-background shadow-lg" role="listbox">
          {matches.length === 0 ? (
            <li className="px-3 py-2 text-sm text-muted">
              No servers match.
            </li>
          ) : (
            matches.map((server, idx) => (
              <li
                key={server.metadata.name}
                role="option"
                aria-selected={selectedIndex === idx}
                onMouseEnter={() => setSelectedIndex(idx)}
                onClick={() => navigateToServer(server.metadata.name)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 px-3 py-2 text-sm",
                  selectedIndex === idx
                    ? "bg-surface text-fg"
                    : "text-fg hover:bg-surface"
                )}
                data-testid={`search-result-${server.metadata.name}`}
              >
                <Server className="h-3.5 w-3.5 flex-shrink-0 text-muted" />
                <span className="truncate font-mono">
                  {server.metadata.name}
                </span>
              </li>
            ))
          )}
        </ul>
      )}
    </div>
  );
}
