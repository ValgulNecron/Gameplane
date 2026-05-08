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
});
