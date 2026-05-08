import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Card, CardHeader, CardTitle, CardContent } from "./card";

describe("Card", () => {
  it("renders the structure", () => {
    render(
      <Card data-testid="root">
        <CardHeader>
          <CardTitle>Title</CardTitle>
        </CardHeader>
        <CardContent>Body</CardContent>
      </Card>,
    );
    expect(screen.getByTestId("root")).toBeInTheDocument();
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Body")).toBeInTheDocument();
  });
  it("merges className", () => {
    render(<Card className="extra" data-testid="x" />);
    expect(screen.getByTestId("x").className).toContain("extra");
  });
});
