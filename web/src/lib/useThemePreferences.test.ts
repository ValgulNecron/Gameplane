import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { createElement, type ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { APIError } from "@/lib/api";
import { makeClient } from "@/test/render";
import { makeUser } from "@/test/factories";
import type { User, UserThemePreferences } from "@/types";

function renderThemeHook<T>(cb: () => T) {
  const client = makeClient();
  const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client }, children);
  return renderHook(cb, { wrapper });
}
import {
  CUSTOM_MODE_STORAGE_KEY,
  CUSTOM_THEME_VARS_ELEMENT_ID,
  CUSTOM_CSS_ELEMENT_ID,
  DEFAULT_THEME_PREFERENCES,
  SAFE_MODE_SESSION_KEY,
  THEME_ERROR_EVENT,
  THEME_PREFS_STORAGE_KEY,
  THEME_VARS_CSS_STORAGE_KEY,
  applyThemePreferences,
  isSafeModeActive,
  readThemePreferences,
  resetUrlSafeModeForTests,
  unmountCustomCssOverlay,
  useThemePreferences,
  writeThemePreferences,
} from "./useThemePreferences";

// The hook talks to the backend only through Users.updatePreferences, so
// mocking that single call (rather than MSW) gives full control over
// success/APIError/network-error/non-Error rejection branches without a
// server round trip — the same pattern route tests use for endpoints.ts
// (e.g. src/routes/Modules.test.tsx).
vi.mock("@/lib/endpoints", () => ({
  Users: { updatePreferences: vi.fn() },
}));
import { Users } from "@/lib/endpoints";
const updatePreferencesMock = vi.mocked(Users.updatePreferences);

const CUSTOM_COLORS = { accent: "#10B981", surface: "#1E293B" };

function resetDom() {
  document.getElementById(CUSTOM_THEME_VARS_ELEMENT_ID)?.remove();
  document.getElementById(CUSTOM_CSS_ELEMENT_ID)?.remove();
  const html = document.documentElement;
  html.classList.remove("dark", "light");
  delete html.dataset.theme;
  delete html.dataset.themePreset;
  delete html.dataset.themeType;
  delete html.dataset.customCss;
}

beforeEach(() => {
  window.localStorage.clear();
  window.sessionStorage.clear();
  resetDom();
  updatePreferencesMock.mockReset();
});

afterEach(() => {
  resetDom();
  vi.unstubAllGlobals();
});

describe("isSafeModeActive", () => {
  it("returns false when sessionStorage is blocked (throws on read)", () => {
    const original = Object.getOwnPropertyDescriptor(window, "sessionStorage");
    Object.defineProperty(window, "sessionStorage", {
      value: {
        getItem: vi.fn(() => {
          throw new Error("storage blocked");
        }),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn(),
        length: 0,
        key: vi.fn(),
      },
      writable: true,
      configurable: true,
    });
    try {
      expect(isSafeModeActive()).toBe(false);
    } finally {
      if (original) Object.defineProperty(window, "sessionStorage", original);
    }
  });

  it("returns true from the session flag when set", () => {
    window.sessionStorage.setItem(SAFE_MODE_SESSION_KEY, "1");
    expect(isSafeModeActive()).toBe(true);
  });

  it("keeps URL safe mode across in-app navigation that drops the query string (F-125)", () => {
    const originalPath = window.location.pathname + window.location.search;
    resetUrlSafeModeForTests();
    window.history.replaceState(null, "", "/?safe-mode=1");
    try {
      expect(isSafeModeActive()).toBe(true);
      // In-memory only: the URL entry point must not survive a full reload.
      expect(window.sessionStorage.getItem(SAFE_MODE_SESSION_KEY)).toBeNull();

      // Simulate the in-app navigation that drops the query string (e.g.
      // the banner's navigate() to /settings/theme).
      window.history.replaceState(null, "", "/settings/theme");
      expect(isSafeModeActive()).toBe(true);
    } finally {
      resetUrlSafeModeForTests();
      window.history.replaceState(null, "", originalPath);
    }
  });

  it("does not apply URL safe mode on a fresh page load without the parameter", () => {
    const originalPath = window.location.pathname + window.location.search;
    window.history.replaceState(null, "", "/?safe-mode=1");
    try {
      expect(isSafeModeActive()).toBe(true);
      // A full reload re-evaluates the module: simulate with the reset hook.
      resetUrlSafeModeForTests();
      window.history.replaceState(null, "", "/");
      expect(isSafeModeActive()).toBe(false);
    } finally {
      resetUrlSafeModeForTests();
      window.history.replaceState(null, "", originalPath);
    }
  });
});

describe("readThemePreferences", () => {
  it("returns null and swallows a corrupt cache", () => {
    window.localStorage.setItem(THEME_PREFS_STORAGE_KEY, "{not json");
    expect(readThemePreferences()).toBeNull();
  });

  it("returns null when nothing is cached", () => {
    expect(readThemePreferences()).toBeNull();
  });
});

describe("writeThemePreferences", () => {
  it("caches derived custom-color CSS and the resolved surface mode for a custom_colors theme", () => {
    writeThemePreferences({
      ...DEFAULT_THEME_PREFERENCES,
      themeType: "custom_colors",
      customColors: CUSTOM_COLORS,
    });
    const varsCss = window.localStorage.getItem(THEME_VARS_CSS_STORAGE_KEY);
    expect(varsCss).toContain("--accent: #10b981;");
    // Dark Slate (#1E293B) resolves to the dark surface mode.
    expect(window.localStorage.getItem(CUSTOM_MODE_STORAGE_KEY)).toBe("dark");
  });

  it("removes the cached custom-color CSS/mode when switching back to a preset", () => {
    writeThemePreferences({
      ...DEFAULT_THEME_PREFERENCES,
      themeType: "custom_colors",
      customColors: CUSTOM_COLORS,
    });
    writeThemePreferences(DEFAULT_THEME_PREFERENCES);
    expect(window.localStorage.getItem(THEME_VARS_CSS_STORAGE_KEY)).toBeNull();
    expect(window.localStorage.getItem(CUSTOM_MODE_STORAGE_KEY)).toBeNull();
  });
});

describe("applyThemePreferences — resolveAppearanceMode fallback", () => {
  it("falls back to dark when the mode is system and matchMedia is unavailable", () => {
    const original = window.matchMedia;
    // @ts-expect-error — simulating an environment without matchMedia,
    // mirroring the boot script's own fallback.
    delete window.matchMedia;
    try {
      applyThemePreferences({ ...DEFAULT_THEME_PREFERENCES, appearanceMode: "system" });
      expect(document.documentElement.dataset.theme).toBe("dark");
    } finally {
      window.matchMedia = original;
    }
  });
});

describe("unmountCustomCssOverlay", () => {
  it("removes the mounted overlay element and flips data-custom-css to off", () => {
    applyThemePreferences({
      ...DEFAULT_THEME_PREFERENCES,
      customCssEnabled: true,
      customCss: "body { color: red; }",
    });
    expect(document.getElementById(CUSTOM_CSS_ELEMENT_ID)).not.toBeNull();
    unmountCustomCssOverlay();
    expect(document.getElementById(CUSTOM_CSS_ELEMENT_ID)).toBeNull();
    expect(document.documentElement.dataset.customCss).toBe("off");
  });
});

describe("useThemePreferences — profile reconciliation", () => {
  it("adopts me.preferences into state, cache, and the DOM on mount", async () => {
    const me: User = makeUser({
      preferences: {
        ...DEFAULT_THEME_PREFERENCES,
        presetId: "legacy",
      },
    });
    const { result } = renderThemeHook(() => useThemePreferences(me));

    await waitFor(() => {
      expect(result.current.preferences?.presetId).toBe("legacy");
    });
    expect(readThemePreferences()?.presetId).toBe("legacy");
    expect(document.documentElement.dataset.themePreset).toBe("legacy");
  });

  it("does not overwrite an already-matching cache (skips the write)", async () => {
    writeThemePreferences({ ...DEFAULT_THEME_PREFERENCES, presetId: "legacy" });
    const me: User = makeUser({
      preferences: { ...DEFAULT_THEME_PREFERENCES, presetId: "legacy" },
    });
    const { result } = renderThemeHook(() => useThemePreferences(me));
    await waitFor(() => {
      expect(result.current.preferences?.presetId).toBe("legacy");
    });
    expect(document.documentElement.dataset.themePreset).toBe("legacy");
  });
});

describe("useThemePreferences — updatePreferences success", () => {
  it("optimistically applies the patch, then reconciles from the server response", async () => {
    const server: UserThemePreferences = { ...DEFAULT_THEME_PREFERENCES, presetId: "legacy" };
    updatePreferencesMock.mockResolvedValueOnce(server);
    const { result } = renderThemeHook(() => useThemePreferences());

    act(() => {
      result.current.updatePreferences({ presetId: "legacy" });
    });
    // Optimistic apply lands synchronously.
    expect(document.documentElement.dataset.themePreset).toBe("legacy");

    await waitFor(() => expect(result.current.isUpdating).toBe(false));
    expect(result.current.error).toBeNull();
    expect(result.current.preferences?.presetId).toBe("legacy");
    expect(readThemePreferences()?.presetId).toBe("legacy");
  });
});

describe("useThemePreferences — updatePreferences failure paths", () => {
  it("reverts to the previous state and reports the APIError body via onError + THEME_ERROR_EVENT", async () => {
    updatePreferencesMock.mockRejectedValueOnce(new APIError(403, "forbidden"));
    const onError = vi.fn();
    const eventListener = vi.fn();
    window.addEventListener(THEME_ERROR_EVENT, eventListener);
    const { result } = renderThemeHook(() => useThemePreferences(undefined, { onError }));

    act(() => {
      result.current.updatePreferences({ presetId: "legacy" });
    });
    await waitFor(() => expect(result.current.error).toBe("forbidden"));

    // Reverted back to the default preset both in state and the DOM.
    expect(result.current.preferences?.presetId).toBe("pink");
    expect(document.documentElement.dataset.themePreset).toBe("pink");
    expect(onError).toHaveBeenCalledWith("forbidden");
    expect(eventListener).toHaveBeenCalledTimes(1);
    window.removeEventListener(THEME_ERROR_EVENT, eventListener);
  });

  it("falls back to the status code when the APIError body is empty", async () => {
    updatePreferencesMock.mockRejectedValueOnce(new APIError(500, ""));
    const { result } = renderThemeHook(() => useThemePreferences());
    act(() => {
      result.current.updatePreferences({ presetId: "legacy" });
    });
    await waitFor(() => expect(result.current.error).toBe("Request failed (500)"));
  });

  it("uses the Error message and queues a retry for a non-APIError rejection (connectivity)", async () => {
    updatePreferencesMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));
    updatePreferencesMock.mockResolvedValueOnce({
      ...DEFAULT_THEME_PREFERENCES,
      presetId: "legacy",
    });
    const { result } = renderThemeHook(() => useThemePreferences());

    act(() => {
      result.current.updatePreferences({ presetId: "legacy" });
    });
    await waitFor(() => expect(result.current.error).toBe("Failed to fetch"));
    expect(updatePreferencesMock).toHaveBeenCalledTimes(1);

    // Replays the queued mutation once the browser reports connectivity.
    act(() => {
      window.dispatchEvent(new Event("online"));
    });
    await waitFor(() => expect(updatePreferencesMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(result.current.error).toBeNull());
    expect(result.current.preferences?.presetId).toBe("legacy");
  });

  it("reports a generic message and still queues a retry for a non-Error rejection", async () => {
    updatePreferencesMock.mockRejectedValueOnce("raw rejection");
    const { result } = renderThemeHook(() => useThemePreferences());
    act(() => {
      result.current.updatePreferences({ presetId: "legacy" });
    });
    await waitFor(() => expect(result.current.error).toBe("Theme update failed"));
  });

  it("does nothing on 'online' when no update is queued", async () => {
    const { result } = renderThemeHook(() => useThemePreferences());
    act(() => {
      window.dispatchEvent(new Event("online"));
    });
    expect(updatePreferencesMock).not.toHaveBeenCalled();
    expect(result.current.error).toBeNull();
  });
});
