import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GeneralSection } from "./General";
import { DESCRIPTION_ANNOTATION } from "./types";
import { makeServer } from "@/test/factories";

describe("GeneralSection", () => {
  it("renders the name and template as disabled fields", () => {
    render(<GeneralSection draft={makeServer({ metadata: { name: "alpha" } })} onChange={() => {}} />);
    const name = screen.getByDisplayValue("alpha") as HTMLInputElement;
    expect(name).toBeDisabled();
  });

  it("typing description sets annotation", () => {
    const onChange = vi.fn();
    render(<GeneralSection draft={makeServer()} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText(/Long-standing survival/i), {
      target: { value: "my realm" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.annotations[DESCRIPTION_ANNOTATION]).toBe("my realm");
  });

  it("clearing description removes annotation", () => {
    const draft = makeServer({
      metadata: { name: "alpha", annotations: { [DESCRIPTION_ANNOTATION]: "old" } },
    });
    const onChange = vi.fn();
    render(<GeneralSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText(/Long-standing survival/i), {
      target: { value: "" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.annotations).toBeUndefined();
  });

  it("setting image sets spec.image", () => {
    const onChange = vi.fn();
    render(<GeneralSection draft={makeServer()} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText(/itzg\/minecraft/i), {
      target: { value: "my/image:latest" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.image).toBe("my/image:latest");
  });

  it("clearing image leaves it undefined", () => {
    const onChange = vi.fn();
    render(
      <GeneralSection
        draft={makeServer({ spec: { templateRef: { name: "x" }, image: "old" } })}
        onChange={onChange}
      />,
    );
    fireEvent.change(screen.getByPlaceholderText(/itzg\/minecraft/i), { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.image).toBeUndefined();
  });

  it("Add label button stays disabled when key is empty", async () => {
    render(<GeneralSection draft={makeServer()} onChange={() => {}} />);
    const addBtn = screen.getByRole("button", { name: "Add" });
    expect(addBtn).toBeDisabled();
    const inputs = screen.getAllByPlaceholderText(/key|value/);
    await userEvent.type(inputs[0], "team");
    expect(addBtn).toBeEnabled();
  });

  it("rendering existing labels offers a Remove button", async () => {
    const draft = makeServer({ metadata: { name: "alpha", labels: { team: "ops" } } });
    const onChange = vi.fn();
    render(<GeneralSection draft={draft} onChange={onChange} />);
    expect(screen.getByText("team")).toBeInTheDocument();
    const remove = screen.getByTitle(/Remove label/i);
    await userEvent.click(remove);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.labels).toBeUndefined();
  });
});
