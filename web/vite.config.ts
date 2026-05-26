import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

// __dirname isn't defined in ESM; derive it from import.meta.url.
const here = path.dirname(fileURLToPath(import.meta.url));

// API target for the dev server's proxy. Defaults to the standard local
// process; Playwright live-mode points it at a kubectl port-forward by
// setting KESTREL_API_URL=http://localhost:18080 before spawning vite.
const apiTarget = process.env.KESTREL_API_URL ?? "http://localhost:8000";
const wsTarget = apiTarget.replace(/^http/, "ws");

// htmlBypass returns "/index.html" for HTML navigations (Accept includes
// text/html), telling vite to serve the SPA shell instead of forwarding
// to the API. fetch() / XHR calls fall through to the proxy.
function htmlBypass(req: { headers: Record<string, string | string[] | undefined> }): string | undefined {
  const accept = req.headers.accept;
  const text = Array.isArray(accept) ? accept.join(",") : (accept ?? "");
  if (text.includes("text/html")) return "/index.html";
  return undefined;
}

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(here, "./src") },
  },
  server: {
    port: 5173,
    proxy: {
      // Proxy API + WS traffic to the in-cluster API during `npm run dev`.
      // The SPA uses unprefixed paths (api.ts → fetch("/users/me") etc.),
      // and several of those paths (/servers, /modules, /backups, /admin)
      // are ALSO front-end SPA routes. We disambiguate by Accept header:
      // a browser HTML navigation sends Accept: text/html and bypasses
      // the proxy (vite serves index.html), while fetch()/XHR sends
      // Accept: */* or application/json and is forwarded to the API.
      //
      // /auth, /events, /healthz, /ws, /cluster, /templates, /schedules,
      // /restores, /backup-destinations, /module-sources, /users have
      // no SPA-route collision so they're forwarded unconditionally.
      "/api":    { target: apiTarget, changeOrigin: true, rewrite: (p) => p.replace(/^\/api/, "") },
      "/auth":   { target: apiTarget, changeOrigin: true },
      "/events": { target: apiTarget, changeOrigin: true },
      "/healthz": { target: apiTarget, changeOrigin: true },
      "/users":  { target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/cluster":{ target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/servers": { target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/templates": { target: apiTarget, changeOrigin: true },
      "/backups": { target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/schedules": { target: apiTarget, changeOrigin: true },
      "/restores": { target: apiTarget, changeOrigin: true },
      "/backup-destinations": { target: apiTarget, changeOrigin: true },
      "/modules": { target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/module-sources": { target: apiTarget, changeOrigin: true },
      "/admin":  { target: apiTarget, changeOrigin: true, bypass: htmlBypass },
      "/ws":     { target: wsTarget,  ws: true },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          monaco: ["@monaco-editor/react", "monaco-editor"],
          xterm: ["@xterm/xterm", "@xterm/addon-fit"],
        },
      },
    },
  },
});
