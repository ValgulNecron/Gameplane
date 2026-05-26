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

  it("maps Running to success", () => {
    render(<PhaseBadge phase="Running" />);
    expect(screen.getByText("Running").className).toContain("bg-success/20");
  });

  it("maps Succeeded (backup phase) to success", () => {
    render(<PhaseBadge phase="Succeeded" />);
    expect(screen.getByText("Succeeded").className).toContain("bg-success/20");
  });

  it("maps Failed to danger", () => {
    render(<PhaseBadge phase="Failed" />);
    expect(screen.getByText("Failed").className).toContain("bg-danger/20");
  });

  it("maps Suspending (restore phase) to warning", () => {
    render(<PhaseBadge phase="Suspending" />);
    expect(screen.getByText("Suspending").className).toContain("bg-warning/20");
  });

  it("maps Resuming (restore phase) to warning", () => {
    render(<PhaseBadge phase="Resuming" />);
    expect(screen.getByText("Resuming").className).toContain("bg-warning/20");
  });

  it("falls back to muted style for unknown phases", () => {
    render(<PhaseBadge phase="Mysterious" />);
    expect(screen.getByText("Mysterious").className).toContain("bg-muted/20");
  });
});
