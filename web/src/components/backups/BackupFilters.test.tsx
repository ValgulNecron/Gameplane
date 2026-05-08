import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
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
  it("renders all server options plus an All-servers row", () => {
    render(<BackupFilters {...baseProps} />);
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

  it("emits onServerChange when server dropdown changes", () => {
    const onServerChange = vi.fn();
    render(<BackupFilters {...baseProps} onServerChange={onServerChange} />);
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[0], { target: { value: "alpha" } });
    expect(onServerChange).toHaveBeenCalledWith("alpha");
  });

  it("emits onPhaseChange when phase dropdown changes", () => {
    const onPhaseChange = vi.fn();
    render(<BackupFilters {...baseProps} onPhaseChange={onPhaseChange} />);
    const selects = screen.getAllByRole("combobox");
    fireEvent.change(selects[1], { target: { value: "Succeeded" } });
    expect(onPhaseChange).toHaveBeenCalledWith("Succeeded");
  });

  it("renders the trailing slot when provided", () => {
    render(<BackupFilters {...baseProps} trailing="3 results" />);
    expect(screen.getByText("3 results")).toBeInTheDocument();
  });
});
