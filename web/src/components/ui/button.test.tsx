import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Button } from "./button";

describe("Button", () => {
  it("renders its children", () => {
    render(<Button>Save</Button>);
    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });

  it("invokes onClick", async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Go</Button>);
    await userEvent.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("disables pointer events when disabled", async () => {
    const onClick = vi.fn();
    render(<Button disabled onClick={onClick}>Off</Button>);
    const btn = screen.getByRole("button");
    expect(btn).toBeDisabled();
    await userEvent.click(btn);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("applies the danger variant class", () => {
    render(<Button variant="danger">Delete</Button>);
    const btn = screen.getByRole("button");
    expect(btn.className).toContain("bg-danger");
  });

  it("applies size", () => {
    render(<Button size="lg">Big</Button>);
    expect(screen.getByRole("button").className).toContain("h-11");
  });

  it("asChild renders the inner element", () => {
    render(
      <Button asChild>
        <a href="/x">link</a>
      </Button>,
    );
    expect(screen.getByRole("link", { name: "link" })).toHaveAttribute("href", "/x");
  });

  it("merges custom className", () => {
    render(<Button className="custom-x">x</Button>);
    expect(screen.getByRole("button").className).toContain("custom-x");
  });
});
