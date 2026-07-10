// Module-singleton "current cluster" store for multi-cluster support.
// Persists to localStorage and notifies subscribers on changes.

import { useSyncExternalStore } from "react";

const CLUSTER_KEY = "gameplane.cluster";
const DEFAULT_CLUSTER = "local";

// Subscribers for cluster changes
const subscribers = new Set<() => void>();

// In-memory cache (initially undefined to detect first load)
let currentClusterId: string | undefined;

/**
 * Gets the currently selected cluster id from localStorage, defaulting to "local".
 * Safe for SSR/initial render (reads lazily, guarded for missing localStorage).
 */
export function getCurrentCluster(): string {
  // Return cached value if available (even without localStorage for SSR compatibility)
  if (currentClusterId !== undefined) {
    return currentClusterId;
  }

  // SSR guard: if localStorage is unavailable, use default
  if (typeof window === "undefined" || typeof localStorage === "undefined") {
    return DEFAULT_CLUSTER;
  }

  // Load from localStorage on first read
  const stored = localStorage.getItem(CLUSTER_KEY);
  currentClusterId = stored || DEFAULT_CLUSTER;

  return currentClusterId;
}

/**
 * Sets the current cluster id and persists to localStorage.
 * Notifies all subscribers.
 */
export function setCurrentCluster(id: string): void {
  if (typeof window !== "undefined" && typeof localStorage !== "undefined") {
    localStorage.setItem(CLUSTER_KEY, id);
  }
  currentClusterId = id;
  notifySubscribers();
}

/**
 * Subscribes to cluster changes. Returns an unsubscribe function.
 * Call the returned function to stop listening.
 */
export function subscribeCluster(cb: () => void): () => void {
  subscribers.add(cb);
  return () => subscribers.delete(cb);
}

/**
 * Notifies all subscribers that the cluster changed.
 */
function notifySubscribers(): void {
  subscribers.forEach((cb) => cb());
}

/**
 * React hook that syncs the current cluster from the external store.
 * Re-renders components when the cluster changes.
 */
export function useCurrentCluster(): string {
  return useSyncExternalStore<string>(
    subscribeCluster,
    getCurrentCluster,
    // SSR fallback: return default cluster during server-side rendering
    () => DEFAULT_CLUSTER,
  );
}
