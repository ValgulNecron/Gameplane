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
      "--accent": "oklch(59.16% 0.2180 0.58)",
      "--surface": "oklch(98.34% 0.0100 345.41)",
      "--background": "oklch(100.00% 0.0000 89.88)",
      "--foreground": "oklch(21.67% 0.0502 347.62)",
      "--border": "oklch(86.09% 0.0556 350.54)",
      "--muted": "oklch(47.33% 0.0441 343.67)",
      "--accent-soft": "oklch(100.00% 0.0000 89.88)",
      "--surface-secondary": "oklch(92.30% 0.0335 349.09)",
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
      "--accent": "oklch(69.50% 0.2229 355.31)",
      "--surface": "oklch(22.27% 0.0119 300.63)",
      "--background": "oklch(18.02% 0.0063 300.93)",
      "--foreground": "oklch(96.68% 0.0057 308.40)",
      "--border": "oklch(28.79% 0.0167 300.56)",
      "--muted": "oklch(68.92% 0.0213 305.13)",
      "--accent-soft": "oklch(24.65% 0.0537 348.54)",
      "--surface-secondary": "oklch(20.03% 0.0103 303.61)",
    };

    for (const [name, value] of Object.entries(expected)) {
      it(`${name} should be ${value}`, () => {
        expect(darkTokens[name]).toBe(value);
      });
    }
  });

  describe("focus and link are explicit values", () => {
    it("light mode: --focus and --link have explicit oklch values", () => {
      expect(lightTokens["--focus"]).toBe("oklch(54.13% 0.2466 293.01)");
      expect(lightTokens["--link"]).toBe("oklch(54.61% 0.2152 262.88)");
    });

    it("dark mode: --focus and --link have explicit oklch values", () => {
      expect(darkTokens["--focus"]).toBe("oklch(70.90% 0.1592 293.54)");
      expect(darkTokens["--link"]).toBe("oklch(76.21% 0.1231 256.39)");
    });
  });
});
