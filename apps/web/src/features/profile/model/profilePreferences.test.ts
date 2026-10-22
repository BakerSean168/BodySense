import { describe, expect, it } from "vitest";
import {
  effectiveTimeZone,
  effectiveUnitPreference,
  formatInstantInZone,
  formatMeasurement,
  selectableTimeZones,
} from "./profilePreferences";

describe("profile preferences", () => {
  it("labels an unset time zone as the device default and rejects server-local", () => {
    expect(effectiveTimeZone({ time_zone: "Asia/Tokyo" })).toEqual({
      zone: "Asia/Tokyo",
      isDefault: false,
    });
    expect(effectiveTimeZone(null).isDefault).toBe(true);
    expect(effectiveTimeZone({ time_zone: "Local" }).isDefault).toBe(true);
    expect(effectiveUnitPreference({})).toEqual({
      unit: "metric",
      isDefault: true,
    });
  });

  it("formats the same instant on the preferred clock without changing it", () => {
    const instant = "2026-10-18T16:30:00Z";
    expect(formatInstantInZone(instant, "Asia/Shanghai")).toContain("00:30");
    expect(formatInstantInZone(instant, "America/New_York")).toContain("12:30");
    expect(formatInstantInZone("not a date", "UTC")).toBeNull();
  });

  it("converts length and mass for display only and keeps the recorded value", () => {
    expect(formatMeasurement(170, "cm", "metric")).toEqual({
      text: "170 cm",
      original: null,
    });
    expect(formatMeasurement(170, "cm", "imperial")).toEqual({
      text: "5 ft 7 in",
      original: "170 cm",
    });
    expect(formatMeasurement(65.5, "kg", "imperial")).toEqual({
      text: "144.4 lb",
      original: "65.5 kg",
    });
    // Units without a defined conversion are never invented.
    expect(formatMeasurement(120, "mmHg", "imperial")).toEqual({
      text: "120 mmHg",
      original: null,
    });
  });

  it("always offers the stored zone even when the browser list lacks it", () => {
    expect(selectableTimeZones("Asia/Shanghai")).toContain("Asia/Shanghai");
  });
});
