import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LifecycleSection } from "./Lifecycle";
import { GRACE_PERIOD_ANNOTATION } from "./types";
import { makeServer } from "@/test/factories";
import type { GameTemplate } from "@/types";

const baseDraft = makeServer();

const templateWithProbe = {
  metadata: { name: "minecraft" },
  spec: {
    displayName: "MC",
    game: "minecraft",
    version: "1",
    image: "img",
    probes: {
      liveness: { initialDelaySeconds: 5, httpGet: { path: "/health", port: 8080 } },
    },
  },
} as GameTemplate;

describe("LifecycleSection", () => {
  it("Auto-restart switch toggles suspend", async () => {
    const onChange = vi.fn();
    render(
      <LifecycleSection draft={baseDraft} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Auto-restart/i });
    await userEvent.click(sw);
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ spec: expect.objectContaining({ suspend: true }) }),
    );
  });

  it("renders the suspended description when suspend=true", () => {
    render(
      <LifecycleSection
        draft={{ ...baseDraft, spec: { ...baseDraft.spec, suspend: true } }}
        onChange={() => {}}
      />,
    );
    expect(screen.getByText(/Pod is suspended/i)).toBeInTheDocument();
  });

  it("setGrace writes spec.stopGracePeriodSeconds as a number", () => {
    const onChange = vi.fn();
    render(<LifecycleSection draft={baseDraft} onChange={onChange} />);
    // Controlled input — fire a single change with the full string
    // rather than typing char-by-char (the parent never re-renders
    // between keystrokes, so userEvent.type would lose interim chars).
    const grace = screen.getAllByRole("textbox")[0];
    fireEvent.change(grace, { target: { value: "30" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.stopGracePeriodSeconds).toBe(30);
  });

  it("removing the value clears spec.stopGracePeriodSeconds", () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, stopGracePeriodSeconds: 10 },
    };
    const onChange = vi.fn();
    render(<LifecycleSection draft={draft} onChange={onChange} />);
    const grace = screen.getAllByRole("textbox")[0];
    fireEvent.change(grace, { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.stopGracePeriodSeconds).toBeUndefined();
  });

  it("reads the legacy annotation as a fallback and migrates it on edit", () => {
    const draft = {
      ...baseDraft,
      metadata: {
        ...baseDraft.metadata,
        annotations: { [GRACE_PERIOD_ANNOTATION]: "45" },
      },
    };
    const onChange = vi.fn();
    render(<LifecycleSection draft={draft} onChange={onChange} />);
    const grace = screen.getAllByRole("textbox")[0] as HTMLInputElement;
    // The legacy value is shown in the field...
    expect(grace.value).toBe("45");
    // ...and the first edit writes the real spec field and drops the
    // dead annotation.
    fireEvent.change(grace, { target: { value: "60" } });
    const last = onChange.mock.calls.at(-1)![0];
    expect(last.spec.stopGracePeriodSeconds).toBe(60);
    expect(last.metadata.annotations).toBeUndefined();
  });

  it("flags an out-of-range grace period", () => {
    render(
      <LifecycleSection
        draft={{ ...baseDraft, spec: { ...baseDraft.spec, stopGracePeriodSeconds: 900 } }}
        onChange={() => {}}
      />,
    );
    expect(screen.getByText(/between 0 and 600/i)).toBeInTheDocument();
  });

  it("editing a probe field seeds an override from the template", () => {
    const onChange = vi.fn();
    render(
      <LifecycleSection draft={baseDraft} onChange={onChange} template={templateWithProbe} />,
    );
    fireEvent.change(screen.getByLabelText("liveness Initial delay"), {
      target: { value: "20" },
    });
    const last = onChange.mock.calls.at(-1)![0];
    expect(last.spec.probes.liveness.initialDelaySeconds).toBe(20);
    // The action is inherited from the template probe.
    expect(last.spec.probes.liveness.httpGet).toEqual({ path: "/health", port: 8080 });
  });

  it("shows 'not defined' for probes the template omits", () => {
    render(
      <LifecycleSection draft={baseDraft} onChange={() => {}} template={templateWithProbe} />,
    );
    // readiness + startup are absent from the template → two notices.
    expect(screen.getAllByText(/not\s+defined by the template/i)).toHaveLength(2);
  });
});
