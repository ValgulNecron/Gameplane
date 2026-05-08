import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov", "json-summary"],
      reportsDirectory: "./coverage",
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/main.tsx",
        "src/router/**",
        "src/**/*.d.ts",
        "src/types.ts",
        "src/test/**",
        "src/styles/**",
        "**/*.test.{ts,tsx}",
        "src/lib/config.ts",
      ],
      // Ratchet up via follow-up PRs as more tests land.
      thresholds: {
        lines: 84,
        functions: 69,
        branches: 80,
        statements: 84,
      },
    },
  },
});
