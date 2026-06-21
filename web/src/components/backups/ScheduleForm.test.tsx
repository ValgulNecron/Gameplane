import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { ScheduleForm } from "./ScheduleForm";

describe("ScheduleForm", () => {
  it("renders the form with default schedule and the lone destination preselected", async () => {
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    expect(screen.getByDisplayValue("0 */6 * * *")).toBeInTheDocument();
    // Destination select defaults to the only configured destination
    // (the MSW handler returns one named "default").
    await waitFor(() => {
      expect(
        (screen.getByLabelText("Destination") as HTMLSelectElement).value,
      ).toBe("default");
    });
  });

  it("Cancel calls onClose", async () => {
    const onClose = vi.fn();
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={onClose} />);
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("Create button is disabled while schedule is empty", async () => {
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    const sched = screen.getByDisplayValue("0 */6 * * *");
    await userEvent.clear(sched);
    expect(screen.getByRole("button", { name: /Create schedule/i })).toBeDisabled();
  });

  it("Create posts to /schedules and closes", async () => {
    const onClose = vi.fn();
    let captured: unknown;
    server.use(
      http.post("/schedules", async ({ request }) => {
        captured = await request.json();
        return HttpResponse.json({});
      }),
    );
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={onClose} />);
    const create = screen.getByRole("button", { name: /Create schedule/i });
    await waitFor(() => expect(create).toBeEnabled());
    await userEvent.click(create);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(captured).toMatchObject({ spec: expect.objectContaining({ schedule: "0 */6 * * *" }) });
  });

  it("volume-snapshot type hides the destination and posts no repoRef", async () => {
    let postedStrategy: unknown;
    let hadRepoRef = true;
    server.use(
      http.post("/schedules", async ({ request }) => {
        const spec = ((await request.json()) as { spec?: Record<string, unknown> }).spec ?? {};
        postedStrategy = spec.strategy;
        hadRepoRef = "repoRef" in spec;
        return HttpResponse.json({});
      }),
    );
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    await userEvent.selectOptions(screen.getByLabelText("Backup type"), "volume-snapshot");
    expect(screen.queryByLabelText("Destination")).toBeNull();
    const create = screen.getByRole("button", { name: /Create schedule/i });
    await waitFor(() => expect(create).toBeEnabled());
    await userEvent.click(create);
    await waitFor(() => expect(postedStrategy).toBe("volume-snapshot"));
    expect(hadRepoRef).toBe(false);
  });

  it("renders an error banner if Create fails", async () => {
    server.use(
      http.post("/schedules", () =>
        HttpResponse.text("rejected", { status: 422 }),
      ),
    );
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    const create = screen.getByRole("button", { name: /Create schedule/i });
    await waitFor(() => expect(create).toBeEnabled());
    await userEvent.click(create);
    await waitFor(() =>
      expect(screen.getByText(/rejected/i)).toBeInTheDocument(),
    );
  });
});
