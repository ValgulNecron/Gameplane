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
});
