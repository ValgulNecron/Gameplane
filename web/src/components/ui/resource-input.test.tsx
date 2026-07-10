import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ResourceInput } from "./resource-input";

describe("ResourceInput", () => {
  describe("CPU mode", () => {
    it("renders slider, numeric input, and unit dropdown for CPU", () => {
      render(
        <ResourceInput
          kind="cpu"
          value="2"
          onChange={() => {}}
          id="test-cpu"
        />
      );
      expect(screen.getByRole("slider")).toBeInTheDocument();
      expect(screen.getByRole("combobox")).toBeInTheDocument();
      expect(screen.getByDisplayValue("2")).toBeInTheDocument();
    });

    it("parses canonical CPU quantity and displays in natural unit", () => {
      render(
        <ResourceInput
          kind="cpu"
          value="500m"
          onChange={() => {}}
        />
      );
      // 500m = 0.5 cores, but should display as 500 in mCPU unit
      const input = screen.getByDisplayValue("500") as HTMLInputElement;
      expect(input).toBeInTheDocument();
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      expect(select.value).toBe("m");
    });

    it("moving the slider calls onChange with canonical CPU string", () => {
      const fn = vi.fn();
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={fn}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      fireEvent.change(slider, { target: { value: "2" } });
      expect(fn).toHaveBeenCalledWith("2");
    });

    it("emits fractional CPU as millicores", () => {
      const fn = vi.fn();
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={fn}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      fireEvent.change(slider, { target: { value: "0.5" } });
      expect(fn).toHaveBeenCalledWith("500m");
    });

    it("parses input in current display unit and emits canonical CPU string", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={fn}
        />
      );
      const input = screen.getByDisplayValue("1") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "0.5");
      fireEvent.blur(input);
      // 0.5 cores should emit as "500m"
      expect(fn).toHaveBeenCalledWith("500m");
    });

    it("switching CPU unit converts displayed value without changing emitted amount", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={fn}
        />
      );
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      // Start in cores, value is "1"
      expect(screen.getByDisplayValue("1")).toBeInTheDocument();
      // Switch to mCPU
      await user.selectOptions(select, "m");
      // Input should now show 1000 (1 core = 1000 millicores)
      expect(screen.getByDisplayValue("1000")).toBeInTheDocument();
      // onChange should not have been called just from unit switch
      // (the last call is from typing/blur, but unit change alone doesn't emit)
      // Verify that the value hasn't changed by checking the slider still reflects 1 core
      const slider = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(slider.value)).toBe(1);
    });

    it("clamps input outside min/max range on blur", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="2"
          onChange={fn}
          min={0.5}
          max={8}
        />
      );
      const input = screen.getByDisplayValue("2") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "20");
      fireEvent.blur(input);
      // Should clamp to max (8) and emit "8"
      expect(fn).toHaveBeenCalledWith("8");
    });

    it("respects default CPU limits when min/max omitted", () => {
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={() => {}}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(slider.min)).toBe(0.5);
      expect(Number(slider.max)).toBe(16);
    });
  });

  describe("Memory mode", () => {
    it("renders slider, numeric input, and unit dropdown for memory", () => {
      render(
        <ResourceInput
          kind="memory"
          value="4Gi"
          onChange={() => {}}
          id="test-mem"
        />
      );
      expect(screen.getByRole("slider")).toBeInTheDocument();
      expect(screen.getByRole("combobox")).toBeInTheDocument();
      expect(screen.getByDisplayValue("4")).toBeInTheDocument();
    });

    it("parses canonical memory quantity and displays in natural unit", () => {
      render(
        <ResourceInput
          kind="memory"
          value="512Mi"
          onChange={() => {}}
        />
      );
      // 512Mi should display as 512 in Mi unit (not converted to Gi)
      const input = screen.getByDisplayValue("512") as HTMLInputElement;
      expect(input).toBeInTheDocument();
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      expect(select.value).toBe("Mi");
    });

    it("moving the slider calls onChange with canonical memory string", () => {
      const fn = vi.fn();
      render(
        <ResourceInput
          kind="memory"
          value="2"
          onChange={fn}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      fireEvent.change(slider, { target: { value: "4" } });
      expect(fn).toHaveBeenCalledWith("4Gi");
    });

    it("emits fractional memory as smaller unit", () => {
      const fn = vi.fn();
      render(
        <ResourceInput
          kind="memory"
          value="4"
          onChange={fn}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      fireEvent.change(slider, { target: { value: "1.5" } });
      // 1.5 Gi should emit as "1536Mi" (integral millibinary unit)
      expect(fn).toHaveBeenCalledWith("1536Mi");
    });

    it("parses input in current display unit and emits canonical memory string", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="memory"
          value="1Gi"
          onChange={fn}
        />
      );
      const input = screen.getByDisplayValue("1") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "2");
      fireEvent.blur(input);
      // 2 in Gi unit should emit "2Gi"
      expect(fn).toHaveBeenCalledWith("2Gi");
    });

    it("switching memory unit converts displayed value without changing emitted amount", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="memory"
          value="4Gi"
          onChange={fn}
        />
      );
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      // Start in Gi, value is "4"
      expect(screen.getByDisplayValue("4")).toBeInTheDocument();
      // Switch to MiB
      await user.selectOptions(select, "Mi");
      // Input should now show 4096 (4 Gi = 4096 MiB)
      expect(screen.getByDisplayValue("4096")).toBeInTheDocument();
      // Verify the slider still reflects 4 GiB
      const slider = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(slider.value)).toBe(4);
    });

    it("clamps input outside min/max range on blur", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="memory"
          value="4"
          onChange={fn}
          min={1}
          max={16}
        />
      );
      const input = screen.getByDisplayValue("4") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "100");
      fireEvent.blur(input);
      // Should clamp to max (16 GiB) and emit "16Gi"
      expect(fn).toHaveBeenCalledWith("16Gi");
    });

    it("respects default memory limits when min/max omitted", () => {
      render(
        <ResourceInput
          kind="memory"
          value="1"
          onChange={() => {}}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(slider.min)).toBe(0.5);
      expect(Number(slider.max)).toBe(64);
    });

    it("treats unparseable value as min", () => {
      render(
        <ResourceInput
          kind="memory"
          value="invalid"
          onChange={() => {}}
          min={1}
          max={16}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(slider.value)).toBe(1);
    });
  });

  describe("Synchronized behavior", () => {
    it("slider and input stay in sync when slider moves", () => {
      const fn = vi.fn();
      render(
        <ResourceInput
          kind="memory"
          value="2Gi"
          onChange={fn}
        />
      );
      const slider = screen.getByRole("slider") as HTMLInputElement;
      // Move slider to 4
      fireEvent.change(slider, { target: { value: "4" } });
      // Input should update to reflect 4 Gi
      expect(fn).toHaveBeenCalledWith("4Gi");
    });

    it("slider and input stay in sync when input is edited", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={fn}
        />
      );
      const input = screen.getByDisplayValue("1") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "4");
      fireEvent.blur(input);
      // Slider should reflect 4 cores
      expect(fn).toHaveBeenCalledWith("4");
    });

    it("all three controls can be toggled without data loss", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="2"
          onChange={fn}
        />
      );
      // Change unit from cores to mCPU
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      await user.selectOptions(select, "m");
      // Input should now show 2000
      expect(screen.getByDisplayValue("2000")).toBeInTheDocument();
      // Edit input back to some value
      const input = screen.getByDisplayValue("2000") as HTMLInputElement;
      await user.clear(input);
      await user.type(input, "1000");
      fireEvent.blur(input);
      // Should emit "1" (1000 millicores = 1 core)
      expect(fn).toHaveBeenCalledWith("1");
    });
  });

  describe("Accessibility", () => {
    it("passes aria-label to slider", () => {
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={() => {}}
        />
      );
      const slider = screen.getByRole("slider");
      expect(slider).toHaveAttribute("aria-label", "CPU cores");
    });

    it("provides aria-labels for input and select", () => {
      render(
        <ResourceInput
          kind="memory"
          value="4Gi"
          onChange={() => {}}
        />
      );
      const input = screen.getByLabelText("Memory (GiB) value");
      const select = screen.getByLabelText("Memory (GiB) unit");
      expect(input).toBeInTheDocument();
      expect(select).toBeInTheDocument();
    });

    it("disables all controls when disabled prop is true", () => {
      render(
        <ResourceInput
          kind="cpu"
          value="1"
          onChange={() => {}}
          disabled
        />
      );
      expect(screen.getByRole("slider")).toBeDisabled();
      expect(screen.getByRole("combobox")).toBeDisabled();
      const input = screen.getByDisplayValue("1") as HTMLInputElement;
      expect(input).toBeDisabled();
    });
  });

  describe("Edge cases", () => {
    it("handles empty input gracefully", async () => {
      const fn = vi.fn();
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="cpu"
          value="2"
          onChange={fn}
        />
      );
      const input = screen.getByDisplayValue("2") as HTMLInputElement;
      await user.clear(input);
      // Before blur, onChange should not be called
      expect(fn).not.toHaveBeenCalled();
      // After blur, it should snap to min
      fireEvent.blur(input);
      // min defaults to 0.5 for CPU
      expect(fn).toHaveBeenCalledWith("500m");
    });

    it("handles step property correctly on slider", () => {
      render(
        <ResourceInput
          kind="memory"
          value="2Gi"
          onChange={() => {}}
          step={2}
        />
      );
      const sliderEl = screen.getByRole("slider") as HTMLInputElement;
      expect(Number(sliderEl.step)).toBe(2);
    });

    it("converts Ki/Ti memory units if specified", async () => {
      const user = userEvent.setup();
      render(
        <ResourceInput
          kind="memory"
          value="1024Ki"
          onChange={() => {}}
        />
      );
      // Input should show 1024 in Ki
      expect(screen.getByDisplayValue("1024")).toBeInTheDocument();
      // Select should show "Ki"
      const select = screen.getByRole("combobox") as HTMLSelectElement;
      expect(select.value).toBe("Ki");
      // Convert to Gi
      await user.selectOptions(select, "Gi");
      // Should display 0.001 Gi (1 Ki = 1/1024 Mi = 1/(1024^2) Gi)
      expect(screen.getByDisplayValue("0.0009765625")).toBeInTheDocument();
    });
  });
});
