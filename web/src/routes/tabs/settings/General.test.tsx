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

  it("adding label with trimmed key and value", async () => {
    const onChange = vi.fn();
    render(<GeneralSection draft={makeServer()} onChange={onChange} />);
    const inputs = screen.getAllByPlaceholderText(/key|value/);
    const keyInput = inputs[0];
    const valueInput = inputs[1];

    await userEvent.type(keyInput, "  team  ");
    await userEvent.type(valueInput, "  ops  ");
    const addBtn = screen.getByRole("button", { name: "Add" });
    await userEvent.click(addBtn);

    const lastCall = onChange.mock.calls.at(-1)![0];
    // Keys should be trimmed
    expect(lastCall.metadata.labels.team).toBe("ops");
  });

  it("add label button disabled when key is only whitespace", async () => {
    render(<GeneralSection draft={makeServer()} onChange={() => {}} />);
    const inputs = screen.getAllByPlaceholderText(/key|value/);
    const addBtn = screen.getByRole("button", { name: "Add" });

    await userEvent.type(inputs[0], "   ");
    expect(addBtn).toBeDisabled();
  });

  it("clears label input after adding", async () => {
    const onChange = vi.fn();
    render(<GeneralSection draft={makeServer()} onChange={onChange} />);
    const inputs = screen.getAllByPlaceholderText(/key|value/);
    const keyInput = inputs[0] as HTMLInputElement;
    const valueInput = inputs[1] as HTMLInputElement;

    await userEvent.type(keyInput, "team");
    await userEvent.type(valueInput, "ops");
    const addBtn = screen.getByRole("button", { name: "Add" });
    await userEvent.click(addBtn);

    // After adding, inputs should be cleared
    expect(keyInput.value).toBe("");
    expect(valueInput.value).toBe("");
  });

  it("allows multiple labels", async () => {
    const onChange = vi.fn();
    render(<GeneralSection draft={makeServer()} onChange={onChange} />);
    const inputs = screen.getAllByPlaceholderText(/key|value/);

    await userEvent.type(inputs[0], "team");
    await userEvent.type(inputs[1], "ops");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));

    // Add another label
    const newInputs = screen.getAllByPlaceholderText(/key|value/);
    await userEvent.type(newInputs[0], "env");
    await userEvent.type(newInputs[1], "prod");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));

    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.labels.team).toBe("ops");
    expect(lastCall.metadata.labels.env).toBe("prod");
  });

  it("clears all annotations when description is last annotation", () => {
    const draft = makeServer({
      metadata: {
        name: "alpha",
        annotations: { "gameplane.local/description": "test" },
      },
    });
    const onChange = vi.fn();
    render(<GeneralSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText(/Long-standing survival/i), {
      target: { value: "" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.annotations).toBeUndefined();
  });

  it("preserves other annotations when clearing description", () => {
    const draft = makeServer({
      metadata: {
        name: "alpha",
        annotations: {
          "gameplane.local/description": "old",
          "custom.annotation": "keep",
        },
      },
    });
    const onChange = vi.fn();
    render(<GeneralSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText(/Long-standing survival/i), {
      target: { value: "" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.metadata.annotations["custom.annotation"]).toBe("keep");
    expect(lastCall.metadata.annotations["gameplane.local/description"]).toBeUndefined();
  });

  it("removing last label clears labels field", async () => {
    const draft = makeServer({
      metadata: { name: "alpha", labels: { team: "ops", env: "prod" } },
    });
    const onChange = vi.fn();
    render(<GeneralSection draft={draft} onChange={onChange} />);

    const removeButtons = screen.getAllByTitle(/Remove label/i);
    await userEvent.click(removeButtons[0]);
    const lastCall = onChange.mock.calls.at(-1)![0];
    // After removing one, we should have one left
    expect(Object.keys(lastCall.metadata.labels)).toHaveLength(1);

    render(
      <GeneralSection draft={lastCall} onChange={onChange} />,
    );

    // Remove the last one
    const lastRemoveBtn = screen.getByTitle(/Remove label/i);
    await userEvent.click(lastRemoveBtn);

    const finalCall = onChange.mock.calls.at(-1)![0];
    expect(finalCall.metadata.labels).toBeUndefined();
  });

  it("image field shows disabled template field", () => {
    const draft = makeServer({ spec: { templateRef: { name: "minecraft-java" } } });
    render(<GeneralSection draft={draft} onChange={() => {}} />);
    const templateInput = screen.getByDisplayValue("minecraft-java") as HTMLInputElement;
    expect(templateInput).toBeDisabled();
  });

  it("setting image to empty string removes it from spec", () => {
    const onChange = vi.fn();
    render(
      <GeneralSection
        draft={makeServer({ spec: { templateRef: { name: "x" }, image: "my/image" } })}
        onChange={onChange}
      />,
    );
    fireEvent.change(screen.getByPlaceholderText(/itzg\/minecraft/i), {
      target: { value: "" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.image).toBeUndefined();
  });
});
