import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ResourcesSection } from "./Resources";
import { makeServer } from "@/test/factories";

describe("ResourcesSection", () => {
  it("renders default CPU and memory values when no resources set", () => {
    render(<ResourcesSection draft={makeServer()} onChange={() => {}} />);
    // Defaults are 2 CPU / 4 Gi.
    expect(screen.getByLabelText("CPU cores value")).toHaveValue(2);
    expect(screen.getByLabelText("Memory (GiB) value")).toHaveValue(4);
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

  it("setting storageClassName", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const scInput = screen.getByPlaceholderText("fast-ssd");
    fireEvent.change(scInput, { target: { value: "fast-nvme" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage.storageClassName).toBe("fast-nvme");
  });

  it("clearing storageClassName leaves storage undefined if no size", () => {
    const onChange = vi.fn();
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: { ...makeServer().spec, storage: { storageClassName: "fast-ssd" } },
        }}
        onChange={onChange}
      />,
    );
    const scInput = screen.getByPlaceholderText("fast-ssd");
    fireEvent.change(scInput, { target: { value: "" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage).toBeUndefined();
  });

  it("preserves storage size when setting storageClassName", () => {
    const onChange = vi.fn();
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: { ...makeServer().spec, storage: { size: "20Gi" } },
        }}
        onChange={onChange}
      />,
    );
    const scInput = screen.getByPlaceholderText("fast-ssd");
    fireEvent.change(scInput, { target: { value: "fast-nvme" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage.size).toBe("20Gi");
    expect(lastCall.spec.storage.storageClassName).toBe("fast-nvme");
  });

  it("renders storage section when storage already set", () => {
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: {
            ...makeServer().spec,
            storage: { size: "50Gi", storageClassName: "fast-ssd", mountPath: "/data" },
          },
        }}
        onChange={() => {}}
      />,
    );
    expect(screen.getByDisplayValue("50Gi")).toBeInTheDocument();
    expect(screen.getByDisplayValue("fast-ssd")).toBeInTheDocument();
  });

  it("CPU slider to 0 clamps to the 100m floor", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const cpuSlider = screen.getAllByRole("slider")[0];
    // ResourceInput's CPU_DEFAULTS floors the slider at 0.1 cores (100m) —
    // a raw DOM value of "0" (below the slider's own min) still gets
    // clamped there by emitBase, it never reaches true zero.
    fireEvent.change(cpuSlider, { target: { value: "0" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.resources.requests.cpu).toBe("100m");
  });

  it("memory slider min value", () => {
    const onChange = vi.fn();
    render(<ResourcesSection draft={makeServer()} onChange={onChange} />);
    const memSlider = screen.getAllByRole("slider")[1];
    fireEvent.change(memSlider, { target: { value: "1" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.resources.requests.memory).toBe("1Gi");
  });

  it("preserves storage config when updating resources", () => {
    const onChange = vi.fn();
    render(
      <ResourcesSection
        draft={{
          ...makeServer(),
          spec: {
            ...makeServer().spec,
            storage: { size: "10Gi" },
          },
        }}
        onChange={onChange}
      />,
    );
    const cpuSlider = screen.getAllByRole("slider")[0];
    fireEvent.change(cpuSlider, { target: { value: "8" } });
    const lastCall = onChange.mock.calls.at(-1)![0];
    expect(lastCall.spec.storage.size).toBe("10Gi");
    expect(lastCall.spec.resources.requests.cpu).toBe("8");
  });
});
