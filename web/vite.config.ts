import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { fileURLToPath } from "node:url";

// __dirname isn't defined in ESM; derive it from import.meta.url.
const here = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(here, "./src") },
  },
  server: {
    port: 5173,
    proxy: {
      // Proxy API + WS traffic to the in-cluster API during `npm run dev`.
      "/api":    { target: "http://localhost:8000", changeOrigin: true, rewrite: (p) => p.replace(/^\/api/, "") },
      "/auth":   { target: "http://localhost:8000", changeOrigin: true },
      "/events": { target: "http://localhost:8000", changeOrigin: true },
      "/ws":     { target: "ws://localhost:8000",   ws: true },
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
