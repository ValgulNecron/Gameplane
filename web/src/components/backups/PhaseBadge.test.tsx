import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PhaseBadge } from "./PhaseBadge";

describe("PhaseBadge (backups)", () => {
  it("falls back to Pending when phase is undefined", () => {
    render(<PhaseBadge />);
    expect(screen.getByText("Pending")).toBeInTheDocument();
  });

  it("shows the supplied phase", () => {
    render(<PhaseBadge phase="Succeeded" />);
    const el = screen.getByText("Succeeded");
    expect(el.className).toContain("bg-success/20");
  });

  it("falls back to muted style for unknown phases", () => {
    render(<PhaseBadge phase="Mysterious" />);
    const el = screen.getByText("Mysterious");
    expect(el.className).toContain("bg-muted/20");
  });

  it("maps Failed to danger style", () => {
    render(<PhaseBadge phase="Failed" />);
    expect(screen.getByText("Failed").className).toContain("bg-danger/20");
  });

  it("maps Running to primary", () => {
    render(<PhaseBadge phase="Running" />);
    expect(screen.getByText("Running").className).toContain("bg-primary/20");
  });

  it("maps Suspending to warning", () => {
    render(<PhaseBadge phase="Suspending" />);
    expect(screen.getByText("Suspending").className).toContain("bg-warning/20");
  });
});
