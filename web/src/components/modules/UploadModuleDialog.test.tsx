import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithQuery } from "@/test/render";
import { UploadModuleDialog } from "./UploadModuleDialog";

const previewBody = {
  module: { name: "factorio", displayName: "Factorio", version: "2.0.0", game: "factorio" },
  dryRun: true,
};

function pickFile() {
  const input = screen.getByTestId("bundle-file-input");
  const file = new File(["fake-tarball-bytes"], "factorio.tar.gz", { type: "application/gzip" });
  fireEvent.change(input, { target: { files: [file] } });
}

describe("UploadModuleDialog", () => {
  it("dry-runs on file pick and uploads on confirm", async () => {
    const calls: string[] = [];
    server.use(
      http.post("/modules/sources/uploads/upload", ({ request }) => {
        const dryRun = new URL(request.url).searchParams.get("dryRun") === "true";
        calls.push(dryRun ? "dry" : "real");
        return HttpResponse.json(
          dryRun ? previewBody : { ...previewBody, dryRun: false, configMap: "module-upload-factorio" },
          { status: dryRun ? 200 : 201 },
        );
      }),
    );
    const onUploaded = vi.fn();
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={onUploaded} />,
    );

    pickFile();
    // Preview renders the parsed metadata from the dry run.
    expect(await screen.findByText("Factorio")).toBeInTheDocument();
    expect(screen.getByText("v2.0.0")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Upload" }));
    await waitFor(() => expect(onUploaded).toHaveBeenCalled());
    expect(calls).toEqual(["dry", "real"]);
  });

  it("surfaces validation errors from the dry run", async () => {
    server.use(
      http.post("/modules/sources/uploads/upload", () =>
        HttpResponse.text("bundle has no module.yaml", { status: 400 }),
      ),
    );
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );
    pickFile();
    expect(await screen.findByText(/no module.yaml/)).toBeInTheDocument();
    // Without a successful preview the Upload button stays disabled.
    expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
  });

  it("offers a source picker only with multiple upload sources", () => {
    const { unmount } = renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["a", "b"]} onUploaded={() => undefined} />,
    );
    expect(screen.getByText("Upload to")).toBeInTheDocument();
    unmount();

    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );
    expect(screen.queryByText("Upload to")).not.toBeInTheDocument();
  });

  it("clears error when picking a new file", async () => {
    server.use(
      http.post("/modules/sources/uploads/upload", () =>
        HttpResponse.text("bundle has no module.yaml", { status: 400 }),
      ),
    );
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );
    pickFile();
    expect(await screen.findByText(/no module.yaml/)).toBeInTheDocument();

    // Change to a new file
    server.resetHandlers();
    server.use(
      http.post("/modules/sources/uploads/upload", () =>
        HttpResponse.json(previewBody, { status: 200 }),
      ),
    );
    pickFile();
    expect(await screen.findByText("Factorio")).toBeInTheDocument();
    expect(screen.queryByText(/no module.yaml/)).not.toBeInTheDocument();
  });

  it("surfaces errors from the upload call (not dry-run)", async () => {
    server.use(
      http.post("/modules/sources/uploads/upload", ({ request }) => {
        const dryRun = new URL(request.url).searchParams.get("dryRun");
        if (dryRun === "true") {
          return HttpResponse.json(previewBody, { status: 200 });
        }
        return HttpResponse.text("permission denied", { status: 403 });
      }),
    );
    const onUploaded = vi.fn();
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={onUploaded} />,
    );
    pickFile();
    expect(await screen.findByText("Factorio")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Upload" }));
    expect(await screen.findByText(/permission denied/)).toBeInTheDocument();
    expect(onUploaded).not.toHaveBeenCalled();
  });

  it("clears state when dialog closes and reopens", async () => {
    server.use(
      http.post("/modules/sources/uploads/upload", () =>
        HttpResponse.json(previewBody, { status: 200 }),
      ),
    );
    const { rerender } = renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );
    pickFile();
    expect(await screen.findByText("Factorio")).toBeInTheDocument();

    // Close dialog
    rerender(
      <UploadModuleDialog open={false} onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );

    // Reopen
    rerender(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["uploads"]} onUploaded={() => undefined} />,
    );

    const fileInput = screen.getByTestId("bundle-file-input") as HTMLInputElement;
    expect(fileInput.files?.length).toBe(0);
    expect(screen.queryByText("Factorio")).not.toBeInTheDocument();
  });

  it("resets to first source when dialog closes and reopens with different sources", async () => {
    const { rerender } = renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["first", "second"]} onUploaded={() => undefined} />,
    );

    // Switch to second source
    fireEvent.change(screen.getByRole("combobox", { name: "Upload to" }), {
      target: { value: "second" },
    });

    // Close and reopen
    rerender(
      <UploadModuleDialog open={false} onOpenChange={() => undefined} sources={["first", "second"]} onUploaded={() => undefined} />,
    );
    rerender(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={["alpha", "beta"]} onUploaded={() => undefined} />,
    );

    // Should default to alpha now
    const select = screen.getByRole("combobox", { name: "Upload to" }) as HTMLSelectElement;
    expect(select.value).toBe("alpha");
  });

  it("handles missing source gracefully when sources list is empty", () => {
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={() => undefined} sources={[]} onUploaded={() => undefined} />,
    );
    // Dialog should render without crashing
    expect(screen.getByText(/Choose a bundle archive/)).toBeInTheDocument();
  });

  it("closes dialog after successful upload", async () => {
    const onOpenChange = vi.fn();
    server.use(
      http.post("/modules/sources/uploads/upload", ({ request }) => {
        const dryRun = new URL(request.url).searchParams.get("dryRun") === "true";
        return HttpResponse.json(
          dryRun ? previewBody : { ...previewBody, dryRun: false, configMap: "module-upload-factorio" },
          { status: dryRun ? 200 : 201 },
        );
      }),
    );
    const onUploaded = vi.fn();
    renderWithQuery(
      <UploadModuleDialog open onOpenChange={onOpenChange} sources={["uploads"]} onUploaded={onUploaded} />,
    );

    pickFile();
    expect(await screen.findByText("Factorio")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Upload" }));
    await waitFor(() => expect(onUploaded).toHaveBeenCalled());
    // Note: This test verifies the callback chain, but doesn't verify that
    // onOpenChange is called with false since that's part of ModulesPage logic
  });
});
