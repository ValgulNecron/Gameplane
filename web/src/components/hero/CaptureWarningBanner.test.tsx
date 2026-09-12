import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CaptureWarningBanner } from "./CaptureWarningBanner";

describe("CaptureWarningBanner", () => {
  it("renders the warning title", () => {
    render(<CaptureWarningBanner />);
    expect(screen.getByText(/Caution: network packet captures contain real player data/i)).toBeInTheDocument();
  });

  it("renders all warning bullet points", () => {
    render(<CaptureWarningBanner />);
    expect(screen.getByText(/Player IP addresses and port numbers/i)).toBeInTheDocument();
    expect(screen.getByText(/Network timing and game protocol messages/i)).toBeInTheDocument();
    expect(screen.getByText(/For some games, in-band credentials/i)).toBeInTheDocument();
  });

  it("displays retention duration", () => {
    render(<CaptureWarningBanner retentionHours={24} />);
    expect(screen.getByText(/currently 24 hours/i)).toBeInTheDocument();
  });

  it("uses singular 'hour' when retentionHours is 1", () => {
    render(<CaptureWarningBanner retentionHours={1} />);
    expect(screen.getByText(/currently 1 hour\)/i)).toBeInTheDocument();
  });

  it("uses plural 'hours' when retentionHours is not 1", () => {
    render(<CaptureWarningBanner retentionHours={7} />);
    expect(screen.getByText(/currently 7 hours/i)).toBeInTheDocument();
  });

  it("renders Dismiss button when onDismiss callback is provided", () => {
    render(<CaptureWarningBanner onDismiss={() => {}} />);
    expect(screen.getByRole("button", { name: /Dismiss/i })).toBeInTheDocument();
  });

  it("does not render Dismiss button when onDismiss is not provided", () => {
    render(<CaptureWarningBanner />);
    expect(screen.queryByRole("button", { name: /Dismiss/i })).not.toBeInTheDocument();
  });

  it("calls onDismiss when Dismiss button is clicked", async () => {
    const onDismiss = vi.fn();
    render(<CaptureWarningBanner onDismiss={onDismiss} />);
    const dismissBtn = screen.getByRole("button", { name: /Dismiss/i });
    await userEvent.click(dismissBtn);
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
