import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PortOverridesEditor } from "./PortOverridesEditor";
import type { PortOverride } from "@/types";

describe("PortOverridesEditor", () => {
  it("appends an empty row via 'Add override'", async () => {
    const onChange = vi.fn();
    render(<PortOverridesEditor values={[]} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Add override/i }));
    expect(onChange).toHaveBeenCalledWith([{ name: "" }]);
  });

  it("writes numeric servicePort and nodePort values", () => {
    const onChange = vi.fn();
    const values: PortOverride[] = [{ name: "game" }];
    render(<PortOverridesEditor values={values} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("—"), { target: { value: "25565" } });
    expect(onChange).toHaveBeenLastCalledWith([{ name: "game", servicePort: 25565 }]);
    fireEvent.change(screen.getByPlaceholderText("30000-32767"), { target: { value: "30005" } });
    expect(onChange).toHaveBeenLastCalledWith([{ name: "game", nodePort: 30005 }]);
  });

  it("removes the targeted row only", async () => {
    const onChange = vi.fn();
    render(
      <PortOverridesEditor
        values={[{ name: "game" }, { name: "rcon" }]}
        onChange={onChange}
      />,
    );
    const removes = screen.getAllByTitle("Remove");
    await userEvent.click(removes[0]);
    expect(onChange).toHaveBeenCalledWith([{ name: "rcon" }]);
  });
});
