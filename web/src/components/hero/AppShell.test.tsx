import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppShell } from "./AppShell";

describe("AppShell", () => {
  it("renders sidebar, topBar, and children", () => {
    render(
      <AppShell
        sidebar={<div>Sidebar Content</div>}
        topBar={<div>TopBar Content</div>}
      >
        <div>Main Content</div>
      </AppShell>
    );

    expect(screen.getByText("Sidebar Content")).toBeInTheDocument();
    expect(screen.getByText("TopBar Content")).toBeInTheDocument();
    expect(screen.getByText("Main Content")).toBeInTheDocument();
  });

  it("renders sidebar with navigation landmark", () => {
    render(
      <AppShell
        sidebar={<div>Nav Items</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Content</div>
      </AppShell>
    );

    const aside = screen.getByRole("navigation");
    expect(aside).toBeInTheDocument();
    expect(aside).toHaveTextContent("Nav Items");
  });

  it("renders main content area with main landmark", () => {
    render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Page Content</div>
      </AppShell>
    );

    const main = screen.getByRole("main");
    expect(main).toBeInTheDocument();
    expect(main).toHaveTextContent("Page Content");
  });

  it("applies correct CSS classes for layout", () => {
    const { container } = render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Content</div>
      </AppShell>
    );

    // Outer container
    const outerDiv = container.querySelector(".flex.h-screen.w-full");
    expect(outerDiv).toBeInTheDocument();

    // Sidebar should have hidden on mobile, flex and fixed width on lg+
    const aside = screen.getByRole("navigation");
    expect(aside).toHaveClass("hidden", "lg:flex", "lg:w-[260px]");

    // Sidebar should have flex-col and border
    expect(aside).toHaveClass("flex-col", "border-r");

    // TopBar container should have fixed height - look for the wrapper with h-16
    const topBarWrapper = container.querySelector(".h-16.border-b");
    expect(topBarWrapper).toBeInTheDocument();
    expect(topBarWrapper).toHaveClass("flex-shrink-0");

    // Main content should be scrollable
    const main = screen.getByRole("main");
    expect(main).toHaveClass("overflow-auto");
  });

  it("renders topBar in a fixed-height container", () => {
    const { container } = render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar Content</div>}
      >
        <div>Content</div>
      </AppShell>
    );

    // Query for the topBar wrapper by looking for the div with h-16 and border-b
    const topBarWrapper = container.querySelector(".h-16.border-b");
    expect(topBarWrapper).toBeInTheDocument();
    expect(topBarWrapper).toHaveClass("h-16");
    expect(topBarWrapper).toHaveClass("border-b");
  });

  it("renders children inside main content area", () => {
    render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Child Element 1</div>
        <div>Child Element 2</div>
      </AppShell>
    );

    const main = screen.getByRole("main");
    expect(main).toHaveTextContent("Child Element 1");
    expect(main).toHaveTextContent("Child Element 2");
  });

  it("applies flex-1 to main for proper flex layout", () => {
    render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Content</div>
      </AppShell>
    );

    const main = screen.getByRole("main");
    expect(main).toHaveClass("flex-1");
  });

  it("has correct background and border classes", () => {
    const { container } = render(
      <AppShell
        sidebar={<div>Sidebar</div>}
        topBar={<div>TopBar</div>}
      >
        <div>Content</div>
      </AppShell>
    );

    const outerDiv = container.querySelector(".bg-background");
    expect(outerDiv).toBeInTheDocument();

    const aside = screen.getByRole("navigation");
    expect(aside).toHaveClass("border-divider");

    const topBar = screen.getByText("TopBar").parentElement;
    expect(topBar).toHaveClass("border-divider");
  });

  it("renders ReactNode children correctly", () => {
    render(
      <AppShell
        sidebar={<button>Sidebar Button</button>}
        topBar={<input placeholder="Search" />}
      >
        <span>Content Span</span>
      </AppShell>
    );

    expect(screen.getByRole("button", { name: "Sidebar Button" })).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Search")).toBeInTheDocument();
    expect(screen.getByText("Content Span")).toBeInTheDocument();
  });
});
