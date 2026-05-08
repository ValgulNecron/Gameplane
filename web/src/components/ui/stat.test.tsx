import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatCard } from "./stat";

describe("StatCard", () => {
  it("renders label, value, and sub", () => {
    render(<StatCard label="Players" value={42} sub="online" />);
    expect(screen.getByText("Players")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("online")).toBeInTheDocument();
  });
  it("renders without sub", () => {
    render(<StatCard label="X" value="ok" />);
    expect(screen.queryByText("online")).not.toBeInTheDocument();
  });
  it("applies accent", () => {
    const { container } = render(
      <StatCard label="Z" value="v" icon={<span data-testid="i">i</span>} accent="danger" />,
    );
    const icon = screen.getByTestId("i").parentElement!;
    expect(icon.className).toContain("text-danger");
    expect(container.firstChild).toBeTruthy();
  });
  it("defaults to primary accent when none specified", () => {
    render(<StatCard label="Z" value="v" icon={<span data-testid="i">i</span>} />);
    expect(screen.getByTestId("i").parentElement!.className).toContain("text-primary");
  });
});
