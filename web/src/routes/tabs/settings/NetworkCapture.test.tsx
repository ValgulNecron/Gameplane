import { describe, it, expect, vi } from "vitest";
import { screen, fireEvent, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { NetworkCaptureSection } from "./NetworkCapture";
import { SettingsTab } from "../Settings";
import { makeServer, makeUser } from "@/test/factories";

const baseDraft = makeServer();

describe("NetworkCaptureSection", () => {
  it("reflects spec.capture.enabled = false as an unchecked switch", async () => {
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    const sw = await screen.findByRole("switch", { name: /Enable Capture/i });
    expect(sw).toHaveAttribute("aria-checked", "false");
    expect(screen.getByText("Disabled")).toBeInTheDocument();
  });

  it("reflects spec.capture.enabled = true as a checked switch", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={() => {}} />);
    const sw = await screen.findByRole("switch", { name: /Enable Capture/i });
    expect(sw).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText("Enabled")).toBeInTheDocument();
  });

  it("toggling the switch flips spec.capture.enabled", async () => {
    const onChange = vi.fn();
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={onChange} />);
    const sw = await screen.findByRole("switch", { name: /Enable Capture/i });
    await userEvent.click(sw);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          capture: expect.objectContaining({ enabled: true }),
        }),
      }),
    );
  });

  it("toggling back off sets spec.capture.enabled = false, not undefined", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={onChange} />);
    const sw = await screen.findByRole("switch", { name: /Enable Capture/i });
    await userEvent.click(sw);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          capture: expect.objectContaining({ enabled: false }),
        }),
      }),
    );
  });

  it("states plainly that disabling stops captures immediately but the sidecar stays until pod recreation", async () => {
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    await screen.findByRole("switch", { name: /Enable Capture/i });
    expect(
      screen.getByText(/stops any running capture and blocks new ones immediately/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/capture container itself stays in the pod, idle, until the pod is next recreated/i),
    ).toBeInTheDocument();
    // Must not be softened into "removes the sidecar".
    expect(screen.queryByText(/removes the sidecar/i)).not.toBeInTheDocument();
  });

  it("shows the admin-access / unredacted-data warning", async () => {
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    await screen.findByRole("switch", { name: /Enable Capture/i });
    expect(
      screen.getByText(/network packet capture requires admin access/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/are not redacted/i)).toBeInTheDocument();
  });

  it("defaults the retention unit to days for the 86400s cluster default", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true, retentionSeconds: 86400 } },
    };
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={() => {}} />);
    const value = await screen.findByLabelText("Retention window value");
    expect(value).toHaveValue("1");
    const unit = screen.getByLabelText("Retention window unit") as HTMLSelectElement;
    expect(unit.value).toBe("days");
  });

  it("converts a value entered in days to seconds on spec.capture.retentionSeconds", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={onChange} />);
    const unit = await screen.findByLabelText("Retention window unit");
    fireEvent.change(unit, { target: { value: "days" } });
    const value = screen.getByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "2" } });

    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.capture.retentionSeconds).toBe(172800);
  });

  it("rejects a retention value above the cluster maximum of 604800 seconds", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    const onValidityChange = vi.fn();
    renderWithQuery(
      <NetworkCaptureSection draft={draft} onChange={onChange} onValidityChange={onValidityChange} />,
    );
    const unit = await screen.findByLabelText("Retention window unit");
    fireEvent.change(unit, { target: { value: "days" } });
    const value = screen.getByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "8" } }); // 8 days > 7-day cluster max

    expect(
      screen.getByText(/exceeds the cluster maximum of 7 days \(604,800 seconds\)/i),
    ).toBeInTheDocument();
    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    // The invalid value must not be written to the draft.
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          capture: expect.objectContaining({ retentionSeconds: 691200 }),
        }),
      }),
    );
  });

  it("clears the field back to 'use cluster default' when emptied", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true, retentionSeconds: 3600 } },
    };
    const onChange = vi.fn();
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={onChange} />);
    const value = await screen.findByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "" } });

    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.capture.retentionSeconds).toBeUndefined();
  });

  it("explains the cluster maximum as a storage-limitation default, not a legal requirement", async () => {
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    await screen.findByRole("switch", { name: /Enable Capture/i });
    expect(
      screen.getByText(/storage-limitation-informed default, not a legal requirement/i),
    ).toBeInTheDocument();
  });

  it("disables the controls for a session without captures:manage", async () => {
    server.use(
      http.get("/users/me", () => HttpResponse.json(makeUser({ role: "operator" }))),
    );
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    // Ensure /users/me has resolved by waiting for the permission error text.
    await screen.findByText(/don't have permission to change capture settings/i);
    const sw = screen.getByRole("switch", { name: /Enable Capture/i });
    expect(sw).toBeDisabled();
    expect(screen.getByLabelText("Retention window value")).toBeDisabled();
    expect(screen.getByLabelText("Retention window unit")).toBeDisabled();
  });

  it("leaves the controls enabled for an admin session", async () => {
    renderWithQuery(<NetworkCaptureSection draft={baseDraft} onChange={() => {}} />);
    const sw = await screen.findByRole("switch", { name: /Enable Capture/i });
    await waitFor(() => expect(sw).not.toBeDisabled());
  });

  it("defaults to 'minutes' unit for a 5-minute retention (300 seconds)", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true, retentionSeconds: 300 } },
    };
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={() => {}} />);
    const value = await screen.findByLabelText("Retention window value");
    expect(value).toHaveValue("5");
    const unit = screen.getByLabelText("Retention window unit") as HTMLSelectElement;
    expect(unit.value).toBe("minutes");
  });

  it("defaults to 'seconds' unit for a non-divisible-by-60 retention (61 seconds)", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true, retentionSeconds: 61 } },
    };
    renderWithQuery(<NetworkCaptureSection draft={draft} onChange={() => {}} />);
    const value = await screen.findByLabelText("Retention window value");
    expect(value).toHaveValue("61");
    const unit = screen.getByLabelText("Retention window unit") as HTMLSelectElement;
    expect(unit.value).toBe("seconds");
  });

  it("rejects '0' as the retention value, shows error, and does not call onChange with invalid value", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    const onValidityChange = vi.fn();
    renderWithQuery(
      <NetworkCaptureSection draft={draft} onChange={onChange} onValidityChange={onValidityChange} />,
    );
    const value = await screen.findByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "0" } });

    expect(screen.getByText(/Enter a positive number\./)).toBeInTheDocument();
    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    // Ensure onChange was never called with retentionSeconds: 0
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          capture: expect.objectContaining({ retentionSeconds: 0 }),
        }),
      }),
    );
  });

  it("rejects a negative retention value, shows error, and does not call onChange with invalid value", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    const onValidityChange = vi.fn();
    renderWithQuery(
      <NetworkCaptureSection draft={draft} onChange={onChange} onValidityChange={onValidityChange} />,
    );
    const value = await screen.findByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "-5" } });

    expect(screen.getByText(/Enter a positive number\./)).toBeInTheDocument();
    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    // Ensure onChange was never called with a negative retentionSeconds
    expect(onChange).not.toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          capture: expect.objectContaining({ retentionSeconds: -300 }),
        }),
      }),
    );
  });

  it("rejects a non-numeric retention value, shows error, and does not call onChange", async () => {
    const draft = {
      ...baseDraft,
      spec: { ...baseDraft.spec, capture: { enabled: true } },
    };
    const onChange = vi.fn();
    const onValidityChange = vi.fn();
    renderWithQuery(
      <NetworkCaptureSection draft={draft} onChange={onChange} onValidityChange={onValidityChange} />,
    );
    const value = await screen.findByLabelText("Retention window value");
    fireEvent.change(value, { target: { value: "not-a-number" } });

    expect(screen.getByText(/Enter a positive number\./)).toBeInTheDocument();
    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    // onChange should not be called with any valid retentionSeconds when input is invalid
    const calls = onChange.mock.calls;
    const hasValidCall = calls.some((c) =>
      c[0]?.spec?.capture?.retentionSeconds !== undefined &&
      typeof c[0]?.spec?.capture?.retentionSeconds === "number" &&
      Number.isFinite(c[0]?.spec?.capture?.retentionSeconds),
    );
    expect(hasValidCall).toBe(false);
  });
});

describe("Settings sub-nav — Network capture position", () => {
  it("renders the 'Network capture' entry between 'Scheduled backups' and 'Placement'", async () => {
    renderWithQuery(<SettingsTab gs={baseDraft} name={baseDraft.metadata.name} />);
    const nav = await screen.findByRole("navigation");
    const labels = within(nav)
      .getAllByRole("button")
      .map((b) => b.textContent?.trim());

    const backupsIdx = labels.indexOf("Scheduled backups");
    const captureIdx = labels.indexOf("Network capture");
    const placementIdx = labels.indexOf("Placement");

    expect(backupsIdx).toBeGreaterThanOrEqual(0);
    expect(captureIdx).toBe(backupsIdx + 1);
    expect(placementIdx).toBe(captureIdx + 1);
  });

  it("navigates to the Network capture section on click", async () => {
    renderWithQuery(<SettingsTab gs={baseDraft} name={baseDraft.metadata.name} />);
    const navButton = await screen.findByRole("button", { name: /Network capture/i });
    await userEvent.click(navButton);

    expect(await screen.findByText(/Records raw network protocol traffic/i)).toBeInTheDocument();
  });
});
