import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { ErrorBanner } from "./ErrorBanner";
import { APIError } from "@/lib/api";

describe("ErrorBanner", () => {
  it("shows the body for an APIError", () => {
    render(<ErrorBanner err={new APIError(500, "boom")} />);
    expect(screen.getByText("boom")).toBeInTheDocument();
  });

  it("falls back to message when APIError body is empty", () => {
    render(<ErrorBanner err={new APIError(500, "")} />);
    expect(screen.getByText(/500/)).toBeInTheDocument();
  });

  it("stringifies arbitrary errors", () => {
    render(<ErrorBanner err={new Error("upstream down")} />);
    expect(screen.getByText(/upstream down/)).toBeInTheDocument();
  });

  it("stringifies non-Error values", () => {
    render(<ErrorBanner err="just a string" />);
    expect(screen.getByText("just a string")).toBeInTheDocument();
  });

  it("renders an alert icon", () => {
    const { container } = render(
      <ErrorBanner err={new APIError(400, "bad request")} />
    );
    const icon = container.querySelector("svg");
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveClass("text-danger");
  });

  it("shows dismiss button when onDismiss is provided", () => {
    render(
      <ErrorBanner
        err={new APIError(400, "test error")}
        onDismiss={() => {}}
      />
    );
    const dismissButton = screen.getByLabelText("Dismiss error");
    expect(dismissButton).toBeInTheDocument();
    expect(dismissButton).toHaveTextContent("×");
  });

  it("hides dismiss button when onDismiss is not provided", () => {
    render(<ErrorBanner err="error without dismiss" />);
    const dismissButton = screen.queryByLabelText("Dismiss error");
    expect(dismissButton).not.toBeInTheDocument();
  });

  it("calls onDismiss when dismiss button is clicked", async () => {
    const onDismiss = vi.fn();
    const user = userEvent.setup();
    render(
      <ErrorBanner
        err={new APIError(409, "conflict error")}
        onDismiss={onDismiss}
      />
    );
    const dismissButton = screen.getByLabelText("Dismiss error");
    await user.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
