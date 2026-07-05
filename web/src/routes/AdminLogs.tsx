import { useCallback, useEffect, useRef, useState } from "react";
import { Download } from "lucide-react";
import { APIError } from "@/lib/api";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

type LogComponent = "api" | "operator";

const COMPONENTS: Array<{ value: LogComponent; label: string }> = [
  { value: "api", label: "API server" },
  { value: "operator", label: "Operator" },
];

const TAIL_OPTIONS = [100, 500, 1000, 5000] as const;

// The scrollback buffer is capped so a chatty follow stream can't grow the
// tab's memory without bound; the head is trimmed at a line boundary.
const MAX_BUFFER = 2 * 1024 * 1024;
const RECONNECT_DELAY_MS = 1_000;

// capBuffer keeps at most `max` characters of scrollback, dropping the
// oldest lines first. Exported for tests.
export function capBuffer(s: string, max: number = MAX_BUFFER): string {
  if (s.length <= max) return s;
  const cut = s.slice(s.length - max);
  const nl = cut.indexOf("\n");
  return nl >= 0 ? cut.slice(nl + 1) : cut;
}

function logsURL(
  component: LogComponent,
  tailLines: number,
  follow: boolean,
  pod?: string,
): string {
  const params = new URLSearchParams({
    tailLines: String(tailLines),
    follow: String(follow),
  });
  if (pod) params.set("pod", pod);
  return `/admin/system-logs/${component}?${params.toString()}`;
}

// streamSystemLogs fetches one plaintext log stream and forwards decoded
// chunks as they arrive. The JSON wrapper in lib/api.ts can't consume a
// text stream, so this follows its conventions (relative URL, cookie
// credentials, APIError on non-2xx) with a ReadableStream reader instead.
async function streamSystemLogs(opts: {
  url: string;
  signal: AbortSignal;
  onPod: (pod: string) => void;
  onChunk: (text: string) => void;
}): Promise<void> {
  const res = await fetch(opts.url, {
    credentials: "include",
    signal: opts.signal,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new APIError(res.status, text);
  }
  const pod = res.headers.get("X-Gameplane-Pod");
  if (pod) opts.onPod(pod);
  if (!res.body) return;
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    opts.onChunk(decoder.decode(value, { stream: true }));
  }
  const rest = decoder.decode();
  if (rest) opts.onChunk(rest);
}

export function AdminLogsPage() {
  const [component, setComponent] = useState<LogComponent>("api");
  const [tail, setTail] = useState(500);
  const [follow, setFollow] = useState(false);
  const [pod, setPod] = useState("");
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  // Sticks the panel to the bottom on new output; a manual scroll up
  // releases it, scrolling back down re-engages it.
  const stickRef = useRef(true);

  useEffect(() => {
    const ctrl = new AbortController();
    let timer: ReturnType<typeof setTimeout> | undefined;
    // Pin follow reconnects to the pod the first response picked, so the
    // stream doesn't hop between replicas; a component/tail change resets it.
    let currentPod = "";
    setText("");
    setPod("");
    setError("");
    stickRef.current = true;

    const run = async () => {
      for (;;) {
        try {
          await streamSystemLogs({
            url: logsURL(component, tail, follow, currentPod || undefined),
            signal: ctrl.signal,
            onPod: (p) => {
              currentPod = p;
              setPod(p);
            },
            onChunk: (chunk) => setText((prev) => capBuffer(prev + chunk)),
          });
        } catch (err) {
          if (ctrl.signal.aborted) return;
          setError(err instanceof Error ? err.message : String(err));
          return; // only clean stream ends reconnect; errors stop the loop
        }
        // A non-follow fetch is one-shot. The server caps follow streams at
        // ~50s, so a clean end while following just means "reconnect".
        if (!follow || ctrl.signal.aborted) return;
        await new Promise<void>((resolve) => {
          timer = setTimeout(resolve, RECONNECT_DELAY_MS);
        });
        if (ctrl.signal.aborted) return;
      }
    };
    void run();
    return () => {
      ctrl.abort();
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [component, tail, follow]);

  // Auto-scroll to the newest output unless the user scrolled up to read.
  useEffect(() => {
    const el = scrollerRef.current;
    if (el && stickRef.current) el.scrollTop = el.scrollHeight;
  }, [text]);

  const onScroll = useCallback(() => {
    const el = scrollerRef.current;
    if (!el) return;
    stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  }, []);

  const downloadLogs = async () => {
    setError("");
    try {
      const res = await fetch(logsURL(component, tail, false, pod || undefined), {
        credentials: "include",
      });
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        throw new APIError(res.status, body);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `gameplane-${component}-${new Date().toISOString().replace(/[:.]/g, "-")}.log`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex h-full flex-col gap-4 p-6">
      <PageHeader
        title="System logs"
        subtitle="Live logs from the Gameplane control-plane pods."
      />

      <div className="flex flex-wrap items-center gap-3">
        <div className="flex gap-1 rounded-md border border-border bg-surface/40 p-1">
          {COMPONENTS.map((c) => (
            <button
              key={c.value}
              type="button"
              onClick={() => setComponent(c.value)}
              aria-pressed={component === c.value}
              className={cn(
                "rounded px-3 py-1 text-xs font-medium",
                component === c.value
                  ? "bg-primary/15 text-primary"
                  : "text-muted hover:text-fg",
              )}
            >
              {c.label}
            </button>
          ))}
        </div>

        <label className="flex items-center gap-2 text-xs text-muted">
          Tail
          <Select
            aria-label="Tail lines"
            className="w-32"
            value={String(tail)}
            onValueChange={(v) => setTail(Number(v))}
            options={TAIL_OPTIONS.map((n) => ({
              value: String(n),
              label: `${n} lines`,
            }))}
          />
        </label>

        <label className="flex items-center gap-2 text-xs text-muted">
          Follow
          <Switch checked={follow} onCheckedChange={setFollow} aria-label="Follow" />
        </label>

        <span
          className="inline-flex h-6 items-center rounded bg-primary/10 px-2 font-mono text-xs text-primary"
          title="Pod currently streamed"
        >
          {pod || "—"}
        </span>

        <div className="ml-auto">
          <Button variant="outline" onClick={() => void downloadLogs()}>
            <Download className="h-4 w-4" /> Download
          </Button>
        </div>
      </div>

      {error && <div className="text-xs text-danger">{error}</div>}

      <div
        ref={scrollerRef}
        onScroll={onScroll}
        className="min-h-0 flex-1 overflow-auto rounded-md border border-border bg-[#0b0b0d] font-mono text-xs scrollbar-thin"
      >
        {text ? (
          <pre className="whitespace-pre-wrap break-words px-4 py-3 leading-[18px]">
            {text}
          </pre>
        ) : (
          <div className="flex h-full items-center justify-center text-muted">
            {error ? "No output." : "Waiting for output…"}
          </div>
        )}
      </div>
    </div>
  );
}
