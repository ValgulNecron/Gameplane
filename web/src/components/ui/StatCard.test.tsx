import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatCard } from "./StatCard";

describe("StatCard", () => {
  it("renders label, value, and sub", () => {
    render(<StatCard label="Players" value={42} sub="online" />);
    expect(screen.getByText("Players")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("online")).toBeInTheDocument();
  });

  it("renders without sub", () => {
    render(<StatCard label="X" value="ok" />);
    expect(screen.getByText("X")).toBeInTheDocument();
    expect(screen.getByText("ok")).toBeInTheDocument();
    expect(screen.queryByText("online")).not.toBeInTheDocument();
  });

  it("applies accent to icon", () => {
    render(
      <StatCard
        label="Z"
        value="v"
        icon={<span data-testid="test-icon">📊</span>}
        accent="danger"
      />,
    );
    const icon = screen.getByTestId("test-icon").parentElement!;
    expect(icon.className).toContain("text-danger");
  });

  it("defaults to primary accent when none specified", () => {
    render(
      <StatCard
        label="Z"
        value="v"
        icon={<span data-testid="test-icon">📊</span>}
      />,
    );
    const icon = screen.getByTestId("test-icon").parentElement!;
    expect(icon.className).toContain("text-accent");
  });

  it("renders with success accent", () => {
    render(
      <StatCard
        label="Status"
        value={100}
        sub="healthy"
        icon={<span data-testid="test-icon">✓</span>}
        accent="success"
      />,
    );
    const icon = screen.getByTestId("test-icon").parentElement!;
    expect(icon.className).toContain("text-success");
    expect(screen.getByText("Status")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByText("healthy")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <StatCard
        label="Test"
        value="123"
        className="custom-class"
      />,
    );
    const card = container.querySelector("[class*='bg-surface']");
    expect(card?.className).toContain("custom-class");
  });
});
