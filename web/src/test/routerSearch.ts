import { useSyncExternalStore } from "react";

// Search-state contract for page tests that replace Router's Link with an
// anchor. Actual navigation is covered with a real router in fleetLocation.test.
let search: Record<string, unknown> = {};
const listeners = new Set<() => void>();

export function useTestLocation() {
  return { search: useSyncExternalStore(
    (listener) => { listeners.add(listener); return () => { listeners.delete(listener); }; },
    () => search,
  ) };
}

export function navigateTestSearch(options: { search?: (previous: Record<string, unknown>) => Record<string, unknown> }) {
  if (options.search) {
    search = options.search(search);
    listeners.forEach((listener) => listener());
  }
  return Promise.resolve();
}

export function resetTestSearch() {
  search = {};
}
