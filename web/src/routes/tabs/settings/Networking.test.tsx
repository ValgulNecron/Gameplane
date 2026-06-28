import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { NetworkingSection } from "./Networking";
import { makeServer } from "@/test/factories";

const baseDraft = makeServer();

describe("NetworkingSection", () => {
  it("changing expose persists the new value", () => {
    const onChange = vi.fn();
    render(<NetworkingSection draft={baseDraft} onChange={onChange} />);
    fireEvent.change(screen.getAllByRole("combobox")[0], { target: { value: "NodePort" } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          networking: expect.objectContaining({ expose: "NodePort" }),
        }),
      }),
    );
  });

  it("clears networking entirely when no fields remain", () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, networking: { hostname: "h" } },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={draft} onChange={onChange} />);
    const hostInput = screen.getAllByRole("textbox")[0];
    fireEvent.change(hostInput, { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    // Empty hostname + no other fields → networking dropped to undefined.
    expect(lastCall.spec.networking).toBeUndefined();
  });

  it("renders all four expose options", () => {
    render(<NetworkingSection draft={baseDraft} onChange={() => {}} />);
    for (const label of ["ClusterIP", "NodePort", "LoadBalancer", "Hostport"]) {
      expect(screen.getByRole("option", { name: new RegExp(label) })).toBeInTheDocument();
    }
  });

  it("shows the IP allow-list only for LoadBalancer", () => {
    const lb = {
      ...baseDraft,
      spec: { ...baseDraft.spec, networking: { expose: "LoadBalancer" as const } },
    };
    const { rerender } = render(<NetworkingSection draft={lb} onChange={() => {}} />);
    expect(screen.getByLabelText("LoadBalancer IP allow-list")).toBeInTheDocument();

    const np = {
      ...baseDraft,
      spec: { ...baseDraft.spec, networking: { expose: "NodePort" as const } },
    };
    rerender(<NetworkingSection draft={np} onChange={() => {}} />);
    expect(screen.queryByLabelText("LoadBalancer IP allow-list")).not.toBeInTheDocument();
  });

  it("parses and persists sourceRanges from the allow-list", () => {
    const lb = {
      ...baseDraft,
      spec: { ...baseDraft.spec, networking: { expose: "LoadBalancer" as const } },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={lb} onChange={onChange} />);
    fireEvent.change(screen.getByLabelText("LoadBalancer IP allow-list"), {
      target: { value: "203.0.113.0/24\n10.0.0.0/8" },
    });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          networking: expect.objectContaining({
            sourceRanges: ["203.0.113.0/24", "10.0.0.0/8"],
          }),
        }),
      }),
    );
  });

  it("preserves existing sourceRanges when editing another field", () => {
    const draft = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        networking: { expose: "LoadBalancer" as const, sourceRanges: ["203.0.113.0/24"] },
      },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("mc.example.com"), {
      target: { value: "mc.example.com" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking.sourceRanges).toEqual(["203.0.113.0/24"]);
  });
});
