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

  it("enables idle auto-sleep via switch", async () => {
    const onChange = vi.fn();
    render(
      <LifecycleSection draft={baseDraft} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable idle auto-sleep/i });
    await userEvent.click(sw);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.enabled).toBe(true);
    expect(lastCall.spec.idle?.afterMinutes).toBe(30);
    expect(lastCall.spec.idle?.wakeWindows).toEqual([]);
  });

  it("disabling idle auto-sleep keeps the configuration and only flips enabled off", async () => {
    // Regression: this used to delete spec.idle entirely on disable, losing
    // the user's afterMinutes/wakeWindows with no undo (re-enabling started
    // from scratch). `enabled: false` is exactly what the operator reads as
    // disabled, so the config must survive the round trip.
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        idle: { enabled: true, afterMinutes: 60, wakeWindows: ["0 9 * * 1-5"] },
      },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable idle auto-sleep/i });
    await userEvent.click(sw);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.enabled).toBe(false);
    expect(lastCall.spec.idle?.afterMinutes).toBe(60);
    expect(lastCall.spec.idle?.wakeWindows).toEqual(["0 9 * * 1-5"]);
  });

  it("re-enabling idle auto-sleep restores the prior configuration instead of resetting it", async () => {
    const onChange = vi.fn();
    const draftDisabled = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        idle: { enabled: false, afterMinutes: 90, wakeWindows: ["0 8 * * *"] },
      },
    };
    render(
      <LifecycleSection draft={draftDisabled} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable idle auto-sleep/i });
    await userEvent.click(sw);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.enabled).toBe(true);
    expect(lastCall.spec.idle?.afterMinutes).toBe(90);
    expect(lastCall.spec.idle?.wakeWindows).toEqual(["0 8 * * *"]);
  });

  it("changes sleep-after minutes", () => {
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: [] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const sleepAfter = screen.getByLabelText("Idle sleep after");
    fireEvent.change(sleepAfter, { target: { value: "60" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.afterMinutes).toBe(60);
  });

  it("clearing sleep-after minutes leaves the field empty instead of snapping back to the default", () => {
    // Regression: value={String(afterMinutes ?? 30)} fought the field being
    // cleared — the moment afterMinutes went undefined the display fell back
    // to "30", so a user could never actually empty the box.
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: [] } },
    };
    const { rerender } = render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const sleepAfter = screen.getByLabelText("Idle sleep after") as HTMLInputElement;
    fireEvent.change(sleepAfter, { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.afterMinutes).toBeUndefined();

    rerender(<LifecycleSection draft={lastCall} onChange={onChange} />);
    expect((screen.getByLabelText("Idle sleep after") as HTMLInputElement).value).toBe("");
  });

  it("flags an out-of-range idle afterMinutes", () => {
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 2, wakeWindows: [] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={() => {}} />,
    );
    expect(screen.getByText(/between 5 and 1440/i)).toBeInTheDocument();
  });

  it("adds a wake window", async () => {
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: [] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const addBtn = screen.getByRole("button", { name: /Add wake window/i });
    await userEvent.click(addBtn);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.wakeWindows).toHaveLength(1);
  });

  it("disables add-window button at 8 entries", () => {
    const windows = Array.from({ length: 8 }, (_, i) => `0 ${9 + i} * * *`);
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: windows } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={() => {}} />,
    );
    const addBtn = screen.getByRole("button", { name: /Add wake window/i });
    expect(addBtn).toBeDisabled();
  });

  it("removes a wake window", async () => {
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: ["0 9 * * *"] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const removeBtn = screen.getByRole("button", { name: /Remove/i });
    await userEvent.click(removeBtn);
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.wakeWindows).toHaveLength(0);
  });

  it("edits a wake window cron", () => {
    const onChange = vi.fn();
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: ["0 9 * * *"] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={onChange} />,
    );
    const cronInput = screen.getByLabelText("Wake window 1") as HTMLInputElement;
    fireEvent.change(cronInput, { target: { value: "0 12 * * *" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.idle?.wakeWindows[0]).toBe("0 12 * * *");
  });

  it("flags a blank wake window inline instead of silently dropping it", () => {
    // A freshly-added row is "" — MinLength=9 at the apiserver, so it must be
    // visibly invalid rather than filtered out (filtering would delete the
    // row a user is mid-typing).
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: [""] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={() => {}} />,
    );
    expect(screen.getByText(/five-field cron expression/i)).toBeInTheDocument();
  });

  it("clears the invalid flag once a wake window becomes well-formed", () => {
    const draftWithIdle = {
      ...baseDraft,
      spec: { ...baseDraft.spec, idle: { enabled: true, afterMinutes: 30, wakeWindows: ["0 9 * * 1-5"] } },
    };
    render(
      <LifecycleSection draft={draftWithIdle} onChange={() => {}} />,
    );
    expect(screen.queryByText(/five-field cron expression/i)).not.toBeInTheDocument();
  });
});
