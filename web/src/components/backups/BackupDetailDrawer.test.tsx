import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup } from "@/test/factories";
import { BackupDetailDrawer } from "./BackupDetailDrawer";

describe("BackupDetailDrawer", () => {
  it("does not render content when name is null", () => {
    renderWithQuery(<BackupDetailDrawer name={null} onClose={() => {}} onRestore={() => {}} />);
    expect(screen.queryByText(/Backup details/i)).not.toBeInTheDocument();
  });

  it("renders details when open", async () => {
    server.use(
      http.get("/backups/alpha-1", () =>
        HttpResponse.json(
          makeBackup({
            metadata: { name: "alpha-1" },
            status: {
              phase: "Succeeded",
              snapshotID: "abc123",
              completionTime: "2026-05-07T03:00:00Z",
              size: "120 MiB",
            },
          }),
        ),
      ),
    );
    renderWithQuery(
      <BackupDetailDrawer name="alpha-1" onClose={() => {}} onRestore={() => {}} />,
    );
    expect(await screen.findByText(/Backup details/i)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Succeeded")).toBeInTheDocument());
  });

  it("Close button calls onClose", async () => {
    server.use(
      http.get("/backups/alpha-1", () =>
        HttpResponse.json(makeBackup({ metadata: { name: "alpha-1" } })),
      ),
    );
    const onClose = vi.fn();
    renderWithQuery(
      <BackupDetailDrawer name="alpha-1" onClose={onClose} onRestore={() => {}} />,
    );
    await screen.findByText(/Backup details/i);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("renders an error banner on fetch failure", async () => {
    server.use(
      http.get("/backups/x", () => HttpResponse.text("not found", { status: 404 })),
    );
    renderWithQuery(<BackupDetailDrawer name="x" onClose={() => {}} onRestore={() => {}} />);
    await screen.findByText(/Backup details/i);
    await waitFor(() => expect(screen.getByText(/not found/)).toBeInTheDocument());
  });
});
