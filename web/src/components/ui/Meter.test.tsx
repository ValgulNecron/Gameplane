import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Meter } from "./Meter";

describe("Meter", () => {
  it("renders a rounded percentage and a proportional fill", () => {
    const { container } = render(<Meter label="CPU" pct={42.6} accent="primary" />);
    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getByText("43%")).toBeInTheDocument();
    const fill = container.querySelector(".bg-accent") as HTMLElement;
    expect(fill.style.width).toBe("42.6%");
  });

  it("clamps the fill width to 100% but keeps the true percentage in the label", () => {
    const { container } = render(<Meter label="Storage" pct={118} accent="warning" />);
    // Honest overcommit signal: the number is not hidden even past 100%.
    expect(screen.getByText("118%")).toBeInTheDocument();
    const fill = container.querySelector(".bg-warning") as HTMLElement;
    expect(fill.style.width).toBe("100%");
  });

  it("renders an em dash and no fill when unknown, instead of a false 0%", () => {
    const { container } = render(<Meter label="CPU" pct={0} unknown accent="primary" />);
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
    expect(container.querySelector(".bg-accent")).toBeNull();
  });

  it("renders the sub label when provided", () => {
    render(<Meter label="Memory" pct={50} sub="4 GB / 8 GB" accent="violet" />);
    expect(screen.getByText("4 GB / 8 GB")).toBeInTheDocument();
  });

  it("applies the correct accent color class for each color variant", () => {
    const variants: Array<{ accent: "primary" | "violet" | "success" | "warning" | "danger"; expectedClass: string }> = [
      { accent: "primary", expectedClass: "bg-accent" },
      { accent: "violet", expectedClass: "bg-violet" },
      { accent: "success", expectedClass: "bg-success" },
      { accent: "warning", expectedClass: "bg-warning" },
      { accent: "danger", expectedClass: "bg-danger" },
    ];

    variants.forEach(({ accent, expectedClass }) => {
      const { container } = render(<Meter label={`Test-${accent}`} pct={50} accent={accent} />);
      const fill = container.querySelector(`.${expectedClass}`) as HTMLElement;
      expect(fill).toBeInTheDocument();
      expect(fill.style.width).toBe("50%");
    });
  });

  it("clamps negative percentages to 0%", () => {
    const { container } = render(<Meter label="CPU" pct={-10} accent="danger" />);
    expect(screen.getByText("0%")).toBeInTheDocument();
    const fill = container.querySelector(".bg-danger") as HTMLElement;
    expect(fill.style.width).toBe("0%");
  });

  it("renders unknown state with non-zero pct, showing dash instead of percentage", () => {
    const { container } = render(<Meter label="Memory" pct={75} unknown accent="success" />);
    expect(screen.getByText("—")).toBeInTheDocument();
    expect(screen.queryByText("75%")).not.toBeInTheDocument();
    expect(container.querySelector(".bg-success")).toBeNull();
  });

  it("uses text-foreground for percentage when not unknown, and text-muted when unknown", () => {
    const { rerender } = render(<Meter label="CPU" pct={50} accent="primary" />);
    const percentSpan = screen.getByText("50%") as HTMLElement;
    expect(percentSpan).toHaveClass("text-foreground");

    rerender(<Meter label="CPU" pct={50} unknown accent="primary" />);
    const dashSpan = screen.getByText("—") as HTMLElement;
    expect(dashSpan).toHaveClass("text-muted");
  });
});
