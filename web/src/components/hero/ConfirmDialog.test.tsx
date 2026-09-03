import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "./ConfirmDialog";

describe("ConfirmDialog", () => {
  it("does not render content when closed", () => {
    render(
      <ConfirmDialog
        open={false}
        onOpenChange={() => {}}
        title="Delete"
        description="x"
        onConfirm={() => {}}
      />,
    );
    // When closed, the heading should not be in the document
    expect(screen.queryByText("Delete")).not.toBeInTheDocument();
  });

  it("renders title + description when open", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="Delete"
        description="really?"
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText("Delete")).toBeInTheDocument();
    expect(screen.getByText("really?")).toBeInTheDocument();
  });

  it("calls onConfirm when no phrase required", async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="X"
        description="d"
        onConfirm={onConfirm}
      />,
    );
    // Find the confirm button by its label (default is "Confirm")
    const btn = screen.getByRole("button", { name: "Confirm" });
    await userEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("disables confirm until phrase matches", async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="Drop"
        description="d"
        confirmPhrase="DELETE"
        destructive
        onConfirm={onConfirm}
      />,
    );
    const btn = screen.getByRole("button", { name: "Confirm" });
    expect(btn).toBeDisabled();
    const inp = screen.getByPlaceholderText("Type to confirm") as HTMLInputElement;
    await userEvent.type(inp, "DELETE");
    expect(btn).toBeEnabled();
    await userEvent.click(btn);
    expect(onConfirm).toHaveBeenCalled();
  });

  it("shows busy label when busy", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="X"
        description="d"
        onConfirm={() => {}}
        busy
      />,
    );
    expect(screen.getByRole("button", { name: "Working…" })).toBeDisabled();
  });

  it("Cancel triggers onOpenChange(false)", async () => {
    const fn = vi.fn();
    render(
      <ConfirmDialog
        open
        onOpenChange={fn}
        title="X"
        description="d"
        onConfirm={() => {}}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(fn).toHaveBeenCalledWith(false);
  });

  it("supports custom confirmLabel", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={() => {}}
        title="Delete"
        description="Are you sure?"
        confirmLabel="Delete Forever"
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Delete Forever" })).toBeInTheDocument();
  });
});
