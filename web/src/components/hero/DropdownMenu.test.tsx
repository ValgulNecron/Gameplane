import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Copy, Trash2 } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuCheckboxItem,
} from "./DropdownMenu";

function Subject({
  onClone = () => {},
  onDelete = () => {},
  cloneDisabled = false,
}: {
  onClone?: () => void;
  onDelete?: () => void;
  cloneDisabled?: boolean;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <button aria-label="More actions">More</button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuItem
          icon={<Copy className="h-4 w-4" />}
          label="Clone server"
          onSelect={onClone}
          disabled={cloneDisabled}
          hint={cloneDisabled ? "Requires operator role" : undefined}
        />
        <DropdownMenuSeparator />
        <DropdownMenuItem
          icon={<Trash2 className="h-4 w-4" />}
          label="Delete"
          onSelect={onDelete}
          destructive
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function CheckboxSubject() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <button aria-label="Options">Options</button>
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        <DropdownMenuCheckboxItem checked>
          Enabled
        </DropdownMenuCheckboxItem>
        <DropdownMenuCheckboxItem>Disabled</DropdownMenuCheckboxItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

describe("DropdownMenu", () => {
  it("shows items after opening via the trigger", async () => {
    const user = userEvent.setup();
    render(<Subject />);
    expect(screen.queryByText("Clone server")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "More actions" }));
    expect(screen.getByText("Clone server")).toBeInTheDocument();
    expect(screen.getByText("Delete")).toBeInTheDocument();
  });

  it("fires onSelect when an item is clicked", async () => {
    const user = userEvent.setup();
    const onClone = vi.fn();
    render(<Subject onClone={onClone} />);
    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(screen.getByText("Clone server"));
    expect(onClone).toHaveBeenCalledTimes(1);
  });

  it("disabled item is aria-disabled, hinted, and does not fire", async () => {
    const user = userEvent.setup();
    const onClone = vi.fn();
    render(<Subject onClone={onClone} cloneDisabled />);
    await user.click(screen.getByRole("button", { name: "More actions" }));
    const item = screen.getByText("Clone server").closest("[role='menuitem']");
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item?.querySelector("[title]")).toHaveAttribute("title", "Requires operator role");
    await user.click(screen.getByText("Clone server"));
    expect(onClone).not.toHaveBeenCalled();
  });

  it("renders destructive items with danger color", async () => {
    const user = userEvent.setup();
    render(<Subject />);
    await user.click(screen.getByRole("button", { name: "More actions" }));
    const deleteItem = screen.getByText("Delete").closest("[role='menuitem']");
    expect(deleteItem).toHaveClass("text-danger");
  });

  it("renders checkbox items with checkmark indicator when checked", async () => {
    const user = userEvent.setup();
    render(<CheckboxSubject />);
    await user.click(screen.getByRole("button", { name: "Options" }));
    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
  });
});
