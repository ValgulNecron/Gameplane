import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvVarsSection } from "./EnvVars";
import { makeServer } from "@/test/factories";

describe("EnvVarsSection", () => {
  it("renders the empty placeholder when no env vars", () => {
    render(<EnvVarsSection draft={makeServer()} onChange={() => {}} />);
    expect(screen.getByText(/No environment variables/i)).toBeInTheDocument();
  });

  it("'Add variable' appends a literal env var", async () => {
    const onChange = vi.fn();
    render(<EnvVarsSection draft={makeServer()} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Add variable/i }));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env).toEqual([{ name: "", value: "" }]);
  });

  it("'Add from secret' appends a secretKeyRef env var", async () => {
    const onChange = vi.fn();
    render(<EnvVarsSection draft={makeServer()} onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Add from secret/i }));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env[0].valueFrom?.secretKeyRef).toEqual({ name: "", key: "" });
  });

  it("typing a name updates the row", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "", value: "" }] },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("VAR_NAME"), {
      target: { value: "FOO" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env[0].name).toBe("FOO");
  });

  it("flags an invalid env name", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "lower", value: "" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.getByText(/Must match/i)).toBeInTheDocument();
  });

  it("flags duplicate names", () => {
    const draft = makeServer({
      spec: {
        templateRef: { name: "x" },
        env: [
          { name: "FOO", value: "1" },
          { name: "FOO", value: "2" },
        ],
      },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.getAllByText(/Duplicate name/i).length).toBeGreaterThan(0);
  });

  it("Remove button drops the row", async () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO", value: "1" }] },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    await userEvent.click(screen.getByTitle(/Remove/i));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env).toBeUndefined();
  });

  it("typing into a secret-backed value persists", () => {
    const draft = makeServer({
      spec: {
        templateRef: { name: "x" },
        env: [{ name: "FOO", valueFrom: { secretKeyRef: { name: "", key: "" } } }],
      },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("secret-name"), {
      target: { value: "my-secret" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env[0].valueFrom.secretKeyRef.name).toBe("my-secret");
  });
});
