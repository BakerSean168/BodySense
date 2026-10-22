/**
 * Format an untrusted value for direct UI display.
 *
 * Plain String() renders objects and arrays as "[object Object]"; this
 * helper JSON-encodes them instead and normalizes nullish to "".
 */
export function formatDisplayValue(value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "string") return value;
  if (
    typeof value === "number" ||
    typeof value === "boolean" ||
    typeof value === "bigint"
  ) {
    return String(value);
  }
  try {
    return JSON.stringify(value) ?? "";
  } catch {
    return "";
  }
}
