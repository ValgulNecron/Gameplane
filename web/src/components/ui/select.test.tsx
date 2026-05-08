import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { Select } from "./select";

const opts = [
  { value: "a", label: "Alpha" },
  { value: "b", label: "Beta" },
];

describe("Select", () => {
  it("renders all options", () => {
    render(<Select options={opts} value="a" onValueChange={() => {}} />);
    expect(screen.getByRole("option", { name: "Alpha" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Beta" })).toBeInTheDocument();
  });
  it("invokes onValueChange and onChange", () => {
    const onValueChange = vi.fn();
    const onChange = vi.fn();
    render(
      <Select options={opts} value="a" onValueChange={onValueChange} onChange={onChange} />,
    );
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "b" } });
    expect(onValueChange).toHaveBeenCalledWith("b");
    expect(onChange).toHaveBeenCalled();
  });
  it("works without onValueChange/onChange", () => {
    render(<Select options={opts} defaultValue="a" />);
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "b" } });
  });
});
