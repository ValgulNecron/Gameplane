import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { makeBackup, makeServer } from "@/test/factories";
import { RestoreDialog } from "./RestoreDialog";

describe("RestoreDialog", () => {
  it("does not render when backup is null", () => {
    renderWithQuery(<RestoreDialog backup={null} onClose={() => {}} />);
    expect(screen.queryByText(/Restore from backup/i)).not.toBeInTheDocument();
  });

  it("renders dialog and the backup name", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha" } }),
            makeServer({ metadata: { name: "beta" } }),
          ],
        }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({ metadata: { name: "alpha-1" } })}
        onClose={() => {}}
      />,
    );
    expect(await screen.findByText(/Restore from backup/i)).toBeInTheDocument();
    expect(screen.getByText("alpha-1")).toBeInTheDocument();
  });

  it("submits a restore POST when target is selected", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
      http.post("/restores", () => HttpResponse.json({})),
    );
    const onClose = vi.fn();
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({ metadata: { name: "alpha-1" } })}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );
    await screen.findByText(/Restore from backup/i);
    const buttons = screen.getAllByRole("button");
    const restoreBtn = buttons.find((b) => /Restore$/.test(b.textContent ?? ""));
    if (!restoreBtn) throw new Error("Restore button not found");
    await userEvent.click(restoreBtn);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("surfaces error when restore POST fails", async () => {
    server.use(
      http.get("/servers", () => HttpResponse.json({ items: [] })),
      http.post("/restores", () => HttpResponse.text("boom", { status: 500 })),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({ metadata: { name: "x" } })}
        defaultServer="alpha"
        onClose={() => {}}
      />,
    );
    await screen.findByText(/Restore from backup/i);
    const buttons = screen.getAllByRole("button");
    const restoreBtn = buttons.find((b) => /Restore$/.test(b.textContent ?? ""));
    if (!restoreBtn) throw new Error("Restore button not found");
    await userEvent.click(restoreBtn);
    await waitFor(() => expect(screen.getByText(/boom/)).toBeInTheDocument());
  });
});
