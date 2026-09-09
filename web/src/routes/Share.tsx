import { useEffect, useState, useRef } from "react";
import { useParams } from "@tanstack/react-router";
import {
  Card,
  Button,
  Spinner,
} from "@heroui/react";
import { ShieldCheck, Copy, Check } from "lucide-react";
import { Shares } from "@/lib/api";
import type { ShareLinkPublic } from "@/types";
import type { AppearanceMode } from "@/components/hero/AppearanceToggle";

// localStorage key for appearance preference (must match AppLayout)
const THEME_STORAGE_KEY = "gameplane-theme";

function readStoredTheme(): AppearanceMode {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === "light" || saved === "dark" || saved === "system") return saved;
  } catch {
    // localStorage unavailable — fall back to system default.
  }
  return "system";
}

function applyTheme(mode: AppearanceMode) {
  const resolved =
    mode === "system"
      ? window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light"
      : mode;

  const html = document.documentElement;
  html.setAttribute("data-theme", resolved);
  if (resolved === "dark") {
    html.classList.add("dark");
  } else {
    html.classList.remove("dark");
  }
}

type State = "loading" | "up" | "asleep-start" | "asleep-viewonly" | "starting" | "invalid";

export function SharePage() {
  const { token } = useParams({ from: "/share/$token" });
  const [state, setState] = useState<State>("loading");
  const [data, setData] = useState<ShareLinkPublic | null>(null);
  const [isStarting, setIsStarting] = useState(false);
  const [copied, setCopied] = useState(false);
  const pollingRef = useRef<number | null>(null);

  // Apply stored appearance preference on mount
  useEffect(() => {
    const mode = readStoredTheme();
    applyTheme(mode);
  }, []);

  // Initial resolve + polling for Starting state
  useEffect(() => {
    let active = true;

    const resolve = async () => {
      try {
        const result = await Shares.resolve(token);
        if (!active) return;

        // Check for neutral/error response (serverName is empty, indicating resolve returned an error)
        if (!result.serverName) {
          setState("invalid");
          return;
        }

        setData(result);

        // Determine state from result
        if (result.status === "Running") {
          setState("up");
        } else if (result.status === "Suspended" || result.status === "Stopped") {
          // For Asleep states, always show Start button by default
          // If user clicks and gets error, we'll transition to invalid
          setState("asleep-start");
        } else if (result.status === "Starting" || result.status === "Pending") {
          setState("starting");
        } else {
          // Any other state treats as invalid
          setState("invalid");
        }
      } catch {
        if (!active) return;
        // All errors (404, 429, auth) map to invalid per FR-005
        setState("invalid");
      }
    };

    void resolve();

    return () => {
      active = false;
    };
  }, [token]);

  // Polling when Starting
  useEffect(() => {
    if (state !== "starting") return;

    const poll = async () => {
      try {
        const result = await Shares.resolve(token);

        // Check for neutral/error response
        if (!result.serverName) {
          setState("invalid");
          if (pollingRef.current) clearInterval(pollingRef.current);
          return;
        }

        if (result.status === "Running") {
          setData(result);
          setState("up");
          if (pollingRef.current) clearInterval(pollingRef.current);
        } else if (
          result.status === "Suspended" ||
          result.status === "Stopped"
        ) {
          // If it went back to asleep after trying to start, show view-only
          // (means user doesn't have permission to start this server)
          setState("asleep-viewonly");
          setData(result);
          if (pollingRef.current) clearInterval(pollingRef.current);
        }
        // If still Starting/Pending, keep polling
      } catch {
        // On error during polling, show invalid (link expired/revoked)
        setState("invalid");
        if (pollingRef.current) clearInterval(pollingRef.current);
      }
    };

    pollingRef.current = window.setInterval(poll, 2000) as unknown as number;

    return () => {
      if (pollingRef.current) clearInterval(pollingRef.current);
    };
  }, [state, token]);

  const handleStart = async () => {
    setIsStarting(true);
    try {
      await Shares.start(token);
      // Transition to Starting state and start polling
      setState("starting");
    } catch {
      // On error, show invalid (rate-limited or link expired)
      setState("invalid");
    } finally {
      setIsStarting(false);
    }
  };

  const handleCopyAddress = () => {
    if (data?.address) {
      const addr = `${data.address.host}:${data.address.port}`;
      navigator.clipboard.writeText(addr).catch(() => {
        // Silently fail if copy doesn't work
      });
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const containerClass = "flex h-full w-full items-center justify-center bg-background p-6";

  return (
    <div className={containerClass}>
      {state === "loading" && (
        <Card className="max-w-md p-8 text-center">
          <div className="flex justify-center">
            <Spinner />
          </div>
        </Card>
      )}

      {state === "up" && data && (
        <Card className="max-w-md p-8">
          <div className="mb-6">
            <div className="mb-4 flex items-center justify-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
                <ShieldCheck className="h-5 w-5 text-primary" />
              </div>
            </div>
            <div className="mb-2 text-center">
              <div className="font-mono text-sm text-muted">gameplane</div>
              <div className="font-mono text-sm text-muted">shared server link</div>
            </div>
            <h1 className="font-mono text-2xl font-semibold">{data.serverName}</h1>
            <div className="mt-3 flex justify-center">
              <span className="inline-block rounded-full bg-success/20 px-3 py-1 text-sm font-medium text-success">
                Online
              </span>
            </div>
          </div>

          <div className="space-y-4">
            <div>
              <div className="text-xs text-muted mb-1">CONNECT AT</div>
              {data.address ? (
                <div className="flex items-center gap-2 rounded-lg bg-surface p-3">
                  <code className="flex-1 text-sm font-mono">
                    {data.address.host}:{data.address.port}
                  </code>
                  <button
                    type="button"
                    className="p-1 text-muted hover:text-fg transition-colors"
                    onClick={handleCopyAddress}
                    title="Copy address"
                  >
                    {copied ? (
                      <Check className="h-4 w-4 text-success" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </button>
                </div>
              ) : (
                <div className="text-sm text-muted">Not exposed</div>
              )}
            </div>

            {data.playersOnline !== undefined && (
              <div>
                <div className="text-xs text-muted mb-1">PLAYERS</div>
                <div className="text-sm">{data.playersOnline} players online</div>
              </div>
            )}
          </div>
        </Card>
      )}

      {state === "asleep-start" && data && (
        <Card className="max-w-md p-8">
          <div className="mb-6">
            <div className="mb-4 flex items-center justify-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
                <ShieldCheck className="h-5 w-5 text-primary" />
              </div>
            </div>
            <div className="mb-2 text-center">
              <div className="font-mono text-sm text-muted">gameplane</div>
              <div className="font-mono text-sm text-muted">shared server link</div>
            </div>
            <h1 className="font-mono text-2xl font-semibold">{data.serverName}</h1>
            <div className="mt-3 flex justify-center">
              <span className="inline-block rounded-full bg-secondary/20 px-3 py-1 text-sm font-medium text-secondary">
                Asleep
              </span>
            </div>
          </div>

          <div className="mb-6">
            <p className="text-sm text-muted">
              This server is asleep to save resources. Start it and it&apos;ll be ready in a
              minute or two.
            </p>
          </div>

          <Button
            variant="primary"
            className="w-full"
            isDisabled={isStarting}
            onPress={handleStart}
          >
            {isStarting ? "Starting..." : "Start server"}
          </Button>
        </Card>
      )}

      {state === "asleep-viewonly" && data && (
        <Card className="max-w-md p-8">
          <div className="mb-6">
            <div className="mb-4 flex items-center justify-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
                <ShieldCheck className="h-5 w-5 text-primary" />
              </div>
            </div>
            <div className="mb-2 text-center">
              <div className="font-mono text-sm text-muted">gameplane</div>
              <div className="font-mono text-sm text-muted">shared server link</div>
            </div>
            <h1 className="font-mono text-2xl font-semibold">{data.serverName}</h1>
            <div className="mt-3 flex justify-center">
              <span className="inline-block rounded-full bg-secondary/20 px-3 py-1 text-sm font-medium text-secondary">
                Asleep
              </span>
            </div>
          </div>

          <p className="text-sm text-muted">
            This server is asleep right now. Check back later, or ask the server owner to
            start it.
          </p>
        </Card>
      )}

      {state === "starting" && data && (
        <Card className="max-w-md p-8">
          <div className="mb-6">
            <div className="mb-4 flex items-center justify-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
                <ShieldCheck className="h-5 w-5 text-primary" />
              </div>
            </div>
            <div className="mb-2 text-center">
              <div className="font-mono text-sm text-muted">gameplane</div>
              <div className="font-mono text-sm text-muted">shared server link</div>
            </div>
            <h1 className="font-mono text-2xl font-semibold">{data.serverName}</h1>
            <div className="mt-3 flex justify-center">
              <div className="inline-flex items-center gap-2 rounded-full border border-warning/50 bg-warning/10 px-3 py-1">
                <Spinner size="sm" color="warning" />
                <span className="text-sm font-medium text-warning">Starting...</span>
              </div>
            </div>
          </div>

          <p className="text-sm text-muted">
            The server is waking up. This usually takes a minute or two — this page
            updates on its own.
          </p>
        </Card>
      )}

      {state === "invalid" && (
        <Card className="max-w-md p-8">
          <div className="mb-6">
            <div className="mb-4 flex items-center justify-center">
              <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
                <ShieldCheck className="h-5 w-5 text-primary" />
              </div>
            </div>
            <div className="mb-2 text-center">
              <div className="font-mono text-sm text-muted">gameplane</div>
              <div className="font-mono text-sm text-muted">shared server link</div>
            </div>
          </div>

          <div className="text-center">
            <h1 className="font-mono text-2xl font-semibold">Link not available</h1>
            <p className="mt-2 text-sm text-muted">
              This link may be invalid, expired, or revoked. Ask the person who shared it
              for a new one.
            </p>
          </div>
        </Card>
      )}
    </div>
  );
}
