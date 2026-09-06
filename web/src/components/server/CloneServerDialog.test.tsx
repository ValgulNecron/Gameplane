import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CloneServerDialog } from "./CloneServerDialog";

// Mock the endpoints
vi.mock("@/lib/endpoints", () => ({
  Servers: {
    clone: vi.fn(),
  },
}));

// Mock the router
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
}));

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false },
    mutations: { retry: false },
  },
});

const Wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
);

describe("CloneServerDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    queryClient.clear();
  });

  it("renders when open", () => {
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="minecraft-server"
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("Clone server")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Creates a new server with the same configuration. World data is not copied.",
      ),
    ).toBeInTheDocument();
  });

  it("does not render when closed", () => {
    render(
      <CloneServerDialog
        open={false}
        onOpenChange={() => {}}
        sourceName="minecraft-server"
      />,
      { wrapper: Wrapper },
    );
    expect(screen.queryByText("Clone server")).not.toBeInTheDocument();
  });

  it("populates new name input with source name + '-copy' when opening", () => {
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="test-server"
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("test-server-copy");
    expect(input).toBeInTheDocument();
  });

  it("truncates long source names to 58 chars when creating suggested name", () => {
    const longName = "a".repeat(70);
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName={longName}
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("a".repeat(58) + "-copy");
    expect(input).toBeInTheDocument();
  });

  it("updates input value when user types", async () => {
    const user = userEvent.setup();
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="source"
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("source-copy") as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "new-server-name");
    expect(input.value).toBe("new-server-name");
  });

  it("shows validation error for invalid name", async () => {
    const user = userEvent.setup();
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="source"
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("source-copy") as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "INVALID_NAME");
    expect(
      screen.getByText("Name must be lowercase letters, digits, dashes (max 63)"),
    ).toBeInTheDocument();
  });

  it("disables clone button when name is invalid", async () => {
    const user = userEvent.setup();
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="source"
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("source-copy") as HTMLInputElement;
    await user.clear(input);
    await user.type(input, "INVALID");
    const cloneBtn = screen.getByRole("button", { name: "Clone server" });
    expect(cloneBtn).toBeDisabled();
  });

  it("enables clone button when name is valid", () => {
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="source"
      />,
      { wrapper: Wrapper },
    );
    const input = screen.getByDisplayValue("source-copy") as HTMLInputElement;
    expect(input).toBeInTheDocument();
    const cloneBtn = screen.getByRole("button", { name: "Clone server" });
    expect(cloneBtn).not.toBeDisabled();
  });

  it("calls onOpenChange(false) when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <CloneServerDialog
        open
        onOpenChange={onOpenChange}
        sourceName="source"
      />,
      { wrapper: Wrapper },
    );
    const cancelBtn = screen.getByRole("button", { name: "Cancel" });
    await user.click(cancelBtn);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("renders input with correct label", () => {
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="test"
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("New name")).toBeInTheDocument();
  });

  it("accepts namespace prop", () => {
    render(
      <CloneServerDialog
        open
        onOpenChange={() => {}}
        sourceName="test"
        ns="custom-namespace"
      />,
      { wrapper: Wrapper },
    );
    expect(screen.getByText("Clone server")).toBeInTheDocument();
  });
});
