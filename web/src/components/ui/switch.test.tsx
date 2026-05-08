import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Switch } from "./switch";

describe("Switch", () => {
  it("renders with the right ARIA state", () => {
    render(<Switch checked={true} onCheckedChange={() => {}} aria-label="x" />);
    const sw = screen.getByRole("switch");
    expect(sw).toHaveAttribute("aria-checked", "true");
  });
  it("toggles on click", async () => {
    const fn = vi.fn();
    render(<Switch checked={false} onCheckedChange={fn} />);
    await userEvent.click(screen.getByRole("switch"));
    expect(fn).toHaveBeenCalledWith(true);
  });
  it("respects disabled", async () => {
    const fn = vi.fn();
    render(<Switch checked={false} onCheckedChange={fn} disabled />);
    const sw = screen.getByRole("switch");
    expect(sw).toBeDisabled();
    await userEvent.click(sw);
    expect(fn).not.toHaveBeenCalled();
  });
  it("applies primary bg when checked", () => {
    render(<Switch checked={true} onCheckedChange={() => {}} />);
    expect(screen.getByRole("switch").className).toContain("bg-primary");
  });
});
