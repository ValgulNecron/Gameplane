import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Textarea } from "./textarea";

describe("Textarea", () => {
  it("accepts user input", async () => {
    render(<Textarea placeholder="msg" />);
    const ta = screen.getByPlaceholderText("msg") as HTMLTextAreaElement;
    await userEvent.type(ta, "hello\nworld");
    expect(ta.value).toBe("hello\nworld");
  });
  it("forwards ref", () => {
    const ref = { current: null as HTMLTextAreaElement | null };
    render(<Textarea ref={ref} placeholder="x" />);
    expect(ref.current).toBeInstanceOf(HTMLTextAreaElement);
  });
});
