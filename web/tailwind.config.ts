import type { Config } from "tailwindcss";

// Design tokens mirror the `c:` system in design.pen:
// - primary: orange 500 (#f97316)
// - fonts: Geist sans, JetBrains Mono
// - default theme: dark
// In Tailwind CSS 4, theme tokens are defined in @theme blocks in globals.css
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
} satisfies Config;
