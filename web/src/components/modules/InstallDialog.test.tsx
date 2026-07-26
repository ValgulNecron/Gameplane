import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InstallDialog } from "./InstallDialog";
import { makeCatalog } from "@/test/factories";
import { APIError } from "@/lib/api";

describe("InstallDialog", () => {
  it("renders nothing when entry is null", () => {
    render(
      <InstallDialog
        open={true}
        onOpenChange={() => {}}
        entry={null}
        onConfirm={() => {}}
      />,
    );
    expect(screen.queryByRole("heading", { name: /Install / })).not.toBeInTheDocument();
  });

  it("renders dialog with default source + version", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ sources: [{ name: "upstream", type: "oci" }], versions: ["1.21", "1.20"], latestVersion: "1.21" })}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByRole("heading", { name: /Install / })).toBeInTheDocument();
    // Single source renders as static text; two versions render a select.
    expect(screen.getByText("upstream (oci)")).toBeInTheDocument();
    const selects = screen.getAllByRole("combobox") as HTMLSelectElement[];
    expect(selects).toHaveLength(1);
    expect(selects[0].value).toBe("1.21");
  });

  it("Install button submits source/version/name", async () => {
    const onConfirm = vi.fn();
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "minecraft", sources: [{ name: "upstream", type: "oci" }], versions: ["1.21"], latestVersion: "1.21" })}
        onConfirm={onConfirm}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    await waitFor(() =>
      expect(onConfirm).toHaveBeenCalledWith({
        source: "upstream",
        version: "1.21",
        name: "minecraft",
      }),
    );
  });

  it("shows validation error if any field is empty", async () => {
    const onConfirm = vi.fn();
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "x", sources: [{ name: "s", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={onConfirm}
      />,
    );
    const nameInput = screen.getByPlaceholderText("x");
    fireEvent.change(nameInput, { target: { value: "" } });
    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    expect(screen.getByText(/source, version, and name are all required/i)).toBeInTheDocument();
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("surfaces APIError.body when onConfirm rejects", async () => {
    const onConfirm = vi.fn().mockRejectedValue(new APIError(409, "already installed"));
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "x", sources: [{ name: "s", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={onConfirm}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    await waitFor(() => expect(screen.getByText(/already installed/)).toBeInTheDocument());
  });

  it("Cancel calls onOpenChange(false)", async () => {
    const onOpenChange = vi.fn();
    render(
      <InstallDialog
        open
        onOpenChange={onOpenChange}
        entry={makeCatalog({ sources: [{ name: "a", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={() => {}}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("collapses source/version to static text when only one is available", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ sources: [{ name: "only", type: "upload" }], versions: ["1.0"], latestVersion: "1.0" })}
        onConfirm={() => {}}
      />,
    );
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.getByText("only (upload)")).toBeInTheDocument();
    expect(screen.getByText("1.0")).toBeInTheDocument();
  });

  it("lower-cases the typed name", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "x", sources: [{ name: "s", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={() => {}}
      />,
    );
    const nameInput = screen.getByPlaceholderText("x") as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: "MyServer" } });
    expect(nameInput.value).toBe("myserver");
  });

  it("shows multiple version options in a select when available", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ sources: [{ name: "src", type: "oci" }], versions: ["2.0", "1.5", "1.0"], latestVersion: "2.0" })}
        onConfirm={() => {}}
      />,
    );
    const selects = screen.getAllByRole("combobox") as HTMLSelectElement[];
    // Should have 1 select for versions
    expect(selects.length).toBeGreaterThan(0);
    const versionSelect = selects[selects.length - 1];
    expect(versionSelect.value).toBe("2.0");
  });

  it("shows multiple source options in a select when available", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({
          sources: [
            { name: "upstream", type: "oci" },
            { name: "community", type: "git" },
          ],
          versions: ["1.0"],
          latestVersion: "1.0",
        })}
        onConfirm={() => {}}
      />,
    );
    const selects = screen.getAllByRole("combobox") as HTMLSelectElement[];
    // Should have 1 select for sources
    expect(selects.length).toBeGreaterThan(0);
    expect(screen.getByText(/upstream.*oci/)).toBeInTheDocument();
    expect(screen.getByText(/community.*git/)).toBeInTheDocument();
  });

  it("resets form when entry changes", async () => {
    const { rerender } = render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "first", sources: [{ name: "a", type: "oci" }], versions: ["1.0"], latestVersion: "1.0" })}
        onConfirm={() => {}}
      />,
    );
    let nameInput = screen.getByPlaceholderText("first") as HTMLInputElement;
    fireEvent.change(nameInput, { target: { value: "custom-first" } });
    expect(nameInput.value).toBe("custom-first");

    // Change entry
    rerender(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "second", sources: [{ name: "b", type: "oci" }], versions: ["2.0"], latestVersion: "2.0" })}
        onConfirm={() => {}}
      />,
    );
    nameInput = screen.getByPlaceholderText("second") as HTMLInputElement;
    expect(nameInput.value).toBe("second");
  });

  it("handles entry with only latestVersion (no versions array)", () => {
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({
          name: "single",
          sources: [{ name: "src", type: "oci" }],
          versions: undefined,
          latestVersion: "1.0",
        })}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText("1.0")).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /Version/i })).not.toBeInTheDocument();
  });

  it("clears error when dialog re-opens", async () => {
    const onConfirm = vi.fn().mockRejectedValue(new APIError(500, "server error"));
    const { rerender } = render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "x", sources: [{ name: "s", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={onConfirm}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    await screen.findByText(/server error/);

    // Simulate close and reopen
    rerender(
      <InstallDialog
        open={false}
        onOpenChange={() => {}}
        entry={null}
        onConfirm={onConfirm}
      />,
    );
    rerender(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "y", sources: [{ name: "t", type: "oci" }], versions: ["2"], latestVersion: "2" })}
        onConfirm={onConfirm}
      />,
    );
    expect(screen.queryByText(/server error/)).not.toBeInTheDocument();
  });

  it("changes version when user selects different version", async () => {
    const onConfirm = vi.fn();
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "multi", sources: [{ name: "src", type: "oci" }], versions: ["3.0", "2.0", "1.0"], latestVersion: "3.0" })}
        onConfirm={onConfirm}
      />,
    );
    const selects = screen.getAllByRole("combobox") as HTMLSelectElement[];
    const versionSelect = selects[selects.length - 1];
    fireEvent.change(versionSelect, { target: { value: "1.0" } });

    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    await waitFor(() =>
      expect(onConfirm).toHaveBeenCalledWith({
        source: "src",
        version: "1.0",
        name: "multi",
      }),
    );
  });

  it("disables buttons while mutation is pending", async () => {
    const onConfirm = vi.fn().mockImplementation(() => new Promise(() => {})); // never resolves
    render(
      <InstallDialog
        open
        onOpenChange={() => {}}
        entry={makeCatalog({ name: "x", sources: [{ name: "s", type: "oci" }], versions: ["1"], latestVersion: "1" })}
        onConfirm={onConfirm}
        busy
      />,
    );
    const installBtn = screen.getByRole("button", { name: /Install/i });
    const cancelBtn = screen.getByRole("button", { name: /Cancel/i });
    expect(installBtn).toBeDisabled();
    expect(cancelBtn).toBeDisabled();
  });
});
