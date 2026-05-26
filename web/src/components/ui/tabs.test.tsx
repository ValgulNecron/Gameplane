import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TabBar } from "./tabs";

describe("TabBar", () => {
  const items = [
    { key: "all", label: "All", count: 7 },
    { key: "running", label: "Running", count: 3 },
    { key: "stopped", label: "Stopped" },
  ] as const;

  it("renders labels and counts when provided", () => {
    render(<TabBar items={items} value="all" onChange={() => {}} />);
    expect(screen.getByText("All")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    // Stopped has no count badge.
    expect(screen.getByText("Stopped")).toBeInTheDocument();
  });

  it("calls onChange with the picked key", () => {
    const onChange = vi.fn();
    render(<TabBar items={items} value="all" onChange={onChange} />);
    fireEvent.click(screen.getByText("Running"));
    expect(onChange).toHaveBeenCalledWith("running");
  });
});
