import { describe, it, expect } from "vitest";
import cssText from "../styles/globals.css?raw";

// Note: jsdom does not compute custom properties reliably, so this test
// parses the stylesheet source directly instead of using getComputedStyle().

/**
 * Parse CSS custom property declarations out of the first `:root { ... }`
 * block, or the `.dark { ... }` block, in a stylesheet's source text.
 * Returns a map of property names to their raw declared values (trailing
 * comments stripped), e.g. { "--accent": "oklch(69.11% 0.1944 44.01)" }.
 */
function parseTokens(cssSource: string, selectorPrefix: ":root" | ".dark"): Record<string, string> {
  // Match the first block whose selector list contains the given prefix
  // (globals.css lists several selectors sharing one declaration block,
  // e.g. `:root,\n.light,\n.default, ... {`).
  const regex =
    selectorPrefix === ".dark"
      ? /\.dark[^{]*\{([^}]+)\}/
      : /:root[^{]*\{([^}]+)\}/;
  const match = cssSource.match(regex);

  if (!match) {
    throw new Error(`Could not find ${selectorPrefix} block in CSS`);
  }

  const declarationText = match[1];
  const tokens: Record<string, string> = {};

  // Match each property: --name: value; (dropping a trailing /* comment */)
  const propRegex = /--([\w-]+):\s*([^;]+);/g;
  let propMatch;
  while ((propMatch = propRegex.exec(declarationText)) !== null) {
    const name = `--${propMatch[1]}`;
    const value = propMatch[2].replace(/\/\*.*?\*\//g, "").trim();
    tokens[name] = value;
  }

  return tokens;
}

describe("theme tokens", () => {
  const lightTokens = parseTokens(cssText, ":root");
  const darkTokens = parseTokens(cssText, ".dark");

  describe("light mode", () => {
    // Contract values from specs/014-heroui-web-rebuild/contracts/theme-tokens.md
    const expected: Record<string, string> = {
      "--accent": "oklch(69.11% 0.1944 44.01)",
      "--surface": "oklch(100.00% 0.0000 0.00)",
      "--background": "oklch(100.00% 0.0000 0.00)",
      "--foreground": "oklch(12.88% 0.0254 260.23)",
      "--border": "oklch(92.76% 0.0058 264.53)",
      "--muted": "oklch(55.10% 0.0234 264.36)",
    };

    for (const [name, value] of Object.entries(expected)) {
      it(`${name} should be ${value}`, () => {
        expect(lightTokens[name]).toBe(value);
      });
    }
  });

  describe("dark mode", () => {
    // Contract values from specs/014-heroui-web-rebuild/contracts/theme-tokens.md
    const expected: Record<string, string> = {
      "--accent": "oklch(70.49% 0.1867 47.60)",
      "--surface": "oklch(22.64% 0.0000 0.00)",
      "--background": "oklch(16.84% 0.0000 0.00)",
      "--foreground": "oklch(97.02% 0.0000 0.00)",
      "--border": "oklch(28.09% 0.0000 0.00)",
      "--muted": "oklch(66.65% 0.0000 0.00)",
    };

    for (const [name, value] of Object.entries(expected)) {
      it(`${name} should be ${value}`, () => {
        expect(darkTokens[name]).toBe(value);
      });
    }
  });

  describe("focus and link alias the accent token", () => {
    it("light mode: --focus and --link equal var(--accent)", () => {
      expect(lightTokens["--focus"]).toBe("var(--accent)");
      expect(lightTokens["--link"]).toBe("var(--accent)");
    });

    it("dark mode: --focus and --link equal var(--accent)", () => {
      expect(darkTokens["--focus"]).toBe("var(--accent)");
      expect(darkTokens["--link"]).toBe("var(--accent)");
    });
  });
});
