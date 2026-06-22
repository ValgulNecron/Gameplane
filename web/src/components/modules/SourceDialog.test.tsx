import { describe, it, expect, vi } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { renderWithQuery } from "@/test/render";
import { makeModuleSource } from "@/test/factories";
import type { ModuleSourceSpec } from "@/types";
import { APIError } from "@/lib/api";
import { SourceDialog, specFrom } from "./SourceDialog";

function renderDialog(props: Partial<Parameters<typeof SourceDialog>[0]> = {}) {
  const onConfirm = vi.fn();
  renderWithQuery(
    <SourceDialog
      open
      onOpenChange={() => undefined}
      source={null}
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return onConfirm;
}

describe("specFrom", () => {
  const base = {
    type: "oci" as const,
    url: "",
    modules: "",
    secretName: "",
    insecure: false,
    ref: "",
    subPath: "",
    path: "",
    allow: "",
    refreshInterval: "",
    verifyMode: "none" as const,
    verifyKeySecret: "",
    verifyIssuer: "",
    verifyIdentity: "",
  };

  it("builds an oci spec", () => {
    const spec: ModuleSourceSpec = specFrom({
      ...base,
      url: "ghcr.io/x",
      modules: "minecraft-java, valheim",
      secretName: "creds",
      insecure: true,
      allow: "minecraft-*",
      refreshInterval: "30m",
    });
    expect(spec).toEqual({
      type: "oci",
      oci: {
        url: "ghcr.io/x",
        modules: [{ name: "minecraft-java" }, { name: "valheim" }],
        insecure: true,
        pullSecretRef: { name: "creds" },
      },
      allow: ["minecraft-*"],
      refreshInterval: "30m",
    });
  });

  it("builds a git spec without empty optionals", () => {
    const spec = specFrom({ ...base, type: "git", url: "https://g/x.git", ref: "stable" });
    expect(spec).toEqual({ type: "git", git: { url: "https://g/x.git", ref: "stable" } });
  });

  it("builds http, local and upload specs", () => {
    expect(specFrom({ ...base, type: "http", url: "https://e/m.zip" })).toEqual({
      type: "http",
      http: { url: "https://e/m.zip" },
    });
    expect(specFrom({ ...base, type: "local", path: "bundles" })).toEqual({
      type: "local",
      local: { path: "bundles" },
    });
    expect(specFrom({ ...base, type: "local" })).toEqual({ type: "local", local: {} });
    expect(specFrom({ ...base, type: "upload" })).toEqual({ type: "upload" });
  });

  it("emits keyed and keyless verify only for oci sources", () => {
    const keyed = specFrom({
      ...base,
      url: "ghcr.io/x",
      modules: "mc",
      verifyMode: "keyed",
      verifyKeySecret: "cosign-pub",
    });
    expect(keyed.verify).toEqual({ key: { name: "cosign-pub" } });

    const keyless = specFrom({
      ...base,
      url: "ghcr.io/x",
      modules: "mc",
      verifyMode: "keyless",
      verifyIssuer: "https://issuer",
      verifyIdentity: "id@example.com",
    });
    expect(keyless.verify).toEqual({
      keyless: { issuer: "https://issuer", identity: "id@example.com" },
    });

    // none → no verify block.
    expect(specFrom({ ...base, url: "ghcr.io/x", modules: "mc" }).verify).toBeUndefined();

    // A non-oci type must never carry verify, even if the form has it set.
    const git = specFrom({
      ...base,
      type: "git",
      url: "https://g/x.git",
      verifyMode: "keyless",
      verifyIssuer: "https://issuer",
      verifyIdentity: "id@example.com",
    });
    expect(git.verify).toBeUndefined();
  });
});

describe("SourceDialog", () => {
  it("renders oci fields by default and switches per type", () => {
    renderDialog();
    expect(screen.getByText("Registry URL")).toBeInTheDocument();
    expect(screen.getByText("Modules")).toBeInTheDocument();

    // Switch to upload: no url fields, just the explainer.
    fireEvent.change(screen.getByRole("combobox", { name: "Type" }), {
      target: { value: "upload" },
    });
    expect(screen.queryByText("Registry URL")).not.toBeInTheDocument();
    expect(screen.getByText(/Indexes bundles uploaded/)).toBeInTheDocument();
  });

  it("renders the git, http and local field sets", () => {
    renderDialog();
    const typeSelect = screen.getByRole("combobox", { name: "Type" });

    fireEvent.change(typeSelect, { target: { value: "git" } });
    expect(screen.getByText("Clone URL")).toBeInTheDocument();
    expect(screen.getByText("Ref")).toBeInTheDocument();
    expect(screen.getByText("Subdirectory")).toBeInTheDocument();

    fireEvent.change(typeSelect, { target: { value: "http" } });
    expect(screen.getByText("Archive URL")).toBeInTheDocument();
    expect(screen.getByText(/Allow plain HTTP/)).toBeInTheDocument();

    fireEvent.change(typeSelect, { target: { value: "local" } });
    expect(screen.getByText("Path")).toBeInTheDocument();
    expect(screen.queryByText("Archive URL")).not.toBeInTheDocument();
  });

  it("prefills edit forms for oci and local sources", () => {
    const oci = makeModuleSource({
      metadata: { name: "upstream" },
      spec: {
        type: "oci",
        oci: {
          url: "ghcr.io/x",
          modules: [{ name: "mc" }, { name: "valheim" }],
          insecure: true,
          pullSecretRef: { name: "creds" },
        },
        allow: ["mc"],
        refreshInterval: "15m",
      },
    });
    const { unmount } = renderWithQuery(
      <SourceDialog open onOpenChange={() => undefined} source={oci} onConfirm={vi.fn()} />,
    );
    expect(screen.getByDisplayValue("ghcr.io/x")).toBeInTheDocument();
    expect(screen.getByDisplayValue("mc, valheim")).toBeInTheDocument();
    expect(screen.getByDisplayValue("creds")).toBeInTheDocument();
    expect(screen.getByDisplayValue("15m")).toBeInTheDocument();
    unmount();

    const local = makeModuleSource({
      metadata: { name: "disk" },
      spec: { type: "local", local: { path: "bundles" } },
    });
    renderWithQuery(
      <SourceDialog open onOpenChange={() => undefined} source={local} onConfirm={vi.fn()} />,
    );
    expect(screen.getByDisplayValue("bundles")).toBeInTheDocument();
  });

  it("validates before submitting", async () => {
    const onConfirm = renderDialog();
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await screen.findByText(/name is required/);

    fireEvent.change(screen.getByPlaceholderText("community"), { target: { value: "up" } });
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await screen.findByText(/url is required/);

    fireEvent.change(screen.getByPlaceholderText("ghcr.io/valgulnecron/gameplane-modules"), {
      target: { value: "ghcr.io/x" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await screen.findByText(/at least one module/);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("submits the typed payload", async () => {
    const onConfirm = renderDialog();
    fireEvent.change(screen.getByPlaceholderText("community"), { target: { value: "upstream" } });
    fireEvent.change(screen.getByPlaceholderText("ghcr.io/valgulnecron/gameplane-modules"), {
      target: { value: "ghcr.io/x" },
    });
    fireEvent.change(screen.getByPlaceholderText("minecraft-java, valheim"), {
      target: { value: "minecraft-java" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalled());
    expect(onConfirm).toHaveBeenCalledWith({
      name: "upstream",
      spec: {
        type: "oci",
        oci: { url: "ghcr.io/x", modules: [{ name: "minecraft-java" }] },
      },
    });
  });

  it("reveals keyless verify fields and validates them", async () => {
    const onConfirm = renderDialog();
    fireEvent.change(screen.getByPlaceholderText("community"), { target: { value: "upstream" } });
    fireEvent.change(screen.getByPlaceholderText("ghcr.io/valgulnecron/gameplane-modules"), {
      target: { value: "ghcr.io/x" },
    });
    fireEvent.change(screen.getByPlaceholderText("minecraft-java, valheim"), {
      target: { value: "minecraft-java" },
    });

    // Keyless reveals issuer + identity inputs.
    fireEvent.change(screen.getByRole("combobox", { name: /Signature verification/ }), {
      target: { value: "keyless" },
    });
    expect(screen.getByText("OIDC issuer")).toBeInTheDocument();
    const identity = screen.getByPlaceholderText(/release.yml/);
    fireEvent.change(screen.getByPlaceholderText("https://token.actions.githubusercontent.com"), {
      target: { value: "https://issuer" },
    });

    // Blank identity blocks submit.
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await screen.findByText(/keyless verification needs an issuer and identity/);
    expect(onConfirm).not.toHaveBeenCalled();

    // Filling it submits the verify block.
    fireEvent.change(identity, { target: { value: "id@example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "Add source" }));
    await waitFor(() => expect(onConfirm).toHaveBeenCalled());
    expect(onConfirm).toHaveBeenCalledWith({
      name: "upstream",
      spec: {
        type: "oci",
        oci: { url: "ghcr.io/x", modules: [{ name: "minecraft-java" }] },
        verify: { keyless: { issuer: "https://issuer", identity: "id@example.com" } },
      },
    });
  });

  it("prefills the verify config when editing a signed oci source", () => {
    const signed = makeModuleSource({
      metadata: { name: "upstream" },
      spec: {
        type: "oci",
        oci: { url: "ghcr.io/x", modules: [{ name: "mc" }] },
        verify: { keyless: { issuer: "https://issuer", identity: "id@example.com" } },
      },
    });
    renderWithQuery(
      <SourceDialog open onOpenChange={() => undefined} source={signed} onConfirm={vi.fn()} />,
    );
    expect(screen.getByDisplayValue("https://issuer")).toBeInTheDocument();
    expect(screen.getByDisplayValue("id@example.com")).toBeInTheDocument();
  });

  it("prefills when editing and surfaces API errors", async () => {
    const source = makeModuleSource({
      metadata: { name: "community" },
      spec: { type: "git", git: { url: "https://g/x.git", ref: "main" } },
    });
    const onConfirm = vi.fn().mockRejectedValue(new APIError(409, "still used by installed module(s)"));
    renderWithQuery(
      <SourceDialog open onOpenChange={() => undefined} source={source} onConfirm={onConfirm} />,
    );
    // Name is fixed when editing; git fields are prefilled.
    expect(screen.queryByPlaceholderText("community")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("https://g/x.git")).toBeInTheDocument();
    expect(screen.getByDisplayValue("main")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await screen.findByText(/still used by installed module/);
  });
});
