import { useEffect, useState, type ReactNode } from "react";
import {
  Archive,
  Cpu,
  Gauge,
  KeyRound,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { APIError } from "@/lib/api";
import { Auth } from "@/lib/endpoints";
import type { LoginProvider } from "@/types";

// IMPORTANT: This pre-auth surface must not display any internal
// cluster metrics, server counts, hostnames, or version strings. The
// right panel is static product-marketing content only.
export function LoginPage() {
  const [u, setU] = useState("");
  const [p, setP] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  // Enabled login methods, fetched pre-auth. Defaults to local-only so a
  // failed/slow fetch never blocks password sign-in.
  const [oidc, setOidc] = useState<LoginProvider | null>(null);
  // Self-service reset isn't wired up (admins reset via the CLI), so "Forgot?"
  // just reveals an inline hint rather than linking to a non-existent flow.
  const [forgot, setForgot] = useState(false);

  useEffect(() => {
    let active = true;
    void Auth.providers()
      .then((r) => {
        if (active) setOidc(r.providers.find((x) => x.kind === "oidc") ?? null);
      })
      .catch(() => {
        /* local-only fallback — leave oidc null */
      });
    return () => {
      active = false;
    };
  }, []);

  return (
    <div className="grid h-full grid-cols-1 md:grid-cols-2">
      <section className="flex items-center justify-center bg-background p-8">
        <div className="w-full max-w-sm rounded-xl border border-border bg-card p-8">
          <div className="mb-8 flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/15">
              <ShieldCheck className="h-5 w-5 text-primary" />
            </div>
            <div>
              <div className="font-mono text-lg font-semibold">kestrel</div>
              <div className="text-xs text-muted">game server control panel</div>
            </div>
          </div>

          <div className="mb-6">
            <h1 className="text-2xl font-semibold">Sign in</h1>
            <p className="pt-1 text-sm text-muted">Welcome to Kestrel.</p>
          </div>

          <form
            className="space-y-4"
            onSubmit={async (e) => {
              e.preventDefault();
              setErr(null);
              setBusy(true);
              try {
                await Auth.login({ username: u, password: p });
                location.assign("/");
              } catch (e) {
                setErr(e instanceof APIError ? "Invalid credentials" : "Network error");
              } finally {
                setBusy(false);
              }
            }}
          >
            <div className="space-y-1.5">
              <label htmlFor="username" className="block text-xs text-muted">
                Email or username
              </label>
              <Input
                id="username"
                name="username"
                autoComplete="username"
                autoFocus
                value={u}
                onChange={(e) => setU(e.target.value)}
              />
            </div>
            <div className="space-y-1.5">
              {/* The "Forgot?" control must sit OUTSIDE the <label>: a <label>
                  that wraps a second interactive element steals that element's
                  accessible name and mis-associates the field, so screen
                  readers (and tests) resolve the label to the button, not the
                  password input. */}
              <div className="flex items-center justify-between">
                <label htmlFor="password" className="block text-xs text-muted">
                  Password
                </label>
                <button
                  type="button"
                  className="text-xs text-primary hover:underline"
                  onClick={() => setForgot((v) => !v)}
                >
                  Forgot?
                </button>
              </div>
              <Input
                id="password"
                type="password"
                name="password"
                autoComplete="current-password"
                value={p}
                onChange={(e) => setP(e.target.value)}
              />
              {forgot && (
                <p className="text-xs text-muted">
                  Contact your administrator to reset your password.
                </p>
              )}
            </div>
            {err && <p className="text-sm text-danger">{err}</p>}
            <Button type="submit" className="w-full rounded-full" size="lg" disabled={busy}>
              Sign in →
            </Button>
            {oidc && (
              <>
                <div className="relative py-2 text-center text-[11px] uppercase tracking-widest text-muted">
                  <span className="relative z-10 bg-background px-2">or</span>
                  <span className="absolute inset-x-0 top-1/2 h-px -translate-y-1/2 bg-border" />
                </div>
                <Button
                  type="button"
                  variant="outline"
                  className="w-full rounded-full"
                  size="lg"
                  onClick={() => location.assign(Auth.oidcStartURL())}
                >
                  <KeyRound className="h-4 w-4" />
                  Continue with {oidc.label}
                </Button>
              </>
            )}
          </form>

          <p className="mt-10 text-center text-[11px] text-muted">
            AGPL-3.0 licensed
          </p>
        </div>
      </section>

      <section className="hidden border-l border-border bg-surface/40 p-12 md:flex md:flex-col md:justify-center">
        <div className="max-w-md">
          <div className="mb-2 text-[11px] uppercase tracking-widest text-muted">
            AGPL-3.0
          </div>
          <h2 className="text-3xl font-semibold leading-tight">
            Kubernetes-native<br />game server hosting.
          </h2>
          <p className="mt-4 text-sm text-muted">
            An open-source alternative to proprietary game panels — CRDs, Operators, and
            StatefulSets all the way down.
          </p>
          <ul className="mt-10 space-y-4 text-sm">
            <MarketingRow
              icon={<Gauge className="h-4 w-4 text-primary" />}
              title="Deploy any game"
              body="Minecraft, Valheim, Factorio, Palworld, ARK, CS2, Terraria. Template-driven."
            />
            <MarketingRow
              icon={<Cpu className="h-4 w-4 text-primary" />}
              title="Scale with your cluster"
              body="Single-node k3s or multi-node bare metal or cloud."
            />
            <MarketingRow
              icon={<Archive className="h-4 w-4 text-primary" />}
              title="Backups, auto-restart, rolling upgrades"
              body="WAL, production-grade defaults out of the box."
            />
            <MarketingRow
              icon={<Terminal className="h-4 w-4 text-primary" />}
              title="GitOps-friendly"
              body="kubectl get gameservers — just works."
            />
          </ul>
        </div>
      </section>
    </div>
  );
}

function MarketingRow({ icon, title, body }: { icon: ReactNode; title: string; body: string }) {
  return (
    <li className="flex gap-3">
      <div className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-primary/10">
        {icon}
      </div>
      <div>
        <div className="text-sm text-fg">{title}</div>
        <div className="pt-0.5 text-xs text-muted">{body}</div>
      </div>
    </li>
  );
}

