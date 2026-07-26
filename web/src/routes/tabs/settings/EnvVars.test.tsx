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

  it("typing into secret key field updates key", () => {
    const draft = makeServer({
      spec: {
        templateRef: { name: "x" },
        env: [{ name: "FOO", valueFrom: { secretKeyRef: { name: "my-secret", key: "" } } }],
      },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("key"), {
      target: { value: "password" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env[0].valueFrom.secretKeyRef.key).toBe("password");
  });

  it("updating literal value in row", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO", value: "old" }] },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    fireEvent.change(screen.getByPlaceholderText("value"), {
      target: { value: "new" },
    });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env[0].value).toBe("new");
  });

  it("flags invalid env names with special characters", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO-BAR", value: "" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.getByText(/Must match/i)).toBeInTheDocument();
  });

  it("allows env name starting with underscore", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "_FOO", value: "bar" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.queryByText(/Must match/i)).not.toBeInTheDocument();
  });

  it("allows env name with numbers and underscores", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO_BAR_123", value: "" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.queryByText(/Must match/i)).not.toBeInTheDocument();
  });

  it("does not flag duplicate when only one var", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO", value: "1" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    expect(screen.queryByText(/Duplicate name/i)).not.toBeInTheDocument();
  });

  it("removing one of two env vars", async () => {
    const draft = makeServer({
      spec: {
        templateRef: { name: "x" },
        env: [
          { name: "FOO", value: "1" },
          { name: "BAR", value: "2" },
        ],
      },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    const removeButtons = screen.getAllByTitle(/Remove/i);
    await userEvent.click(removeButtons[0]);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env).toHaveLength(1);
    expect(lastCall.spec.env[0].name).toBe("BAR");
  });

  it("secret-ref icon renders for secret-backed vars", () => {
    const draft = makeServer({
      spec: {
        templateRef: { name: "x" },
        env: [{ name: "FOO", valueFrom: { secretKeyRef: { name: "s", key: "k" } } }],
      },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    // The Lock icon should be rendered
    expect(screen.getByText(/Secret/i)).toBeInTheDocument();
  });

  it("type icon renders for literal vars", () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO", value: "bar" }] },
    });
    render(<EnvVarsSection draft={draft} onChange={() => {}} />);
    // The Type icon should render as "Literal"
    expect(screen.getByText(/Literal/i)).toBeInTheDocument();
  });

  it("clearing env removes spec.env when last removed", async () => {
    const draft = makeServer({
      spec: { templateRef: { name: "x" }, env: [{ name: "FOO", value: "1" }] },
    });
    const onChange = vi.fn();
    render(<EnvVarsSection draft={draft} onChange={onChange} />);
    await userEvent.click(screen.getByTitle(/Remove/i));
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.env).toBeUndefined();
  });
});
