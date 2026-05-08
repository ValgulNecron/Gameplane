import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "./confirm-dialog";

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
    await userEvent.click(screen.getByRole("button", { name: "Confirm" }));
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
    const inp = screen.getByRole("textbox") as HTMLInputElement;
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
});
