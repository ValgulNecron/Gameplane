import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PhaseBadge } from "./badge";

describe("PhaseBadge", () => {
  it("shows the phase label", () => {
    render(<PhaseBadge phase="Running" />);
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("falls back to Pending when phase is missing", () => {
    render(<PhaseBadge />);
    expect(screen.getByText("Pending")).toBeInTheDocument();
  });
});
