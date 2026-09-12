import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmAdminMappingDialog } from "./ConfirmAdminMappingDialog";

describe("ConfirmAdminMappingDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("does not render content when closed", () => {
    render(
      <ConfirmAdminMappingDialog
        open={false}
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.queryByText(/Confirm admin role mapping/)).not.toBeInTheDocument();
  });

  it("renders title when open", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText("Confirm admin role mapping?")).toBeInTheDocument();
  });

  it("renders warning about full admin access", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText("Full admin access")).toBeInTheDocument();
    expect(
      screen.getByText(/Mapping users to the admin role grants full cluster control/),
    ).toBeInTheDocument();
  });

  it("displays admin groups being mapped", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["developers", "admins"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByText("developers")).toBeInTheDocument();
    expect(screen.getByText("admins")).toBeInTheDocument();
  });

  it("renders confirm button with correct label", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Map to admin role" })).toBeInTheDocument();
  });

  it("renders cancel button", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("calls onConfirm when confirm button is clicked", async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        onConfirm={onConfirm}
      />,
    );

    const confirmBtn = screen.getByRole("button", { name: "Map to admin role" });
    await userEvent.click(confirmBtn);

    expect(onConfirm).toHaveBeenCalled();
  });

  it("calls onOpenChange(false) when cancel button is clicked", async () => {
    const onOpenChange = vi.fn();
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={onOpenChange}
        adminGroups={["group1"]}
        onConfirm={() => {}}
      />,
    );

    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await userEvent.click(cancelBtn);

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables buttons when busy is true", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        busy
        onConfirm={() => {}}
      />,
    );

    const confirmBtn = screen.getByRole("button", { name: "Working…" });
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });

    expect(confirmBtn).toBeDisabled();
    expect(cancelBtn).toBeDisabled();
  });

  it("shows 'Working…' text when busy", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={["group1"]}
        busy
        onConfirm={() => {}}
      />,
    );

    expect(screen.getByRole("button", { name: "Working…" })).toBeInTheDocument();
  });

  it("handles empty adminGroups gracefully", () => {
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={[]}
        onConfirm={() => {}}
      />,
    );

    expect(screen.getByText("Confirm admin role mapping?")).toBeInTheDocument();
    expect(screen.getByText("Full admin access")).toBeInTheDocument();
    // Group list should not be displayed when no groups
    expect(screen.queryByText(/Group\(s\) being mapped to admin/)).not.toBeInTheDocument();
  });

  it("displays all admin groups when multiple are provided", () => {
    const groups = ["group-a", "group-b", "group-c"];
    render(
      <ConfirmAdminMappingDialog
        open
        onOpenChange={() => {}}
        adminGroups={groups}
        onConfirm={() => {}}
      />,
    );

    groups.forEach((group) => {
      expect(screen.getByText(group)).toBeInTheDocument();
    });
  });
});
