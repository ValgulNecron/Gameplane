import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ResourcesSection } from "./Resources";
import { makeServer } from "@/test/factories";

describe("ResourcesSection", () => {
  it("renders default CPU and memory values when no resources set", () => {
    render(<ResourcesSection draft={makeServer()} onChange={() => {}} />);
    // Defaults are 2 CPU / 4 Gi.
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("4 Gi")).toBeInTheDocument();
  });

  it("changing CPU slider updates draft to a quantity", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const cpuSlider = screen.getAllByRole("slider")[0];
    fireEvent.change(cpuSlider, { target: { value: "4" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.resources.requests.cpu).toBe("4");
    expect(lastCall.spec.resources.limits.cpu).toBe("4");
  });

  it("fractional CPU produces a milli-string", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const cpuSlider = screen.getAllByRole("slider")[0];
    fireEvent.change(cpuSlider, { target: { value: "0.5" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.resources.requests.cpu).toBe("500m");
  });

  it("memory slider sets memory limits", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const memSlider = screen.getAllByRole("slider")[1];
    fireEvent.change(memSlider, { target: { value: "8" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.resources.requests.memory).toBe("8Gi");
  });

  it("rejects an invalid storage size with an error message", () => {
    const onChange = vi.fn();
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: { ...makeServer().spec, storage: { size: "10Pi-bad" } },
        }}
        onChange={onChange}
      />,
    );
    expect(screen.getByText(/Invalid quantity/i)).toBeInTheDocument();
  });

  it("typing valid storage sets spec.storage.size", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const sizeInput = screen.getByPlaceholderText("10Gi");
    fireEvent.change(sizeInput, { target: { value: "20Gi" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage.size).toBe("20Gi");
  });

  it("clearing storage size leaves storage undefined", () => {
    const onChange = vi.fn();
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: { ...makeServer().spec, storage: { size: "5Gi" } },
        }}
        onChange={onChange}
      />,
    );
    const sizeInput = screen.getByPlaceholderText("10Gi");
    fireEvent.change(sizeInput, { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage).toBeUndefined();
  });
});
