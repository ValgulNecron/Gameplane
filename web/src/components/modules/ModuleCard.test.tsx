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

  it("VersionUnavailable Failed offers an Update instead of a dead error", async () => {
    const onUpgrade = vi.fn();
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          phase: "Failed",
          reason: "VersionUnavailable",
          pinnedVersion: "2.0.2",
          versions: ["2.0.4"],
          latestVersion: "2.0.4",
          lastError: 'version "2.0.2" not in catalog for "7dtd" (available: [2.0.4])',
          moduleName: "seven-days-to-die",
        })}
        {...handlers}
        onUpgrade={onUpgrade}
      />,
    );
    // Actionable "update" pill, not a red "failed", and no raw error dump.
    expect(screen.getByText("update")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(screen.queryByText(/not in catalog/)).not.toBeInTheDocument();
    // Explains the situation and offers the available version.
    expect(screen.getByText(/no longer in the catalog/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Update to v2\.0\.4/i }));
    expect(onUpgrade).toHaveBeenCalled();
  });

  it("does not flash a stale error mid-re-pin (spec.version now available, status still Failed)", () => {
    // Right after clicking Update: spec.version=2.0.4 (available) but the CR
    // status hasn't caught up (still Failed with the old error). Neither the
    // raw error nor the update affordance should show.
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          phase: "Failed",
          reason: "VersionUnavailable",
          pinnedVersion: "2.0.4",
          versions: ["2.0.4"],
          latestVersion: "2.0.4",
          lastError: 'version "2.0.2" not in catalog for "7dtd" (available: [2.0.4])',
          moduleName: "seven-days-to-die",
        })}
        {...handlers}
      />,
    );
    expect(screen.queryByText(/not in catalog/)).not.toBeInTheDocument();
    expect(screen.queryByText(/no longer in the catalog/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Update to/i })).not.toBeInTheDocument();
  });

  it("Pulling with a leftover lastError does not show the stale error", () => {
    render(
      <ModuleCard
        entry={makeCatalog({
          installed: true,
          phase: "Pulling",
          reason: "VersionUnavailable",
          pinnedVersion: "2.0.4",
          versions: ["2.0.4"],
          latestVersion: "2.0.4",
          lastError: 'version "2.0.2" not in catalog for "7dtd" (available: [2.0.4])',
        })}
        {...handlers}
      />,
    );
    expect(screen.queryByText(/not in catalog/)).not.toBeInTheDocument();
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

  it("enforced verification renders a solid 'verified' badge (keyed)", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyed", enforced: true, mixed: false }}
        {...handlers}
      />,
    );
    const badge = screen.getByText("verified");
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass("bg-success/15");
    expect(badge).toHaveAttribute("title", "signature verified");
    expect(screen.queryByText("policy")).not.toBeInTheDocument();
  });

  it("enforced keyless verification keeps the 'verified' label, with keyless in the tooltip", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyless", enforced: true, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.getByText("verified")).toBeInTheDocument();
    expect(screen.getByTitle(/keyless/i)).toBeInTheDocument();
  });

  it("policy-declared (not installed) renders a softer outline 'policy' badge", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyed", enforced: false, mixed: false }}
        {...handlers}
      />,
    );
    const badge = screen.getByText("policy");
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass("border-success/40");
    expect(badge).not.toHaveClass("bg-success/15");
    expect(screen.queryByText("verified")).not.toBeInTheDocument();
  });

  it("keyless policy carries keyless in the tooltip", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "keyless", enforced: false, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.getByText("policy")).toBeInTheDocument();
    expect(screen.getByTitle(/keyless/i)).toBeInTheDocument();
  });

  it("mixed candidate sources suppress the badge (no over-claim)", () => {
    for (const mode of ["keyless", "keyed"] as const) {
      const { unmount } = render(
        <ModuleCard
          entry={makeCatalog({})}
          verify={{ mode, enforced: false, mixed: true }}
          {...handlers}
        />,
      );
      expect(screen.queryByText("verified")).not.toBeInTheDocument();
      expect(screen.queryByText("policy")).not.toBeInTheDocument();
      unmount();
    }
  });

  it("renders no verify badge when unsigned", () => {
    render(
      <ModuleCard
        entry={makeCatalog({})}
        verify={{ mode: "none", enforced: false, mixed: false }}
        {...handlers}
      />,
    );
    expect(screen.queryByText("verified")).not.toBeInTheDocument();
    expect(screen.queryByText("policy")).not.toBeInTheDocument();
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
