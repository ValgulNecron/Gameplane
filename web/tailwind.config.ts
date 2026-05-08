import type { Config } from "tailwindcss";

// Design tokens mirror the `c:` system in design.pen:
// - primary: orange 500 (#f97316)
// - fonts: Geist sans, JetBrains Mono
// - default theme: dark
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "hsl(var(--bg))",
        surface: "hsl(var(--surface))",
        card: "hsl(var(--card))",
        border: "hsl(var(--border))",
        muted: "hsl(var(--muted))",
        fg: "hsl(var(--fg))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          fg: "hsl(var(--primary-fg))",
        },
        success: "hsl(var(--success))",
        warning: "hsl(var(--warning))",
        danger: "hsl(var(--danger))",
        violet: "hsl(var(--violet))",
      },
      fontFamily: {
        sans: ["Geist", "ui-sans-serif", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "ui-monospace", "monospace"],
      },
      borderRadius: { md: "6px", lg: "10px" },
    },
  },
  plugins: [],
} satisfies Config;
