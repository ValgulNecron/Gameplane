import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageHeader } from "./PageHeader";

describe("PageHeader", () => {
  it("renders the title only", () => {
    render(<PageHeader title="Servers" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Servers");
    expect(screen.queryByText("description")).not.toBeInTheDocument();
    expect(screen.queryByText("subtitle")).not.toBeInTheDocument();
  });

  it("renders subtitle when provided", () => {
    render(<PageHeader title="Users" subtitle="Manage access" />);
    expect(screen.getByText("Manage access")).toBeInTheDocument();
  });

  it("renders action slot", () => {
    render(<PageHeader title="X" actions={<button>Add</button>} />);
    expect(screen.getByRole("button", { name: "Add" })).toBeInTheDocument();
  });

  it("renders breadcrumbs when provided", () => {
    render(
      <PageHeader
        title="Server Details"
        breadcrumbs={[
          { label: "Servers", href: "/servers" },
          { label: "my-server" },
        ]}
      />
    );
    expect(screen.getByText("Servers")).toBeInTheDocument();
    expect(screen.getByText("my-server")).toBeInTheDocument();
  });
});
