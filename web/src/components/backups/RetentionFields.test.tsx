import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { render } from "@testing-library/react";
import { RetentionFields, buildRetention, type RetentionForm } from "./RetentionFields";

describe("RetentionFields", () => {
  it("renders six input fields with correct labels", () => {
    const onChange = vi.fn();
    render(
      <RetentionFields
        value={{}}
        onChange={onChange}
      />
    );

    expect(screen.getByLabelText("Keep last")).toBeInTheDocument();
    expect(screen.getByLabelText("Hourly")).toBeInTheDocument();
    expect(screen.getByLabelText("Daily")).toBeInTheDocument();
    expect(screen.getByLabelText("Weekly")).toBeInTheDocument();
    expect(screen.getByLabelText("Monthly")).toBeInTheDocument();
    expect(screen.getByLabelText("Yearly")).toBeInTheDocument();
  });

  it("displays the provided values in the inputs", () => {
    const onChange = vi.fn();
    render(
      <RetentionFields
        value={{
          keepLast: 7,
          keepDaily: 30,
          keepMonthly: 12,
        }}
        onChange={onChange}
      />
    );

    expect((screen.getByLabelText("Keep last") as HTMLInputElement).value).toBe("7");
    expect((screen.getByLabelText("Daily") as HTMLInputElement).value).toBe("30");
    expect((screen.getByLabelText("Monthly") as HTMLInputElement).value).toBe("12");
    expect((screen.getByLabelText("Hourly") as HTMLInputElement).value).toBe("");
  });

  it("calls onChange when typing into an input", async () => {
    let currentValue: RetentionForm = { keepLast: 7 };
    const handleChange = (v: RetentionForm) => {
      currentValue = v;
      // Re-render with the new value to simulate parent component updating
      rerender(
        <RetentionFields value={currentValue} onChange={handleChange} />
      );
    };
    const { rerender } = render(
      <RetentionFields value={currentValue} onChange={handleChange} />
    );

    const dailyInput = screen.getByLabelText("Daily") as HTMLInputElement;
    await userEvent.type(dailyInput, "30");

    // After typing, the parent component tracks the new value
    expect(currentValue).toHaveProperty("keepDaily", 30);
    expect(currentValue).toHaveProperty("keepLast", 7);
  });

  it("converts empty input to undefined", async () => {
    const onChange = vi.fn();
    const initialValue: RetentionForm = { keepDaily: 30 };
    render(
      <RetentionFields value={initialValue} onChange={onChange} />
    );

    const dailyInput = screen.getByLabelText("Daily") as HTMLInputElement;
    await userEvent.clear(dailyInput);

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        keepDaily: undefined,
      })
    );
  });

  it("preserves other fields when updating one", async () => {
    let currentValue: RetentionForm = {
      keepLast: 7,
      keepHourly: 24,
      keepDaily: 30,
    };
    const handleChange = (v: RetentionForm) => {
      currentValue = v;
      // Re-render with the new value to simulate parent component updating
      rerender(
        <RetentionFields value={currentValue} onChange={handleChange} />
      );
    };
    const { rerender } = render(
      <RetentionFields value={currentValue} onChange={handleChange} />
    );

    const monthlyInput = screen.getByLabelText("Monthly") as HTMLInputElement;
    await userEvent.type(monthlyInput, "12");

    // All other fields should be preserved
    expect(currentValue).toHaveProperty("keepLast", 7);
    expect(currentValue).toHaveProperty("keepHourly", 24);
    expect(currentValue).toHaveProperty("keepDaily", 30);
    expect(currentValue).toHaveProperty("keepMonthly", 12);
  });
});

describe("buildRetention", () => {
  it("returns undefined when all values are empty or zero", () => {
    expect(buildRetention({})).toBeUndefined();
    expect(buildRetention({ keepLast: 0 })).toBeUndefined();
    expect(buildRetention({
      keepLast: 0,
      keepDaily: undefined,
      keepMonthly: 0,
    })).toBeUndefined();
  });

  it("omits buckets with value 0 or undefined", () => {
    const result = buildRetention({
      keepLast: 7,
      keepDaily: 30,
      keepWeekly: 0,
      keepMonthly: undefined,
      keepYearly: 5,
    });

    expect(result).toEqual({
      keepLast: 7,
      keepDaily: 30,
      keepYearly: 5,
    });
    expect(result).not.toHaveProperty("keepWeekly");
    expect(result).not.toHaveProperty("keepMonthly");
  });

  it("includes only buckets with positive values", () => {
    const result = buildRetention({
      keepLast: 7,
      keepHourly: 24,
      keepDaily: 30,
      keepWeekly: 4,
      keepMonthly: 12,
      keepYearly: 1,
    });

    expect(result).toEqual({
      keepLast: 7,
      keepHourly: 24,
      keepDaily: 30,
      keepWeekly: 4,
      keepMonthly: 12,
      keepYearly: 1,
    });
  });

  it("handles single bucket with a value", () => {
    const result = buildRetention({ keepDaily: 30 });
    expect(result).toEqual({ keepDaily: 30 });
  });
});
