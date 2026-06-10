import type React from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BackupRow } from "./BackupRow";
import { makeBackup } from "@/test/factories";

function tableWrap(children: React.ReactNode) {
  return (
    <table>
      <tbody>{children}</tbody>
    </table>
  );
}

describe("BackupRow", () => {
  it("clicking the row calls onSelect", async () => {
    const onSelect = vi.fn();
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({ metadata: { name: "alpha-1" } })}
          showServer={true}
          onSelect={onSelect}
          onRestore={() => {}}
        />,
      ),
    );
    await userEvent.click(screen.getByText("alpha-1"));
    expect(onSelect).toHaveBeenCalled();
  });

  it("Restore button is enabled when phase=Succeeded with snapshotID", async () => {
    const onRestore = vi.fn();
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({
            metadata: { name: "alpha-1" },
            status: { phase: "Succeeded", snapshotID: "abc123" },
          })}
          showServer={false}
          onSelect={() => {}}
          onRestore={onRestore}
        />,
      ),
    );
    const btn = screen.getByRole("button", { name: "Restore" });
    expect(btn).toBeEnabled();
    await userEvent.click(btn);
    expect(onRestore).toHaveBeenCalled();
  });

  it("Restore button is disabled when no snapshotID", () => {
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({ status: { phase: "Succeeded" } })}
          showServer={false}
          onSelect={() => {}}
          onRestore={() => {}}
        />,
      ),
    );
    expect(screen.getByRole("button", { name: "Restore" })).toBeDisabled();
  });

  it("Restore button is disabled when phase is not Succeeded", () => {
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({ status: { phase: "Running", snapshotID: "x" } })}
          showServer={false}
          onSelect={() => {}}
          onRestore={() => {}}
        />,
      ),
    );
    expect(screen.getByRole("button", { name: "Restore" })).toBeDisabled();
  });

  it("hides server column when showServer=false", () => {
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({ spec: { serverRef: { name: "srv-x" } } })}
          showServer={false}
          onSelect={() => {}}
          onRestore={() => {}}
        />,
      ),
    );
    expect(screen.queryByText("srv-x")).not.toBeInTheDocument();
  });

  it("shows the server name when showServer=true", () => {
    render(
      tableWrap(
        <BackupRow
          backup={makeBackup({ spec: { serverRef: { name: "srv-x" } } })}
          showServer={true}
          onSelect={() => {}}
          onRestore={() => {}}
        />,
      ),
    );
    expect(screen.getByText("srv-x")).toBeInTheDocument();
  });
});
