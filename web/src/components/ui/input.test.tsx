import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Input } from "./input";

describe("Input", () => {
  it("renders with placeholder and accepts user typing", async () => {
    render(<Input placeholder="email" />);
    const inp = screen.getByPlaceholderText("email") as HTMLInputElement;
    await userEvent.type(inp, "abc");
    expect(inp.value).toBe("abc");
  });
  it("respects disabled", async () => {
    render(<Input placeholder="x" disabled />);
    const inp = screen.getByPlaceholderText("x");
    expect(inp).toBeDisabled();
  });
  it("forwards ref", () => {
    const ref = { current: null as HTMLInputElement | null };
    render(<Input ref={ref} placeholder="z" />);
    expect(ref.current).toBeInstanceOf(HTMLInputElement);
  });
});
