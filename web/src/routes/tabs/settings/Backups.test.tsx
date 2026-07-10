import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider, QueryClient } from "@tanstack/react-query";
import { BackupsSection } from "./Backups";
import { makeServer } from "@/test/factories";

const baseDraft = makeServer();

function renderWithQuery(component: React.ReactNode) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={qc}>
      {component}
    </QueryClientProvider>
  );
}

describe("BackupsSection", () => {
  it("renders with backupPolicy disabled by default", () => {
    renderWithQuery(
      <BackupsSection draft={baseDraft} onChange={() => {}} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable scheduled backups/i });
    expect(sw).toHaveAttribute("aria-checked", "false");
  });

  it("shows 'Disabled' text when backupPolicy is not set", () => {
    renderWithQuery(
      <BackupsSection draft={baseDraft} onChange={() => {}} />,
    );
    expect(screen.getByText("Disabled")).toBeInTheDocument();
  });

  it("toggling Enable on creates default backupPolicy", async () => {
    const onChange = vi.fn();
    renderWithQuery(
      <BackupsSection draft={baseDraft} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable scheduled backups/i });
    await userEvent.click(sw);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          backupPolicy: expect.objectContaining({
            schedule: "0 */6 * * *",
            repoRef: expect.objectContaining({
              key: "repo",
            }),
          }),
        }),
      }),
    );
  });

  it("toggling Enable off clears backupPolicy", async () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
        },
      },
    };
    const onChange = vi.fn();
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={onChange} />,
    );
    const sw = screen.getByRole("switch", { name: /Enable scheduled backups/i });
    await userEvent.click(sw);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({
          backupPolicy: undefined,
        }),
      }),
    );
  });

  it("shows policy fields when backupPolicy is set", () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
        },
      },
    };
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={() => {}} />,
    );

    expect(screen.getByDisplayValue("0 0 * * *")).toBeInTheDocument();
    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByText("Retention policy")).toBeInTheDocument();
  });

  it("edits the schedule cron", async () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
        },
      },
    };
    const onChange = vi.fn();
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={onChange} />,
    );

    const scheduleInput = screen.getByDisplayValue("0 0 * * *");
    await userEvent.clear(scheduleInput);
    await userEvent.type(scheduleInput, "0 12 * * *");

    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.backupPolicy.schedule).toBe("0 12 * * *");
  });

  it("toggles suspend state", async () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
        },
      },
    };
    const onChange = vi.fn();
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={onChange} />,
    );

    const suspendSwitches = screen.getAllByRole("switch");
    const suspendSwitch = suspendSwitches[1]; // Second switch is the suspend one
    await userEvent.click(suspendSwitch);

    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.backupPolicy.suspend).toBe(true);
  });

  it("shows 'Active' when suspend is false or undefined", () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
          suspend: false,
        },
      },
    };
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={() => {}} />,
    );

    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("shows 'Suspended' when suspend is true", () => {
    const draftWithPolicy = {
      ...baseDraft,
      spec: {
        ...baseDraft.spec,
        backupPolicy: {
          schedule: "0 0 * * *",
          repoRef: { name: "backup-repo", key: "repo" },
          suspend: true,
        },
      },
    };
    renderWithQuery(
      <BackupsSection draft={draftWithPolicy} onChange={() => {}} />,
    );

    expect(screen.getByText("Suspended")).toBeInTheDocument();
  });
});
