import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProvenanceBadge } from "./ProvenanceBadge";

describe("ProvenanceBadge", () => {
  // Test 1: Render overridden variant
  it("renders overridden variant with pencil icon and label", () => {
    render(<ProvenanceBadge type="overridden" />);
    expect(screen.getByText("Overridden in dashboard")).toBeInTheDocument();
  });

  // Test 2: Render fromHelm variant
  it("renders fromHelm variant with package icon and label", () => {
    render(<ProvenanceBadge type="fromHelm" />);
    expect(screen.getByText("From Helm values")).toBeInTheDocument();
  });

  // Test 3: Render notConfigured variant
  it("renders notConfigured variant with minus icon and label", () => {
    render(<ProvenanceBadge type="notConfigured" />);
    expect(screen.getByText("Not configured")).toBeInTheDocument();
  });

  // Test 4: data-type attribute for overridden
  it("sets data-type attribute to overridden", () => {
    render(<ProvenanceBadge type="overridden" />);
    const chip = screen.getByText("Overridden in dashboard").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-type", "overridden");
  });

  // Test 5: data-type attribute for fromHelm
  it("sets data-type attribute to fromHelm", () => {
    render(<ProvenanceBadge type="fromHelm" />);
    const chip = screen.getByText("From Helm values").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-type", "fromHelm");
  });

  // Test 6: data-type attribute for notConfigured
  it("sets data-type attribute to notConfigured", () => {
    render(<ProvenanceBadge type="notConfigured" />);
    const chip = screen.getByText("Not configured").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip).toHaveAttribute("data-type", "notConfigured");
  });

  // Test 7: Custom size prop - lg
  it("applies size prop lg", () => {
    render(<ProvenanceBadge type="overridden" size="lg" />);
    const chip = screen.getByText("Overridden in dashboard").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("chip--lg");
  });

  // Test 8: Custom size prop - md
  it("applies size prop md", () => {
    render(<ProvenanceBadge type="fromHelm" size="md" />);
    const chip = screen.getByText("From Helm values").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("chip--md");
  });

  // Test 9: Default size is sm
  it("defaults to size sm", () => {
    render(<ProvenanceBadge type="notConfigured" />);
    const chip = screen.getByText("Not configured").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("chip--sm");
  });

  // Test 10: Custom className is applied
  it("applies custom className", () => {
    render(<ProvenanceBadge type="overridden" className="custom-test-class" />);
    const chip = screen.getByText("Overridden in dashboard").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    expect(chip?.className).toContain("custom-test-class");
  });

  // Test 11: Icon is rendered for overridden
  it("renders icon for overridden variant", () => {
    const { container } = render(<ProvenanceBadge type="overridden" />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
  });

  // Test 12: Icon is rendered for fromHelm
  it("renders icon for fromHelm variant", () => {
    const { container } = render(<ProvenanceBadge type="fromHelm" />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
  });

  // Test 13: Icon is rendered for notConfigured
  it("renders icon for notConfigured variant", () => {
    const { container } = render(<ProvenanceBadge type="notConfigured" />);
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
  });

  // Test 14: All variants render as chip elements
  it("all variants render as chip", () => {
    const { rerender } = render(<ProvenanceBadge type="overridden" />);
    let chip = screen.getByText("Overridden in dashboard").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();

    rerender(<ProvenanceBadge type="fromHelm" />);
    chip = screen.getByText("From Helm values").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();

    rerender(<ProvenanceBadge type="notConfigured" />);
    chip = screen.getByText("Not configured").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
  });

  // Test 15: Chip uses secondary variant
  it("uses secondary variant", () => {
    const { container } = render(<ProvenanceBadge type="overridden" />);
    const chip = container.querySelector('[data-slot="chip"]');
    expect(chip?.className).toContain("chip--secondary");
  });
});
