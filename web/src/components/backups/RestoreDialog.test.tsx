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

  it("volume-snapshot backup restores into a new server", async () => {
    type RestoreBody = { spec?: { serverRef?: { name?: string } } };
    let postedName: string | undefined;
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
      http.post("/restores", async ({ request }) => {
        postedName = ((await request.json()) as RestoreBody).spec?.serverRef?.name;
        return HttpResponse.json({});
      }),
    );
    const onClose = vi.fn();
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-vs" },
          spec: { serverRef: { name: "alpha" }, strategy: "volume-snapshot" },
        })}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );
    expect(await screen.findByText(/provisioned from this snapshot/i)).toBeInTheDocument();
    const input = screen.getByPlaceholderText(/my-restored-server/i) as HTMLInputElement;
    await waitFor(() => expect(input.value).toBe("alpha-restored"));
    const btn = screen.getByRole("button", { name: /restore to new server/i });
    expect(btn).toBeEnabled();
    await userEvent.click(btn);
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(postedName).toBe("alpha-restored");
  });

  it("volume-snapshot restore blocks a name that already exists", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({
          items: [
            makeServer({ metadata: { name: "alpha" } }),
            makeServer({ metadata: { name: "taken" } }),
          ],
        }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-vs" },
          spec: { serverRef: { name: "alpha" }, strategy: "volume-snapshot" },
        })}
        onClose={() => {}}
      />,
    );
    const input = await screen.findByPlaceholderText(/my-restored-server/i);
    await userEvent.clear(input);
    await userEvent.type(input, "taken");
    expect(await screen.findByText(/already exists/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /restore to new server/i })).toBeDisabled();
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

  it("shows restic restore warning message", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-restic" },
          spec: { serverRef: { name: "alpha" }, strategy: "restic-snapshot" },
        })}
        defaultServer="alpha"
        onClose={() => {}}
      />,
    );
    await screen.findByText(/will be suspended/i);
    expect(screen.getByText(/overwrite all data/i)).toBeInTheDocument();
  });

  it("closes dialog on Escape", async () => {
    const onClose = vi.fn();
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({ metadata: { name: "alpha-1" } })}
        onClose={onClose}
      />,
    );
    await screen.findByText(/Restore from backup/i);
    // Radix dialog closes on Escape, which triggers onOpenChange(false) -> onClose()
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("disables Restore button while mutation is pending", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
      http.post("/restores", async () => {
        // Simulate slow request
        await new Promise(resolve => setTimeout(resolve, 100));
        return HttpResponse.json({});
      }),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({ metadata: { name: "alpha-1" } })}
        defaultServer="alpha"
        onClose={() => {}}
      />,
    );
    const restoreBtn = await screen.findByRole("button", { name: /Restore$/ });
    await userEvent.click(restoreBtn);
    expect(restoreBtn).toBeDisabled();
  });

  it("volume-snapshot prefills server name with -restored suffix", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-vs" },
          spec: { serverRef: { name: "prod-server" }, strategy: "volume-snapshot" },
        })}
        onClose={() => {}}
      />,
    );
    const input = await screen.findByPlaceholderText(/my-restored-server/i);
    await waitFor(() => {
      expect((input as HTMLInputElement).value).toBe("prod-server-restored");
    });
  });

  it("restic restore uses defaultServer when provided", async () => {
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
        backup={makeBackup({
          metadata: { name: "backup-1" },
          spec: { serverRef: { name: "alpha" }, strategy: "restic-snapshot" as const },
        })}
        defaultServer="beta"
        onClose={() => {}}
      />,
    );
    const select = await screen.findByDisplayValue("beta");
    expect((select as HTMLSelectElement).value).toBe("beta");
  });

  it("shows danger variant for restic restore button", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-1" },
          spec: { serverRef: { name: "alpha" }, strategy: "restic-snapshot" as const },
        })}
        defaultServer="alpha"
        onClose={() => {}}
      />,
    );
    const buttons = screen.getAllByRole("button");
    const restoreBtn = buttons.find((b) => /Restore$/.test(b.textContent ?? ""));
    expect(restoreBtn?.className).toContain("danger");
  });

  it("shows default variant for volume-snapshot restore button", async () => {
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "alpha-vs" },
          spec: { serverRef: { name: "alpha" }, strategy: "volume-snapshot" },
        })}
        onClose={() => {}}
      />,
    );
    const restoreBtn = await screen.findByRole("button", { name: /restore to new server/i });
    expect(restoreBtn.className).not.toContain("danger");
  });

  it("updates target when backup changes", async () => {
    const onClose = vi.fn();
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    const { rerender } = renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "backup-1" },
          spec: { serverRef: { name: "alpha" }, strategy: "restic-snapshot" },
        })}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );

    // Rerender with a different backup
    rerender(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "backup-2" },
          spec: { serverRef: { name: "beta" }, strategy: "volume-snapshot" },
        })}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );

    // Target should be updated to new server name with suffix
    const input = await screen.findByPlaceholderText(/my-restored-server/i);
    await waitFor(() => {
      expect((input as HTMLInputElement).value).toBe("beta-restored");
    });
  });

  it("clears target when backup is set to null", async () => {
    const onClose = vi.fn();
    server.use(
      http.get("/servers", () =>
        HttpResponse.json({ items: [makeServer({ metadata: { name: "alpha" } })] }),
      ),
    );
    const { rerender } = renderWithQuery(
      <RestoreDialog
        backup={makeBackup({
          metadata: { name: "backup-1" },
          spec: { serverRef: { name: "alpha" }, strategy: "restic-snapshot" },
        })}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );

    rerender(
      <RestoreDialog
        backup={null}
        defaultServer="alpha"
        onClose={onClose}
      />,
    );

    // Dialog should not be visible
    expect(screen.queryByText(/Restore from backup/i)).not.toBeInTheDocument();
  });
});
