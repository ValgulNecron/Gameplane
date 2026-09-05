import { useRef, useState, type JSX, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Search, Server } from "lucide-react";
import {
  SearchFieldRoot,
  SearchFieldInput,
  SearchFieldClearButton,
  PopoverContent,
} from "@heroui/react";
import { Servers } from "@/lib/endpoints";
import { cn } from "@/lib/utils";

/**
 * GlobalSearch provides a server search dropdown with keyboard navigation.
 * Ported from AppLayout.tsx onto HeroUI's SearchField + Popover primitives.
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

  // Note: selectedIndex is reset to -1 directly in the value-change handler
  // below (and in navigateToServer) whenever the query text changes — not in
  // a useEffect here, since `matches` is a fresh array every render and
  // setState-in-effect off that would cascade renders.

  const navigateToServer = async (name: string) => {
    setOpen(false);
    setQ("");
    setSelectedIndex(-1);
    await navigate({ to: "/servers/$name", params: { name } });
  };

  // HeroUI's SearchField already handles Enter (onSubmit, unused here) and
  // Escape (clears the value, which our onChange below turns into a close)
  // as native shortcuts before this handler runs — see
  // node_modules/react-aria/dist/private/searchfield/useSearchField.mjs.
  // We only need Arrow navigation and our own Enter-to-navigate behavior.
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

  return (
    <div className="relative hidden w-[300px] md:block">
      <SearchFieldRoot
        aria-label="Search servers"
        value={q}
        onChange={(value) => {
          setQ(value);
          setOpen(value.trim().length > 0);
          setSelectedIndex(-1);
        }}
        className="relative flex items-center rounded-full border border-border bg-surface"
      >
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
        <SearchFieldInput
          ref={inputRef}
          placeholder="Search servers…"
          data-testid="search-input"
          onFocus={() => {
            if (query.length > 0) setOpen(true);
          }}
          // Delay so a result click registers before the popover unmounts.
          onBlur={() => setTimeout(() => setOpen(false), 120)}
          onKeyDown={handleKeyDown}
          className="h-10 w-full rounded-full border-0 bg-transparent pl-9 pr-8 text-sm text-fg placeholder:text-muted focus:outline-hidden"
        />
        <SearchFieldClearButton className="absolute right-2 top-1/2 -translate-y-1/2" />
      </SearchFieldRoot>

      <PopoverContent
        triggerRef={inputRef}
        isOpen={open && query.length > 0}
        onOpenChange={setOpen}
        isNonModal
        placement="bottom start"
        className="w-72 overflow-hidden rounded-md border border-border bg-background p-0 shadow-lg"
      >
        <ul role="listbox" className="max-h-72 overflow-auto">
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
                onClick={() => void navigateToServer(server.metadata.name)}
                className={cn(
                  "flex cursor-pointer items-center gap-2 px-3 py-2 text-sm",
                  selectedIndex === idx
                    ? "bg-surface text-fg"
                    : "text-fg hover:bg-surface"
                )}
                data-testid={`search-result-${server.metadata.name}`}
              >
                <Server className="h-3.5 w-3.5 shrink-0 text-muted" />
                <span className="truncate font-mono">
                  {server.metadata.name}
                </span>
              </li>
            ))
          )}
        </ul>
      </PopoverContent>
    </div>
  );
}
