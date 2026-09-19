import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EditUserDialog } from "./EditUserDialog";

describe("EditUserDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders when open", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    expect(screen.getByText("Edit user")).toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Primary role (cluster-wide)")).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <EditUserDialog
        open={false}
        onOpenChange={() => {}}
      />,
    );
    expect(screen.queryByText("Edit user")).not.toBeInTheDocument();
  });

  it("populates fields with initial values", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice Operator"
        email="alice@example.com"
        role="admin"
      />,
    );
    const displayNameInput = screen.getByDisplayValue("Alice Operator");
    const emailInput = screen.getByDisplayValue("alice@example.com");
    expect(displayNameInput).toBeInTheDocument();
    expect(emailInput).toBeInTheDocument();
  });

  it("allows editing display name", async () => {
    const user = userEvent.setup();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice Operator"
        email="alice@example.com"
      />,
    );
    const displayNameInput = screen.getByDisplayValue("Alice Operator") as HTMLInputElement;
    await user.clear(displayNameInput);
    await user.type(displayNameInput, "Alice Admin");
    expect(displayNameInput.value).toBe("Alice Admin");
  });

  it("allows editing email", async () => {
    const user = userEvent.setup();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
      />,
    );
    const emailInput = screen.getByDisplayValue("alice@example.com") as HTMLInputElement;
    await user.clear(emailInput);
    await user.type(emailInput, "newalice@example.com");
    expect(emailInput.value).toBe("newalice@example.com");
  });

  it("shows validation error when display name is empty", async () => {
    const user = userEvent.setup();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
      />,
    );
    const displayNameInput = screen.getByDisplayValue("Alice") as HTMLInputElement;
    await user.clear(displayNameInput);
    const saveBtn = screen.getByRole("button", { name: /Save changes/i });
    await user.click(saveBtn);
    expect(screen.getByText("Display name is required")).toBeInTheDocument();
  });

  it("shows validation error when email is empty", async () => {
    const user = userEvent.setup();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
      />,
    );
    const emailInput = screen.getByDisplayValue("alice@example.com") as HTMLInputElement;
    await user.clear(emailInput);
    const saveBtn = screen.getByRole("button", { name: /Save changes/i });
    await user.click(saveBtn);
    expect(screen.getByText("Email is required")).toBeInTheDocument();
  });

  it("calls onSave with correct data when valid", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        role="admin"
        onSave={onSave}
      />,
    );
    const displayNameInput = screen.getByDisplayValue("Alice") as HTMLInputElement;
    await user.clear(displayNameInput);
    await user.type(displayNameInput, "Alice Updated");
    const saveBtn = screen.getByRole("button", { name: /Save changes/i });
    await user.click(saveBtn);
    expect(onSave).toHaveBeenCalledWith("Alice Updated", "alice@example.com", "admin");
  });

  it("calls onOpenChange(false) when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <EditUserDialog
        open
        onOpenChange={onOpenChange}
        displayName="Alice"
        email="alice@example.com"
      />,
    );
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables inputs when isLoading is true", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        isLoading
      />,
    );
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    inputs.forEach((input) => {
      expect(input).toBeDisabled();
    });
  });

  it("shows loading state on save button", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        isLoading
      />,
    );
    const saveBtn = screen.getByRole("button", { name: /Saving/i });
    expect(saveBtn).toBeDisabled();
  });

  it("accepts custom roles list", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        role="operator"
        roles={["admin", "operator", "viewer", "custom"]}
      />,
    );
    expect(screen.getByText("Primary role (cluster-wide)")).toBeInTheDocument();
  });

  it("shows the username as a description when provided", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        username="alice"
        displayName="Alice"
        email="alice@example.com"
      />,
    );
    expect(screen.getByText("alice")).toBeInTheDocument();
  });

  it("renders extraContent below the role select", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        extraContent={<div>Namespace grants go here</div>}
      />,
    );
    expect(screen.getByText("Namespace grants go here")).toBeInTheDocument();
  });

  it("shows an external apiError alongside client validation", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        apiError="Permission denied"
      />,
    );
    expect(screen.getByText("Permission denied")).toBeInTheDocument();
  });

  it("disables save when nothing has changed from the initial values", () => {
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Alice"
        email="alice@example.com"
        role="operator"
      />,
    );
    expect(screen.getByRole("button", { name: /Save changes/i })).toBeDisabled();
  });

  it("warns and blocks save on a self-demotion from a user-management role", async () => {
    const user = userEvent.setup();
    render(
      <EditUserDialog
        open
        onOpenChange={() => {}}
        displayName="Root"
        email="root@example.com"
        role="operator"
        roles={["admin", "operator", "viewer"]}
        isMe
        roleGrantsUserManagement={(r) => r !== "viewer"}
      />,
    );
    // HeroUI Select uses a trigger button and popover
    const selectTrigger = screen.getByRole("button", { name: /operator/i });
    await user.click(selectTrigger);
    const viewerOption = screen.getByRole("option", { name: /viewer/i });
    await user.click(viewerOption);
    expect(
      screen.getByText(/remove your own ability to manage users/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Save changes/i })).toBeDisabled();
  });
});
