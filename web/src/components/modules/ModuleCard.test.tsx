import { describe, it, expect, vi } from "vitest";
import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to, ...rest }: { children: ReactNode; to: string } & Record<string, unknown>) => (
    <a href={to} {...rest}>{children}</a>
  ),
}));

import { ModuleCard } from "./ModuleCard";
import { makeCatalog } from "@/test/factories";

const handlers = { onInstall: vi.fn(), onUpgrade: vi.fn(), onUninstall: vi.fn() };

describe("ModuleCard", () => {
  it("not-installed shows Install action", async () => {
    const onInstall = vi.fn();
    render(
      <ModuleCard
        entry={makeCatalog({ installed: false })}
        {...handlers}
        onInstall={onInstall}
      />,
    );
    expect(screen.getByText(/available/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Install/i }));
    expect(onInstall).toHaveBeenCalled();
  });

  it("installed at current version shows Deploy + Uninstall", () => {
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          installedVersion: "1.21",
          latestVersion: "1.21",
          phase: "Ready",
          moduleName: "minecraft-vanilla",
        })}
        {...handlers}
      />,
    );
    expect(screen.getByRole("link", { name: /Deploy/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Uninstall/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^Install$/ })).not.toBeInTheDocument();
  });

  it("upgrade-available shows Upgrade button", async () => {
    const onUpgrade = vi.fn();
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          installedVersion: "1.20",
          latestVersion: "1.21",
          phase: "Ready",
          moduleName: "minecraft-vanilla",
        })}
        {...handlers}
        onUpgrade={onUpgrade}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /Upgrade/i }));
    expect(onUpgrade).toHaveBeenCalled();
  });

  it("Failed phase shows danger pill and last-error message", () => {
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          phase: "Failed",
          lastError: "image-pull-backoff",
        })}
        {...handlers}
      />,
    );
    expect(screen.getByText(/failed/i)).toBeInTheDocument();
    expect(screen.getByText(/image-pull-backoff/)).toBeInTheDocument();
  });

  it("in-flight (phase != Ready) disables Uninstall", () => {
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          phase: "Pulling",
        })}
        {...handlers}
      />,
    );
    expect(screen.getByRole("button", { name: /Uninstall/i })).toBeDisabled();
  });

  it("multiple sources renders 'N sources'", () => {
    render(
      <ModuleCard
        entry={makeCatalog({ sources: [{ name: "a", type: "oci" }, { name: "b", type: "git" }, { name: "c", type: "upload" }] })}
        {...handlers}
      />,
    );
    expect(screen.getByText("3 sources")).toBeInTheDocument();
  });

  it("renders a keyless verify badge", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyless", enforced: false, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.getByText("keyless")).toBeInTheDocument();
  });

  it("renders a 'signed' badge for keyed verification", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyed", enforced: true, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.getByText("signed")).toBeInTheDocument();
  });

  it("renders no verify badge when unsigned", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "none", enforced: false, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.queryByText("keyless")).not.toBeInTheDocument();
    expect(screen.queryByText("signed")).not.toBeInTheDocument();
  });

  it("shows the bundle digest and rollback target", () => {
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          installedVersion: "1.21",
          phase: "Ready",
          appliedDigest: "sha256:a1b2c3d4e5f6a7b8c9d0e1f2",
          previousVersion: "1.20",
        })}
        {...handlers}
      />,
    );
    expect(screen.getByText(/sha256:a1b2c3d4/)).toBeInTheDocument();
    expect(screen.getByText(/rollback target/)).toBeInTheDocument();
  });
});
