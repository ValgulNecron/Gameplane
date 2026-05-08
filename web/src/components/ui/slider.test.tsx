import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import { Slider } from "./slider";

describe("Slider", () => {
  it("emits numeric value on change", () => {
    const fn = vi.fn();
    render(<Slider value={5} min={0} max={10} onValueChange={fn} />);
    const inp = screen.getByRole("slider") as HTMLInputElement;
    fireEvent.change(inp, { target: { value: "7" } });
    expect(fn).toHaveBeenCalledWith(7);
  });
  it("respects step and range", () => {
    render(<Slider value={3} min={0} max={10} step={2} onValueChange={() => {}} />);
    const inp = screen.getByRole("slider") as HTMLInputElement;
    expect(inp.min).toBe("0");
    expect(inp.max).toBe("10");
    expect(inp.step).toBe("2");
  });
  it("can be disabled", () => {
    render(<Slider value={1} min={0} max={2} onValueChange={() => {}} disabled />);
    expect(screen.getByRole("slider")).toBeDisabled();
  });
});
