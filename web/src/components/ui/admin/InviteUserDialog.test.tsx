import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InviteUserDialog } from "./InviteUserDialog";

describe("InviteUserDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders when open", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    expect(screen.getByRole("heading", { name: "Invite user" })).toBeInTheDocument();
    expect(screen.getByLabelText("Username")).toBeInTheDocument();
    expect(screen.getByLabelText("Display name")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Initial password")).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <InviteUserDialog
        open={false}
        onOpenChange={() => {}}
      />,
    );
    expect(screen.queryByText("Invite user")).not.toBeInTheDocument();
  });

  it("allows input in all fields", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const usernameInput = screen.getAllByDisplayValue("")[0] as HTMLInputElement;
    const displayNameInput = screen.getAllByDisplayValue("")[1] as HTMLInputElement;
    const emailInput = screen.getAllByDisplayValue("")[2] as HTMLInputElement;
    const passwordInput = screen.getAllByDisplayValue("")[3] as HTMLInputElement;

    await user.type(usernameInput, "alice");
    await user.type(displayNameInput, "Alice Operator");
    await user.type(emailInput, "alice@example.com");
    await user.type(passwordInput, "SecurePass123");

    expect(usernameInput.value).toBe("alice");
    expect(displayNameInput.value).toBe("Alice Operator");
    expect(emailInput.value).toBe("alice@example.com");
    expect(passwordInput.value).toBe("SecurePass123");
  });

  it("shows validation error when username is empty", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(screen.getByText("Username is required")).toBeInTheDocument();
  });

  it("shows validation error when display name is empty", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(screen.getByText("Display name is required")).toBeInTheDocument();
  });

  it("shows validation error when email is empty", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[1], "Alice");
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(screen.getByText("Email is required")).toBeInTheDocument();
  });

  it("shows validation error when password is empty", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[1], "Alice");
    await user.type(inputs[2], "alice@example.com");
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(screen.getByText("Password is required")).toBeInTheDocument();
  });

  it("shows validation error when password is too short", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[1], "Alice");
    await user.type(inputs[2], "alice@example.com");
    await user.type(inputs[3], "Short123");
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(screen.getByText("Password must be at least 12 characters")).toBeInTheDocument();
  });

  it("calls onInvite with correct data when valid", async () => {
    const user = userEvent.setup();
    const onInvite = vi.fn();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        onInvite={onInvite}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[1], "Alice Operator");
    await user.type(inputs[2], "alice@example.com");
    await user.type(inputs[3], "SecurePass123");
    const inviteBtn = screen.getByRole("button", { name: /Invite user/i });
    await user.click(inviteBtn);
    expect(onInvite).toHaveBeenCalledWith("alice", "Alice Operator", "alice@example.com", "SecurePass123");
  });

  it("calls onOpenChange(false) when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <InviteUserDialog
        open
        onOpenChange={onOpenChange}
      />,
    );
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables inputs when isLoading is true", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        isLoading
      />,
    );
    const inputs = screen.getAllByRole("textbox") as HTMLInputElement[];
    inputs.forEach((input) => {
      expect(input).toBeDisabled();
    });
  });

  it("shows loading state on invite button", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        isLoading
      />,
    );
    const inviteBtn = screen.getByRole("button", { name: /Inviting/i });
    expect(inviteBtn).toBeDisabled();
  });

  it("renders a role select when roles are provided", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        roles={["admin", "operator", "viewer"]}
      />,
    );
    expect(screen.getByText("Role")).toBeInTheDocument();
  });

  it("does not render a role select when roles are omitted", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
      />,
    );
    expect(screen.queryByText("Role")).not.toBeInTheDocument();
  });

  it("passes the selected role to onInvite when roles are provided", async () => {
    const user = userEvent.setup();
    const onInvite = vi.fn();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        roles={["admin", "operator", "viewer"]}
        onInvite={onInvite}
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[1], "Alice Operator");
    await user.type(inputs[2], "alice@example.com");
    await user.type(inputs[3], "SecurePass123");
    await user.click(screen.getByRole("button", { name: /Invite user/i }));
    expect(onInvite).toHaveBeenCalledWith(
      "alice",
      "Alice Operator",
      "alice@example.com",
      "SecurePass123",
      "viewer",
    );
  });

  it("allows submitting with only a username when contactFieldsOptional is set", async () => {
    const user = userEvent.setup();
    const onInvite = vi.fn();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        contactFieldsOptional
        onInvite={onInvite}
      />,
    );
    const usernameInput = screen.getAllByDisplayValue("")[0] as HTMLInputElement;
    await user.type(usernameInput, "oidc-user");
    await user.click(screen.getByRole("button", { name: /Invite user/i }));
    expect(onInvite).toHaveBeenCalledWith("oidc-user", "", "", "");
  });

  it("shows the OIDC-invite description when contactFieldsOptional is set", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        contactFieldsOptional
      />,
    );
    expect(screen.getByText(/Leave password blank/i)).toBeInTheDocument();
  });

  it("preemptively disables submit when disableSubmitUntilValid is set and username is empty", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        disableSubmitUntilValid
      />,
    );
    expect(screen.getByRole("button", { name: /Invite user/i })).toBeDisabled();
  });

  it("preemptively disables submit when disableSubmitUntilValid is set and password is too short", async () => {
    const user = userEvent.setup();
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        disableSubmitUntilValid
      />,
    );
    const inputs = screen.getAllByDisplayValue("") as HTMLInputElement[];
    await user.type(inputs[0], "alice");
    await user.type(inputs[3], "short");
    expect(screen.getByRole("button", { name: /Invite user/i })).toBeDisabled();
  });

  it("shows an external apiError alongside client validation", () => {
    render(
      <InviteUserDialog
        open
        onOpenChange={() => {}}
        apiError="Username already exists"
      />,
    );
    expect(screen.getByText("Username already exists")).toBeInTheDocument();
  });
});
