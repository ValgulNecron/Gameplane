import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { render } from "@testing-library/react";
import { AppearanceToggle } from "./AppearanceToggle";

describe("AppearanceToggle", () => {
  it("renders three toggle buttons (light, dark, system)", () => {
    const onChange = vi.fn();
    render(<AppearanceToggle value="light" onChange={onChange} />);

    expect(screen.getByLabelText(/Light/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Dark/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/System/i)).toBeInTheDocument();
  });

  it("highlights the current value", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <AppearanceToggle value="light" onChange={onChange} />
    );

    expect(screen.getByLabelText(/Light/i)).toHaveClass("bg-primary/20");
    expect(screen.getByLabelText(/Dark/i)).not.toHaveClass("bg-primary/20");

    rerender(<AppearanceToggle value="dark" onChange={onChange} />);
    expect(screen.getByLabelText(/Dark/i)).toHaveClass("bg-primary/20");
    expect(screen.getByLabelText(/Light/i)).not.toHaveClass("bg-primary/20");
  });

  it("sets aria-pressed correctly for each button", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <AppearanceToggle value="light" onChange={onChange} />
    );

    expect(screen.getByLabelText(/Light/i)).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByLabelText(/Dark/i)).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByLabelText(/System/i)).toHaveAttribute("aria-pressed", "false");

    rerender(<AppearanceToggle value="dark" onChange={onChange} />);
    expect(screen.getByLabelText(/Light/i)).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByLabelText(/Dark/i)).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByLabelText(/System/i)).toHaveAttribute("aria-pressed", "false");
  });

  it("calls onChange when a button is clicked", async () => {
    const onChange = vi.fn();
    render(<AppearanceToggle value="light" onChange={onChange} />);

    const darkBtn = screen.getByLabelText(/Dark/i);
    await userEvent.click(darkBtn);

    expect(onChange).toHaveBeenCalledWith("dark");
  });

  it("cycles through light → dark → system", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <AppearanceToggle value="light" onChange={onChange} />
    );

    const darkBtn = screen.getByLabelText(/Dark/i);
    await userEvent.click(darkBtn);
    expect(onChange).toHaveBeenLastCalledWith("dark");

    rerender(<AppearanceToggle value="dark" onChange={onChange} />);
    const systemBtn = screen.getByLabelText(/System/i);
    await userEvent.click(systemBtn);
    expect(onChange).toHaveBeenLastCalledWith("system");

    rerender(<AppearanceToggle value="system" onChange={onChange} />);
    const lightBtn = screen.getByLabelText(/Light/i);
    await userEvent.click(lightBtn);
    expect(onChange).toHaveBeenLastCalledWith("light");
  });
});
