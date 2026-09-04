import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PhaseChip, Chip } from "./PhaseChip";

describe("PhaseChip", () => {
  // Test 1: Basic phase rendering
  it("renders the phase label", () => {
    render(<PhaseChip phase="Running" />);
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  // Test 2: Color mapping for success state
  it("maps Running phase to success color", () => {
    render(<PhaseChip phase="Running" />);
    const chip = screen.getByText("Running").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "success");
  });

  // Test 3: Color mapping for danger state
  it("maps Failed phase to danger color", () => {
    render(<PhaseChip phase="Failed" />);
    const chip = screen.getByText("Failed").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "danger");
  });

  // Test 4: Color mapping for warning state
  it("maps Starting phase to warning color", () => {
    render(<PhaseChip phase="Starting" />);
    const chip = screen.getByText("Starting").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "warning");
  });

  // Test 5: Color mapping for default state
  it("maps Stopped phase to default color", () => {
    render(<PhaseChip phase="Stopped" />);
    const chip = screen.getByText("Stopped").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "default");
  });

  // Test 6: Fallback to Pending when phase is missing
  it("falls back to Pending when phase is missing", () => {
    render(<PhaseChip />);
    expect(screen.getByText("Pending")).toBeInTheDocument();
  });

  // Test 7: Fallback to default color for unknown phases
  it("falls back to default color for unknown phases", () => {
    render(<PhaseChip phase="UnknownPhase" />);
    const chip = screen.getByText("UnknownPhase").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "default");
  });

  // Test 8: Asleep override changes label
  it("overrides label to Asleep when asleep is true", () => {
    render(<PhaseChip phase="Suspended" asleep={true} />);
    expect(screen.getByText("Asleep")).toBeInTheDocument();
    expect(screen.queryByText("Suspended")).not.toBeInTheDocument();
  });

  // Test 9: Asleep maps to accent color
  it("maps asleep to accent color", () => {
    render(<PhaseChip phase="Suspended" asleep={true} />);
    const chip = screen.getByText("Asleep").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "accent");
  });

  // Test 10: Shows phase when asleep is false
  it("shows phase label when asleep is false", () => {
    render(<PhaseChip phase="Suspended" asleep={false} />);
    expect(screen.getByText("Suspended")).toBeInTheDocument();
    expect(screen.queryByText("Asleep")).not.toBeInTheDocument();
  });

  // Test 11: Size prop is passed to Chip
  it("applies size prop to Chip", () => {
    render(<PhaseChip phase="Running" size="lg" />);
    const chip = screen.getByText("Running").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-size", "lg");
  });

  // Test 12: Custom className is applied
  it("applies custom className", () => {
    render(<PhaseChip phase="Running" className="custom-test-class" />);
    const chip = screen.getByText("Running").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("custom-test-class");
  });

  // Test 13: Succeeded (backup phase) maps to success
  it("maps Succeeded (backup phase) to success", () => {
    render(<PhaseChip phase="Succeeded" />);
    const chip = screen.getByText("Succeeded").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "success");
  });

  // Test 14: Suspending (restore phase) maps to warning
  it("maps Suspending (restore phase) to warning", () => {
    render(<PhaseChip phase="Suspending" />);
    const chip = screen.getByText("Suspending").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "warning");
  });

  // Test 15: Resuming (restore phase) maps to warning
  it("maps Resuming (restore phase) to warning", () => {
    render(<PhaseChip phase="Resuming" />);
    const chip = screen.getByText("Resuming").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "warning");
  });
});

describe("Chip (generic wrapper)", () => {
  // Test 1: Basic Chip rendering
  it("renders content", () => {
    render(<Chip>Label</Chip>);
    expect(screen.getByText("Label")).toBeInTheDocument();
  });

  // Test 2: Color prop is applied
  it("applies color prop", () => {
    render(<Chip color="success">Success</Chip>);
    const chip = screen.getByText("Success").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "success");
  });

  // Test 3: Size prop is applied
  it("applies size prop", () => {
    render(<Chip size="md">Medium</Chip>);
    const chip = screen.getByText("Medium").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-size", "md");
  });

  // Test 4: Custom className is applied
  it("applies custom className", () => {
    render(<Chip className="test-class">Test</Chip>);
    const chip = screen.getByText("Test").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("test-class");
  });

  // Test 5: Default color and size
  it("applies default color and size when not specified", () => {
    render(<Chip>Default</Chip>);
    const chip = screen.getByText("Default").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-color", "default");
    expect(chip).toHaveAttribute("data-size", "sm");
  });
});
