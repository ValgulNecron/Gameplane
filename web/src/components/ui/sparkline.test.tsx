import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Sparkline } from "./sparkline";

describe("Sparkline", () => {
  it("renders nothing for fewer than two points", () => {
    const { container } = render(<Sparkline data={[]} />);
    expect(container.querySelector("svg")).toBeNull();
    const { container: one } = render(<Sparkline data={[5]} />);
    expect(one.querySelector("svg")).toBeNull();
  });

  it("draws a polyline with one point per sample", () => {
    const { container } = render(<Sparkline data={[1, 5, 3]} />);
    const poly = container.querySelector("polyline");
    expect(poly).not.toBeNull();
    expect(poly?.getAttribute("points")?.trim().split(/\s+/).length).toBe(3);
  });
});
