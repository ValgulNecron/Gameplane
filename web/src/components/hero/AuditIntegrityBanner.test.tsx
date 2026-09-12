import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { userEvent } from "@testing-library/user-event";
import { AuditIntegrityBanner } from "./AuditIntegrityBanner";

describe("AuditIntegrityBanner", () => {
  it("renders the message", () => {
    render(
      <AuditIntegrityBanner message="Integrity check failed — chain breaks at event #286" />
    );
    expect(
      screen.getByText("Integrity check failed — chain breaks at event #286")
    ).toBeInTheDocument();
  });

  it("renders an activity icon", () => {
    const { container } = render(
      <AuditIntegrityBanner message="Test integrity failure" />
    );
    const icon = container.querySelector("svg");
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveClass("text-danger");
  });

  it("shows dismiss button when onDismiss is provided", () => {
    render(
      <AuditIntegrityBanner
        message="Integrity check failed"
        onDismiss={() => {}}
      />
    );
    const dismissButton = screen.getByLabelText("Dismiss audit integrity alert");
    expect(dismissButton).toBeInTheDocument();
    expect(dismissButton).toHaveTextContent("×");
  });

  it("hides dismiss button when onDismiss is not provided", () => {
    render(
      <AuditIntegrityBanner message="Integrity check failed" />
    );
    const dismissButton = screen.queryByLabelText(
      "Dismiss audit integrity alert"
    );
    expect(dismissButton).not.toBeInTheDocument();
  });

  it("calls onDismiss when dismiss button is clicked", async () => {
    const onDismiss = vi.fn();
    const user = userEvent.setup();
    render(
      <AuditIntegrityBanner
        message="Chain integrity error"
        onDismiss={onDismiss}
      />
    );
    const dismissButton = screen.getByLabelText("Dismiss audit integrity alert");
    await user.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
