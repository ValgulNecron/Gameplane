import type { ReactNode } from "react";
import {
  Button,
  Checkbox,
  Popover,
  Separator,
} from "@heroui/react";
import { cn } from "@/lib/utils";

interface FilterPopoverProps {
  /**
   * The trigger button content (usually an icon + label).
   */
  children: ReactNode;
  /**
   * List of distinct games to filter by.
   */
  games: string[];
  /**
   * Currently selected games in the draft filter state.
   */
  selectedGames: Set<string>;
  /**
   * Callback when a game checkbox is toggled.
   */
  onToggleGame: (game: string) => void;
  /**
   * List of distinct namespaces to filter by.
   */
  namespaces: string[];
  /**
   * Currently selected namespaces in the draft filter state.
   */
  selectedNamespaces: Set<string>;
  /**
   * Callback when a namespace checkbox is toggled.
   */
  onToggleNamespace: (ns: string) => void;
  /**
   * Callback to apply the current draft filters.
   */
  onApply: () => void;
  /**
   * Callback to clear all draft filters.
   */
  onClear: () => void;
  /**
   * Whether the popover is open.
   */
  isOpen: boolean;
  /**
   * Callback when the popover open state changes.
   */
  onOpenChange: (open: boolean) => void;
  /**
   * Optional className for the popover content.
   */
  contentClassName?: string;
}

/**
 * FilterPopover composes HeroUI Popover with Checkbox controls
 * for filtering server lists by game template and namespace.
 * Follows the design from node FyV6E with Clear/Apply buttons.
 */
export function FilterPopover({
  children,
  games,
  selectedGames,
  onToggleGame,
  namespaces,
  selectedNamespaces,
  onToggleNamespace,
  onApply,
  onClear,
  isOpen,
  onOpenChange,
  contentClassName,
}: FilterPopoverProps) {
  return (
    <Popover isOpen={isOpen} onOpenChange={onOpenChange}>
      <Popover.Trigger>
        {children}
      </Popover.Trigger>
      <Popover.Content className={cn("w-[240px] bg-surface p-1", contentClassName)}>
        <div className="space-y-0">
          {/* Game section header */}
          {games.length > 0 && (
            <>
              <div className="px-2 py-1.5 text-xs font-semibold text-muted">Game</div>
              {games.map((game) => (
                <div
                  key={game}
                  className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-default/40"
                >
                  <Checkbox
                    isSelected={selectedGames.has(game)}
                    onChange={() => onToggleGame(game)}
                  >
                    <Checkbox.Control>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <Checkbox.Content className="text-sm text-foreground">
                      {game}
                    </Checkbox.Content>
                  </Checkbox>
                </div>
              ))}
            </>
          )}

          {/* Separator between sections */}
          {games.length > 0 && namespaces.length > 0 && (
            <Separator className="my-1" />
          )}

          {/* Namespace section header */}
          {namespaces.length > 0 && (
            <>
              <div className="px-2 py-1.5 text-xs font-semibold text-muted">Namespace</div>
              {namespaces.map((ns) => (
                <div
                  key={ns}
                  className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-default/40"
                >
                  <Checkbox
                    isSelected={selectedNamespaces.has(ns)}
                    onChange={() => onToggleNamespace(ns)}
                  >
                    <Checkbox.Control>
                      <Checkbox.Indicator />
                    </Checkbox.Control>
                    <Checkbox.Content className="text-sm text-foreground">
                      {ns}
                    </Checkbox.Content>
                  </Checkbox>
                </div>
              ))}
            </>
          )}

          {/* Separator before footer buttons */}
          {(games.length > 0 || namespaces.length > 0) && (
            <Separator className="my-1" />
          )}

          {/* Footer buttons: Clear and Apply */}
          <div className="flex items-center justify-between gap-2 px-2 py-1.5">
            <Button
              size="sm"
              variant="ghost"
              className="h-7"
              onClick={onClear}
              aria-label="Clear all filters"
            >
              Clear
            </Button>
            <Button
              size="sm"
              variant="primary"
              className="h-7"
              onClick={onApply}
              aria-label="Apply filters"
            >
              Apply
            </Button>
          </div>
        </div>
      </Popover.Content>
    </Popover>
  );
}
