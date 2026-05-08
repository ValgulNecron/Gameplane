import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
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
});
