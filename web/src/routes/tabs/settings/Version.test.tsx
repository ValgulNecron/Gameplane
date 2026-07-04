import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { VersionSection } from "./Version";
import { makeServer, makeTemplate } from "@/test/factories";
import type { GameTemplate } from "@/types";

function versionedTemplate(): GameTemplate {
  const tmpl = makeTemplate();
  tmpl.spec.versions = [
    { id: "1.21.4-paper", displayName: "1.21.4 · Paper", loader: "paper", gameVersion: "1.21.4", default: true },
    { id: "1.21.4-fabric", displayName: "1.21.4 · Fabric", loader: "fabric", gameVersion: "1.21.4" },
    { id: "latest-paper", displayName: "Latest · Paper", loader: "paper" },
  ];
  tmpl.spec.capabilities = {
    mods: { loaders: { paper: { path: "plugins" }, fabric: { path: "mods" } } },
  };
  return tmpl;
}

describe("VersionSection", () => {
  it("renders the catalog with the selected entry checked", () => {
    const draft = makeServer();
    draft.spec.version = "1.21.4-fabric";
    render(<VersionSection draft={draft} onChange={() => {}} template={versionedTemplate()} />);

    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(3);
    expect(screen.getByRole("radio", { name: /Fabric/ })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByRole("radio", { name: /1\.21\.4 · Paper/ })).toHaveAttribute("aria-checked", "false");
    expect(screen.getByText("Default")).toBeInTheDocument();
  });

  it("falls back to the default entry when spec.version is unset", () => {
    render(<VersionSection draft={makeServer()} onChange={() => {}} template={versionedTemplate()} />);
    expect(screen.getByRole("radio", { name: /1\.21\.4 · Paper/ })).toHaveAttribute("aria-checked", "true");
  });

  it("picking an entry writes spec.version", async () => {
    const onChange = vi.fn();
    render(<VersionSection draft={makeServer()} onChange={onChange} template={versionedTemplate()} />);
    await userEvent.click(screen.getByRole("radio", { name: /Fabric/ }));
    const next = onChange.mock.calls.at(-1)![0];
    expect(next.spec.version).toBe("1.21.4-fabric");
  });

  it("shows the mod-volume callout when the template has per-loader volumes", () => {
    render(<VersionSection draft={makeServer()} onChange={() => {}} template={versionedTemplate()} />);
    expect(screen.getByText(/keeps its own mod volume/)).toBeInTheDocument();
  });

  it("hides the mod-volume callout without loader volumes", () => {
    const tmpl = versionedTemplate();
    tmpl.spec.capabilities = undefined;
    render(<VersionSection draft={makeServer()} onChange={() => {}} template={tmpl} />);
    expect(screen.queryByText(/keeps its own mod volume/)).not.toBeInTheDocument();
  });

  it("warns about an image override and clears it", async () => {
    const draft = makeServer({ spec: { templateRef: { name: "x" }, image: "my/pin:1" } });
    const onChange = vi.fn();
    render(<VersionSection draft={draft} onChange={onChange} template={versionedTemplate()} />);
    expect(screen.getByText(/Image override active/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Clear override/ }));
    const next = onChange.mock.calls.at(-1)![0];
    expect(next.spec.image).toBeUndefined();
  });

  it("explains itself when the template has no catalog", () => {
    render(<VersionSection draft={makeServer()} onChange={() => {}} template={makeTemplate()} />);
    expect(screen.getByText(/no version catalog/)).toBeInTheDocument();
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
  });
});
