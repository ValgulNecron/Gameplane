import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};

  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString();
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

beforeEach(() => {
  // Reset store and mock localStorage for each test
  localStorageMock.clear();
  Object.defineProperty(window, "localStorage", {
    value: localStorageMock,
    writable: true,
  });
  // Clear any cached cluster value by re-importing
  vi.resetModules();
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("Cluster Store", () => {
  it("returns default 'local' cluster when nothing is stored", async () => {
    // Re-import to get fresh module state
    const { getCurrentCluster: freshGetCurrentCluster } = await import("./cluster");
    expect(freshGetCurrentCluster()).toBe("local");
  });

  it("persists cluster selection to localStorage", async () => {
    const { getCurrentCluster: freshGetCurrentCluster, setCurrentCluster: freshSetCurrentCluster } =
      await import("./cluster");

    freshSetCurrentCluster("remote-prod");
    expect(localStorageMock.getItem("gameplane.cluster")).toBe("remote-prod");
    expect(freshGetCurrentCluster()).toBe("remote-prod");
  });

  it("retrieves persisted cluster from localStorage on first call", async () => {
    localStorageMock.setItem("gameplane.cluster", "remote-staging");
    const { getCurrentCluster: freshGetCurrentCluster } = await import("./cluster");
    expect(freshGetCurrentCluster()).toBe("remote-staging");
  });

  it("notifies subscribers when cluster changes", async () => {
    const { setCurrentCluster: freshSetCurrentCluster, subscribeCluster: freshSubscribeCluster } =
      await import("./cluster");

    const cb = vi.fn();
    freshSubscribeCluster(cb);

    freshSetCurrentCluster("remote-prod");
    expect(cb).toHaveBeenCalled();

    freshSetCurrentCluster("remote-staging");
    expect(cb).toHaveBeenCalledTimes(2);
  });

  it("unsubscribe stops receiving notifications", async () => {
    const { setCurrentCluster: freshSetCurrentCluster, subscribeCluster: freshSubscribeCluster } =
      await import("./cluster");

    const cb = vi.fn();
    const unsub = freshSubscribeCluster(cb);

    freshSetCurrentCluster("remote-prod");
    expect(cb).toHaveBeenCalledTimes(1);

    unsub();
    freshSetCurrentCluster("remote-staging");
    expect(cb).toHaveBeenCalledTimes(1); // Still only 1, unsubscribe worked
  });

  it("handles missing localStorage gracefully (SSR)", async () => {
    // Temporarily remove localStorage
    const originalLocalStorage = window.localStorage;
    Object.defineProperty(window, "localStorage", {
      value: undefined,
      writable: true,
    });

    const { getCurrentCluster: freshGetCurrentCluster, setCurrentCluster: freshSetCurrentCluster } =
      await import("./cluster");

    expect(freshGetCurrentCluster()).toBe("local");
    // Should not throw even though localStorage is undefined
    freshSetCurrentCluster("remote-prod");
    expect(freshGetCurrentCluster()).toBe("remote-prod");

    // Restore localStorage
    Object.defineProperty(window, "localStorage", {
      value: originalLocalStorage,
      writable: true,
    });
  });

  it("useCurrentCluster hook returns current cluster", async () => {
    const { renderHook, act } = await import("@testing-library/react");
    const { useCurrentCluster: freshUseCurrentCluster, setCurrentCluster: freshSetCurrentCluster } =
      await import("./cluster");

    const { result } = renderHook(() => freshUseCurrentCluster());

    // Initially should return "local"
    expect(result.current).toBe("local");

    // After setting a new cluster, the hook should reflect the change
    act(() => {
      freshSetCurrentCluster("remote-prod");
    });

    expect(result.current).toBe("remote-prod");
  });
});
