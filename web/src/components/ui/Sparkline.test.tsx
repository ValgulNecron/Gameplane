import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import { Sparkline } from "./Sparkline";

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

  it("applies custom className and respects custom strokeWidth", () => {
    const { container } = render(
      <Sparkline data={[1, 2, 3]} className="custom-class" strokeWidth={2.5} />
    );
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg?.classList.contains("custom-class")).toBe(true);
    const poly = container.querySelector("polyline");
    expect(poly?.getAttribute("stroke-width")).toBe("2.5");
  });

  it("handles flat data (all values identical) by scaling with unit range", () => {
    const { container } = render(<Sparkline data={[5, 5, 5]} />);
    const poly = container.querySelector("polyline");
    expect(poly).not.toBeNull();
    const points = poly?.getAttribute("points")?.trim().split(/\s+/) || [];
    // With identical values, all points should have the same y-coordinate
    const yCoords = points.map((p) => p.split(",")[1]);
    expect(new Set(yCoords).size).toBe(1);
  });

  it("renders with default stroke width of 1.5 when not specified", () => {
    const { container } = render(<Sparkline data={[1, 2, 3]} />);
    const poly = container.querySelector("polyline");
    expect(poly?.getAttribute("stroke-width")).toBe("1.5");
  });

  it("has aria-hidden set to true", () => {
    const { container } = render(<Sparkline data={[1, 2, 3]} />);
    const svg = container.querySelector("svg");
    expect(svg?.getAttribute("aria-hidden")).toBe("true");
  });

  it("positions points correctly across the full range", () => {
    const { container } = render(<Sparkline data={[0, 10, 5]} />);
    const poly = container.querySelector("polyline");
    const points = poly?.getAttribute("points")?.trim().split(/\s+/) || [];
    const coords = points.map((p) => {
      const [x, y] = p.split(",").map(Number);
      return { x, y };
    });
    // First point should be at x=0
    expect(coords[0].x).toBeCloseTo(0, 1);
    // Last point should be at x=100
    expect(coords[2].x).toBeCloseTo(100, 1);
    // First point (0) should be at bottom, last (5) should be in middle
    expect(coords[0].y).toBeGreaterThan(coords[2].y);
  });
});
