import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Field } from "./Field";

describe("settings Field", () => {
  it("renders the label and its children", () => {
    render(
      <Field label="Instance name">
        <input aria-label="instance" />
      </Field>,
    );
    expect(screen.getByText("Instance name")).toBeInTheDocument();
    expect(screen.getByLabelText("instance")).toBeInTheDocument();
  });

  it("renders a hint when provided", () => {
    render(
      <Field label="Region" hint="Where the server runs">
        <span>child</span>
      </Field>,
    );
    expect(screen.getByText("Where the server runs")).toBeInTheDocument();
  });

  it("omits the hint block when no hint is given", () => {
    render(
      <Field label="Region">
        <span>child</span>
      </Field>,
    );
    expect(screen.queryByText("Where the server runs")).toBeNull();
  });
});
