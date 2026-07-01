import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PasswordInput } from "./password-input";

describe("PasswordInput", () => {
  it("renders a password field with a Show password toggle", () => {
    render(<PasswordInput placeholder="pw" />);
    const inp = screen.getByPlaceholderText("pw") as HTMLInputElement;
    expect(inp.type).toBe("password");
    expect(screen.getByRole("button", { name: "Show password" })).toBeInTheDocument();
  });

  it("toggles the input type and flips the aria-label", async () => {
    render(<PasswordInput placeholder="pw" />);
    const inp = screen.getByPlaceholderText("pw") as HTMLInputElement;

    await userEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(inp.type).toBe("text");

    await userEvent.click(screen.getByRole("button", { name: "Hide password" }));
    expect(inp.type).toBe("password");
    expect(screen.getByRole("button", { name: "Show password" })).toBeInTheDocument();
  });

  it("keeps the toggle out of the tab order and off the submit path", () => {
    render(<PasswordInput placeholder="pw" />);
    const btn = screen.getByRole("button", { name: "Show password" });
    expect(btn).toHaveAttribute("tabindex", "-1");
    expect(btn).toHaveAttribute("type", "button");
  });

  it("forwards props and ref to the underlying input", async () => {
    const ref = { current: null as HTMLInputElement | null };
    render(<PasswordInput ref={ref} placeholder="pw" autoComplete="current-password" />);
    expect(ref.current).toBeInstanceOf(HTMLInputElement);
    expect(ref.current).toHaveAttribute("autocomplete", "current-password");
    await userEvent.type(screen.getByPlaceholderText("pw"), "abc");
    expect(ref.current!.value).toBe("abc");
  });
});
