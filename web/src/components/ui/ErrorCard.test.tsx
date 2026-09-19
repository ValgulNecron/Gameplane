import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { ErrorCard } from "./ErrorCard";

describe("ErrorCard", () => {
  it("renders the error message", () => {
    render(<ErrorCard message="Failed to load data" />);
    expect(screen.getByText("Failed to load data")).toBeInTheDocument();
  });

  it("renders title when provided", () => {
    render(
      <ErrorCard
        title="Configuration Error"
        message="Unable to parse config file"
      />
    );
    expect(screen.getByText("Configuration Error")).toBeInTheDocument();
    expect(screen.getByText("Unable to parse config file")).toBeInTheDocument();
  });

  it("renders error icon by default", () => {
    const { container } = render(
      <ErrorCard message="Something went wrong" />
    );
    const icon = container.querySelector("svg");
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveClass("text-danger");
  });

  it("uses custom icon when provided", () => {
    const customIcon = <span data-testid="custom-icon">!</span>;
    render(<ErrorCard message="Error" icon={customIcon} />);
    expect(screen.getByTestId("custom-icon")).toBeInTheDocument();
  });

  it("shows dismiss button when onDismiss is provided", () => {
    render(
      <ErrorCard message="Error message" onDismiss={() => {}} />
    );
    const dismissButton = screen.getByLabelText("Dismiss error");
    expect(dismissButton).toBeInTheDocument();
    expect(dismissButton).toHaveTextContent("×");
  });

  it("hides dismiss button when onDismiss is not provided", () => {
    render(<ErrorCard message="Error without dismiss" />);
    const dismissButton = screen.queryByLabelText("Dismiss error");
    expect(dismissButton).not.toBeInTheDocument();
  });

  it("calls onDismiss when dismiss button is clicked", async () => {
    const onDismiss = vi.fn();
    const user = userEvent.setup();
    render(<ErrorCard message="Error" onDismiss={onDismiss} />);
    const dismissButton = screen.getByLabelText("Dismiss error");
    await user.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("shows retry button when onRetry is provided", () => {
    render(<ErrorCard message="Failed" onRetry={() => {}} />);
    const retryButton = screen.getByLabelText("Retry");
    expect(retryButton).toBeInTheDocument();
    expect(retryButton).toHaveTextContent("Retry");
  });

  it("calls onRetry when retry button is clicked", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(<ErrorCard message="Error" onRetry={onRetry} />);
    const retryButton = screen.getByLabelText("Retry");
    await user.click(retryButton);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("renders both retry and dismiss buttons when both callbacks are provided", () => {
    render(
      <ErrorCard
        message="Error"
        onRetry={() => {}}
        onDismiss={() => {}}
      />
    );
    expect(screen.getByLabelText("Retry")).toBeInTheDocument();
    expect(screen.getByLabelText("Dismiss error")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <ErrorCard message="Error" className="custom-class" />
    );
    const alert = container.firstChild;
    expect(alert).toHaveClass("custom-class");
  });

  it("renders title and message together with icon and action buttons", () => {
    const onDismiss = vi.fn();
    const onRetry = vi.fn();
    const { container } = render(
      <ErrorCard
        title="Load Failed"
        message="Server returned an error"
        onDismiss={onDismiss}
        onRetry={onRetry}
      />
    );
    expect(screen.getByText("Load Failed")).toBeInTheDocument();
    expect(screen.getByText("Server returned an error")).toBeInTheDocument();
    expect(container.querySelector("svg")).toBeInTheDocument();
    expect(screen.getByLabelText("Retry")).toBeInTheDocument();
    expect(screen.getByLabelText("Dismiss error")).toBeInTheDocument();
  });
});
