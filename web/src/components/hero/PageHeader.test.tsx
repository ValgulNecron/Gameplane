import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageHeader } from "./PageHeader";

describe("PageHeader", () => {
  it("renders the title only", () => {
    render(<PageHeader title="Servers" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Servers");
    expect(screen.queryByText(/description/i)).not.toBeInTheDocument();
  });

  it("renders description when provided", () => {
    render(<PageHeader title="Users" description="Manage access and permissions" />);
    expect(screen.getByText("Manage access and permissions")).toBeInTheDocument();
  });

  it("renders action slot", () => {
    render(<PageHeader title="Dashboard" actions={<button>Add Server</button>} />);
    expect(screen.getByRole("button", { name: "Add Server" })).toBeInTheDocument();
  });

  it("renders breadcrumbs when provided", () => {
    render(
      <PageHeader
        title="Servers"
        breadcrumbs={[
          { label: "Home", href: "/" },
          { label: "Infrastructure" },
          { label: "Servers" },
        ]}
      />
    );
    // Check that all breadcrumb labels are present
    expect(screen.getByText("Home")).toBeInTheDocument();
    expect(screen.getByText("Infrastructure")).toBeInTheDocument();
    expect(screen.getByText("Servers")).toBeInTheDocument();
  });

  it("renders breadcrumbs with links for items with href", () => {
    render(
      <PageHeader
        title="Detail"
        breadcrumbs={[
          { label: "Dashboard", href: "/dashboard" },
          { label: "Current Page" },
        ]}
      />
    );
    const link = screen.getByRole("link");
    expect(link).toHaveTextContent("Dashboard");
    expect(link).toHaveAttribute("href", "/dashboard");
  });

  it("renders title, description, and actions together", () => {
    render(
      <PageHeader
        title="Servers"
        description="Manage your game servers"
        actions={<button>Create</button>}
      />
    );
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Servers");
    expect(screen.getByText("Manage your game servers")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Create" })).toBeInTheDocument();
  });

  it("renders breadcrumbs, title, description, and actions all together", () => {
    render(
      <PageHeader
        breadcrumbs={[
          { label: "Home", href: "/" },
          { label: "Servers" },
        ]}
        title="Server List"
        description="All servers in your cluster"
        actions={<button>Refresh</button>}
      />
    );
    expect(screen.getByText("Home")).toBeInTheDocument();
    expect(screen.getByText("Servers")).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Server List");
    expect(screen.getByText("All servers in your cluster")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Refresh" })).toBeInTheDocument();
  });

  it("does not render breadcrumbs when empty array is provided", () => {
    const { container } = render(
      <PageHeader title="Test" breadcrumbs={[]} />
    );
    // Breadcrumbs container should not be rendered
    const breadcrumbsNav = container.querySelector("nav");
    expect(breadcrumbsNav).not.toBeInTheDocument();
  });

  it("renders non-link breadcrumb items as spans with muted text", () => {
    render(
      <PageHeader
        title="Test"
        breadcrumbs={[{ label: "No Link Item" }]}
      />
    );
    const span = screen.getByText("No Link Item");
    expect(span).toHaveClass("text-muted");
  });
});
