import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Meter } from "./meter";

describe("Meter", () => {
  it("renders a rounded percentage and a proportional fill", () => {
    const { container } = render(<Meter label="CPU" pct={42.6} accent="primary" />);
    expect(screen.getByText("CPU")).toBeInTheDocument();
    expect(screen.getByText("43%")).toBeInTheDocument();
    const fill = container.querySelector(".bg-primary") as HTMLElement;
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
    expect(container.querySelector(".bg-primary")).toBeNull();
  });

  it("renders the sub label when provided", () => {
    render(<Meter label="Memory" pct={50} sub="4 GB / 8 GB" accent="violet" />);
    expect(screen.getByText("4 GB / 8 GB")).toBeInTheDocument();
  });
});
