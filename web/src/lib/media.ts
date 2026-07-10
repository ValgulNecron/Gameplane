import { useEffect, useState } from "react";

// useMediaQuery mirrors a CSS media query in JS so a component can pick
// between two *structurally different* renders (e.g. a data table vs. a
// stacked card list) without mounting both. Mounting both and toggling
// visibility with responsive Tailwind classes (the usual "hidden md:block"
// pattern used elsewhere in this file tree) would duplicate every row's
// accessible text in the DOM — harmless in a real browser, but it breaks
// single-match queries (getByText/findByText) in tests, since jsdom never
// evaluates the stylesheet that would otherwise hide one of the two.
//
// In tests, src/test/setup.ts stubs window.matchMedia to always report
// `matches: false`, so this hook consistently resolves to the "desktop"
// branch there — matching this repo's existing (desktop-oriented) test
// fixtures without any changes to them.
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window !== "undefined" && typeof window.matchMedia === "function"
      ? window.matchMedia(query).matches
      : false,
  );

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const mql = window.matchMedia(query);
    const onChange = () => setMatches(mql.matches);
    onChange();
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, [query]);

  return matches;
}
