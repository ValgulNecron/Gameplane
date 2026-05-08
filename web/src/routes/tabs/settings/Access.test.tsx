import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AccessSection } from "./Access";
import { makeServer } from "@/test/factories";

describe("AccessSection", () => {
  it("falls back to 'default' when no serviceAccountName is set", () => {
    render(<AccessSection draft={makeServer()} onChange={() => {}} />);
    const inp = screen.getByDisplayValue("default") as HTMLInputElement;
    expect(inp).toBeDisabled();
  });

  it("shows the configured serviceAccountName", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, serviceAccountName: "kestrel-server" },
    });
    render(<AccessSection draft={draft} onChange={() => {}} />);
    expect(screen.getByDisplayValue("kestrel-server")).toBeInTheDocument();
  });

  it("renders the access placeholder text", () => {
    render(<AccessSection draft={makeServer()} onChange={() => {}} />);
    expect(
      screen.getByText(/Per-server role bindings will appear here/i),
    ).toBeInTheDocument();
  });
});
