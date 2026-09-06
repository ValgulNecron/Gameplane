import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BackupFilters } from "./BackupFilters";
import { makeServer } from "@/test/factories";

const baseProps = {
  search: "",
  onSearchChange: vi.fn(),
  server: "",
  onServerChange: vi.fn(),
  phase: "",
  onPhaseChange: vi.fn(),
  servers: [makeServer({ metadata: { name: "alpha" } }), makeServer({ metadata: { name: "beta" } })],
  phases: ["Pending", "Succeeded"],
};

describe("BackupFilters", () => {
  it("renders all server options plus an All-servers row", async () => {
    render(<BackupFilters {...baseProps} />);
    await userEvent.click(screen.getAllByRole("combobox")[0]);
    expect(screen.getByRole("option", { name: "All servers" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "alpha" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "beta" })).toBeInTheDocument();
  });

  it("emits onSearchChange while user types", async () => {
    const onSearchChange = vi.fn();
    render(<BackupFilters {...baseProps} onSearchChange={onSearchChange} />);
    await userEvent.type(screen.getByPlaceholderText(/Search by name/i), "x");
    expect(onSearchChange).toHaveBeenCalledWith("x");
  });

  it("emits onServerChange when server dropdown changes", async () => {
    const onServerChange = vi.fn();
    render(<BackupFilters {...baseProps} onServerChange={onServerChange} />);
    const comboboxes = screen.getAllByRole("combobox");
    await userEvent.click(comboboxes[0]);
    const alphaOption = screen.getByRole("option", { name: "alpha" });
    await userEvent.click(alphaOption);
    expect(onServerChange).toHaveBeenCalledWith("alpha");
  });

  it("emits onPhaseChange when phase dropdown changes", async () => {
    const onPhaseChange = vi.fn();
    render(<BackupFilters {...baseProps} onPhaseChange={onPhaseChange} />);
    const comboboxes = screen.getAllByRole("combobox");
    await userEvent.click(comboboxes[1]);
    const succeededOption = screen.getByRole("option", { name: "Succeeded" });
    await userEvent.click(succeededOption);
    expect(onPhaseChange).toHaveBeenCalledWith("Succeeded");
  });

  it("renders the trailing slot when provided", () => {
    render(<BackupFilters {...baseProps} trailing="3 results" />);
    expect(screen.getByText("3 results")).toBeInTheDocument();
  });
});
