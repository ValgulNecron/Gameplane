import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { FieldLabel } from "./FieldLabel";

describe("FieldLabel", () => {
  it("renders the label and child content", () => {
    render(
      <FieldLabel label="Name">
        <input data-testid="inp" />
      </FieldLabel>,
    );
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByTestId("inp")).toBeInTheDocument();
  });
});
