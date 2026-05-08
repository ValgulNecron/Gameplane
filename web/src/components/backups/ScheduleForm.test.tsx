import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { ScheduleForm } from "./ScheduleForm";

describe("ScheduleForm", () => {
  it("renders the form with default schedule", () => {
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    expect(screen.getByDisplayValue("0 */6 * * *")).toBeInTheDocument();
    expect(screen.getByDisplayValue("kestrel-backup-repo")).toBeInTheDocument();
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
    await userEvent.click(screen.getByRole("button", { name: /Create schedule/i }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(captured).toMatchObject({ spec: expect.objectContaining({ schedule: "0 */6 * * *" }) });
  });

  it("renders an error banner if Create fails", async () => {
    server.use(
      http.post("/schedules", () =>
        HttpResponse.text("rejected", { status: 422 }),
      ),
    );
    renderWithQuery(<ScheduleForm serverName="alpha" onClose={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: /Create schedule/i }));
    await waitFor(() =>
      expect(screen.getByText(/rejected/i)).toBeInTheDocument(),
    );
  });
});
