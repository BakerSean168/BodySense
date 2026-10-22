import { describe, expect, it } from "vitest";

import { formatDisplayValue } from "./displayValue";

describe("formatDisplayValue", () => {
  it("passes strings through and normalizes nullish to empty", () => {
    expect(formatDisplayValue("abc")).toBe("abc");
    expect(formatDisplayValue(null)).toBe("");
    expect(formatDisplayValue(undefined)).toBe("");
  });

  it("stringifies primitives", () => {
    expect(formatDisplayValue(0)).toBe("0");
    expect(formatDisplayValue(3.5)).toBe("3.5");
    expect(formatDisplayValue(true)).toBe("true");
  });

  it("JSON-encodes objects and arrays instead of [object Object]", () => {
    expect(formatDisplayValue({ a: 1 })).toBe('{"a":1}');
    expect(formatDisplayValue([1, "x"])).toBe('[1,"x"]');
  });

  it("returns empty for values JSON cannot represent", () => {
    const circular: Record<string, unknown> = {};
    circular.self = circular;
    expect(formatDisplayValue(circular)).toBe("");
    expect(formatDisplayValue(Symbol("s"))).toBe("");
  });
});
