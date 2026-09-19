import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppLoadingSkeleton } from "./AppLoadingSkeleton";

describe("AppLoadingSkeleton", () => {
  it("renders", () => {
    render(<AppLoadingSkeleton />);
    expect(screen.getByText("gameplane")).toBeInTheDocument();
  });

  it("exposes an accessible busy status labeled Loading", () => {
    render(<AppLoadingSkeleton />);
    const status = screen.getByRole("status", { name: "Loading" });
    expect(status).toBeInTheDocument();
    expect(status).toHaveAttribute("aria-busy", "true");
  });

  it("shows 4 stat placeholders by default (design N13Xud's skStats row)", () => {
    render(<AppLoadingSkeleton />);
    expect(screen.getAllByTestId("stat-placeholder")).toHaveLength(4);
  });

  it("shows N stat placeholders when statCount is provided", () => {
    render(<AppLoadingSkeleton statCount={2} />);
    expect(screen.getAllByTestId("stat-placeholder")).toHaveLength(2);
  });
});
