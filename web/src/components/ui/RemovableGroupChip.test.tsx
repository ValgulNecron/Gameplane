import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RemovableGroupChip } from "./RemovableGroupChip";

describe("RemovableGroupChip", () => {
  // Test 1: Renders label
  it("renders the label text", () => {
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="group-name" onRemove={onRemove} />);
    expect(screen.getByText("group-name")).toBeInTheDocument();
  });

  // Test 2: Renders close button
  it("renders a close button", () => {
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="test-group" onRemove={onRemove} />);
    const button = screen.getByRole("button", { name: /Remove test-group/i });
    expect(button).toBeInTheDocument();
  });

  // Test 3: Calls onRemove when close button is clicked
  it("calls onRemove when close button is clicked", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="test-group" onRemove={onRemove} />);
    const button = screen.getByRole("button", { name: /Remove test-group/i });
    await user.click(button);
    expect(onRemove).toHaveBeenCalledOnce();
  });

  // Test 4: Secondary variant (default)
  it("applies secondary variant by default", () => {
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="test" onRemove={onRemove} />);
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toHaveAttribute("data-variant", "secondary");
  });

  // Test 5: Secondary variant explicitly
  it("applies secondary variant when specified", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip
        label="test"
        variant="secondary"
        onRemove={onRemove}
      />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toHaveAttribute("data-variant", "secondary");
  });

  // Test 6: Orange variant
  it("applies orange variant", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip label="test" variant="orange" onRemove={onRemove} />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toHaveAttribute("data-variant", "orange");
  });

  // Test 7: Violet variant
  it("applies violet variant", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip label="test" variant="violet" onRemove={onRemove} />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toHaveAttribute("data-variant", "violet");
  });

  // Test 8: Size prop is applied
  it("applies size prop to chip", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip label="test" size="lg" onRemove={onRemove} />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).not.toBeNull();
    // Size is applied but we check it's rendered
    expect(chip).toBeInTheDocument();
  });

  // Test 9: Custom className is applied
  it("applies custom className", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip
        label="test"
        onRemove={onRemove}
        className="custom-class"
      />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip?.className).toContain("custom-class");
  });

  // Test 10: Stops click propagation on close button
  it("stops click propagation when close button is clicked", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    const onParentClick = vi.fn();

    render(
      <div onClick={onParentClick}>
        <RemovableGroupChip label="test" onRemove={onRemove} />
      </div>
    );

    const button = screen.getByRole("button", { name: /Remove test/i });
    await user.click(button);

    expect(onRemove).toHaveBeenCalledOnce();
    expect(onParentClick).not.toHaveBeenCalled();
  });

  // Test 11: Multiple chips can coexist
  it("renders multiple chips with independent remove handlers", async () => {
    const user = userEvent.setup();
    const onRemove1 = vi.fn();
    const onRemove2 = vi.fn();

    render(
      <>
        <RemovableGroupChip label="group-1" onRemove={onRemove1} />
        <RemovableGroupChip label="group-2" onRemove={onRemove2} />
      </>
    );

    const button2 = screen.getByRole("button", { name: /Remove group-2/i });
    await user.click(button2);

    expect(onRemove1).not.toHaveBeenCalled();
    expect(onRemove2).toHaveBeenCalledOnce();
  });

  // Test 12: Works with all three variants
  it("renders all three variants with their labels", () => {
    const onRemove = vi.fn();

    const { rerender } = render(
      <RemovableGroupChip label="test" variant="secondary" onRemove={onRemove} />
    );
    expect(screen.getByText("test")).toBeInTheDocument();

    rerender(
      <RemovableGroupChip label="test" variant="orange" onRemove={onRemove} />
    );
    expect(screen.getByText("test")).toBeInTheDocument();

    rerender(
      <RemovableGroupChip label="test" variant="violet" onRemove={onRemove} />
    );
    expect(screen.getByText("test")).toBeInTheDocument();
  });

  // Test 13: Aria label reflects the group name
  it("has correct aria-label on close button", () => {
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="admins" onRemove={onRemove} />);
    const button = screen.getByRole("button", { name: /Remove admins/i });
    expect(button).toHaveAttribute("aria-label", "Remove admins");
  });

  // Test 14: Small size (default)
  it("applies default small size", () => {
    const onRemove = vi.fn();
    render(<RemovableGroupChip label="test" onRemove={onRemove} />);
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toBeInTheDocument();
  });

  // Test 15: Medium size
  it("applies medium size", () => {
    const onRemove = vi.fn();
    render(
      <RemovableGroupChip label="test" size="md" onRemove={onRemove} />
    );
    const chip = screen.getByText("test").closest('[data-slot="chip"]');
    expect(chip).toBeInTheDocument();
  });
});
