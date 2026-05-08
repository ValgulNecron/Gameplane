import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NetworkingSection } from "./Networking";
import { makeServer } from "@/test/factories";

const baseDraft = makeServer();

describe("NetworkingSection KVEditor (service annotations)", () => {
  it("Add button is disabled until a key is entered", async () => {
    const onChange = vi.fn();
    render(<NetworkingSection draft={baseDraft} onChange={onChange} />);
    const addBtn = screen.getByRole("button", { name: "Add" });
    expect(addBtn).toBeDisabled();
    const keyInput = screen.getByPlaceholderText(/service\.beta/i);
    await userEvent.type(keyInput, "external-dns");
    expect(addBtn).toBeEnabled();
  });

  it("Adding a KV entry persists service annotations", async () => {
    const onChange = vi.fn();
    render(<NetworkingSection draft={baseDraft} onChange={onChange} />);
    const keyInput = screen.getByPlaceholderText(/service\.beta/i);
    const valueInput = screen.getByPlaceholderText("value");
    await userEvent.type(keyInput, "k");
    await userEvent.type(valueInput, "v");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking.serviceAnnotations).toEqual({ k: "v" });
  });

  it("Removing a service annotation drops it", async () => {
    const draft = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        networking: { serviceAnnotations: { team: "ops" } },
      },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={draft} onChange={onChange} />);
    expect(screen.getByText("team")).toBeInTheDocument();
    // Two Remove buttons exist (KV remove, plus per-port remove if any).
    const removes = screen.getAllByTitle(/Remove/i);
    await userEvent.click(removes[0]);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking).toBeUndefined();
  });
});

describe("NetworkingSection PortOverridesEditor", () => {
  it("'Add override' appends an empty PortOverride row", async () => {
    const onChange = vi.fn();
    render(<NetworkingSection draft={baseDraft} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Add override/i }));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking.portOverrides).toEqual([{ name: "" }]);
  });

  it("Editing servicePort writes a numeric override", () => {
    const draft = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        networking: { portOverrides: [{ name: "game" }] },
      },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={draft} onChange={onChange} />);
    const inputs = screen.getAllByRole("textbox");
    // textboxes: hostname (empty default) + KV key + KV value + port name +
    // service port + nodeport. For simplicity, target by placeholder.
    const sp = screen.getAllByPlaceholderText("—")[0];
    fireEvent.change(sp, { target: { value: "30000" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking.portOverrides[0].servicePort).toBe(30000);
    expect(inputs.length).toBeGreaterThan(0);
  });

  it("Removing a PortOverride row drops it", async () => {
    const draft = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        networking: { portOverrides: [{ name: "game" }] },
      },
    };
    const onChange = vi.fn();
    render(<NetworkingSection draft={draft} onChange={onChange} />);
    const removes = screen.getAllByTitle(/Remove/i);
    // The PortOverride remove is the only Remove button when no KV
    // entries exist, so removes[0] targets the row.
    await userEvent.click(removes[0]);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.networking).toBeUndefined();
  });
});
