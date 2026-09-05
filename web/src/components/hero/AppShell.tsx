import type { ReactNode } from "react";

export interface AppShellProps {
  sidebar: ReactNode;
  topBar: ReactNode;
  children: ReactNode;
}

/**
 * AppShell — Pure layout wrapper.
 *
 * Renders sidebar (fixed-width on lg+, hidden on mobile — AppLayout handles mobile drawer
 * state), topBar (fixed height), and children (main content, scrollable). No drawer logic
 * here — AppLayout passes the appropriate Sidebar variant.
 */
export function AppShell({ sidebar, topBar, children }: AppShellProps) {
  return (
    <div className="flex h-screen w-full bg-background">
      {/* Sidebar — desktop only, fixed 260px wide */}
      <aside className="hidden lg:flex lg:w-[260px] flex-col">
        {sidebar}
      </aside>

      {/* Main content area — sidebar + topBar + children in a column */}
      <div className="flex flex-1 flex-col">
        {/* TopBar — fixed height 64px (h-16); TopBar's header provides the border */}
        <div className="h-16 flex-shrink-0">
          {topBar}
        </div>

        {/* Main content — scrollable flex-1 */}
        <main className="flex-1 overflow-auto bg-background">
          {children}
        </main>
      </div>
    </div>
  );
}
