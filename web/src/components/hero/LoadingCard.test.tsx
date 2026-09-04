import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LoadingCard } from "./LoadingCard";

describe("LoadingCard", () => {
  it("renders with default message when message prop is not provided", () => {
    render(<LoadingCard />);
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("renders with custom message when message prop is provided", () => {
    const customMessage = "Loading configuration…";
    render(<LoadingCard message={customMessage} />);
    expect(screen.getByText(customMessage)).toBeInTheDocument();
    expect(screen.queryByText("Loading…")).not.toBeInTheDocument();
  });

  it("renders spinner and message in a card with semantic styling", () => {
    const { container } = render(<LoadingCard message="Loading…" />);
    const card = container.querySelector(".flex");
    expect(card).toBeInTheDocument();
    expect(card).toHaveClass("items-center");
    expect(card).toHaveClass("gap-3");

    const message = screen.getByText("Loading…") as HTMLElement;
    expect(message).toHaveClass("text-muted");
    expect(message).toHaveClass("text-sm");
  });

  it("applies custom className prop without removing default classes", () => {
    const { container } = render(
      <LoadingCard message="Wait…" className="custom-class" />,
    );
    const card = container.querySelector(".flex");
    expect(card).toHaveClass("custom-class");
    expect(card).toHaveClass("items-center");
  });
});
