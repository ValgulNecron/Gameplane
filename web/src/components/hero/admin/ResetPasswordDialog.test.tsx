import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResetPasswordDialog } from "./ResetPasswordDialog";

describe("ResetPasswordDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders when open with username in title", () => {
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
      />,
    );
    expect(screen.getByText("Reset password for alice")).toBeInTheDocument();
    expect(screen.getByLabelText("New password")).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <ResetPasswordDialog
        open={false}
        onOpenChange={() => {}}
        username="alice"
      />,
    );
    expect(screen.queryByText(/Reset password for/)).not.toBeInTheDocument();
  });

  it("allows password input", async () => {
    const user = userEvent.setup();
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
      />,
    );
    const passwordInput = screen.getByDisplayValue("") as HTMLInputElement;
    await user.type(passwordInput, "NewSecurePass123");
    expect(passwordInput.value).toBe("NewSecurePass123");
  });

  it("shows validation error when password is empty", async () => {
    const user = userEvent.setup();
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
      />,
    );
    const resetBtn = screen.getByRole("button", { name: /Reset password/i });
    await user.click(resetBtn);
    expect(screen.getByText("Password is required")).toBeInTheDocument();
  });

  it("shows validation error when password is too short", async () => {
    const user = userEvent.setup();
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
      />,
    );
    const passwordInput = screen.getByDisplayValue("") as HTMLInputElement;
    await user.type(passwordInput, "Short123");
    const resetBtn = screen.getByRole("button", { name: /Reset password/i });
    await user.click(resetBtn);
    expect(screen.getByText("Must be at least 12 characters.")).toBeInTheDocument();
  });

  it("calls onReset with password when valid", async () => {
    const user = userEvent.setup();
    const onReset = vi.fn();
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
        onReset={onReset}
      />,
    );
    const passwordInput = screen.getByDisplayValue("") as HTMLInputElement;
    await user.type(passwordInput, "NewSecurePass123");
    const resetBtn = screen.getByRole("button", { name: /Reset password/i });
    await user.click(resetBtn);
    expect(onReset).toHaveBeenCalledWith("NewSecurePass123");
  });

  it("calls onOpenChange(false) when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <ResetPasswordDialog
        open
        onOpenChange={onOpenChange}
        username="alice"
      />,
    );
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables input when isLoading is true", () => {
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
        isLoading
      />,
    );
    const passwordInput = screen.getByRole("textbox") as HTMLInputElement;
    expect(passwordInput).toBeDisabled();
  });

  it("shows loading state on reset button", () => {
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
        username="alice"
        isLoading
      />,
    );
    const resetBtn = screen.getByRole("button", { name: /Resetting/i });
    expect(resetBtn).toBeDisabled();
  });

  it("renders with empty username by default", () => {
    render(
      <ResetPasswordDialog
        open
        onOpenChange={() => {}}
      />,
    );
    expect(screen.getByText("Reset password for")).toBeInTheDocument();
  });

  it("clears password and errors when closed", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = render(
      <ResetPasswordDialog
        open
        onOpenChange={onOpenChange}
        username="alice"
      />,
    );
    const passwordInput = screen.getByDisplayValue("") as HTMLInputElement;
    await user.type(passwordInput, "Short");
    const resetBtn = screen.getByRole("button", { name: /Reset password/i });
    await user.click(resetBtn);
    expect(screen.getByText("Must be at least 12 characters.")).toBeInTheDocument();

    // Simulate closing and reopening
    rerender(
      <ResetPasswordDialog
        open={false}
        onOpenChange={onOpenChange}
        username="alice"
      />,
    );
    rerender(
      <ResetPasswordDialog
        open
        onOpenChange={onOpenChange}
        username="alice"
      />,
    );
    const newPasswordInput = screen.getByDisplayValue("") as HTMLInputElement;
    expect(newPasswordInput.value).toBe("");
    expect(screen.queryByText("Must be at least 12 characters.")).not.toBeInTheDocument();
  });
});
