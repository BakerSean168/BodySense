/**
 * Exhaustiveness guard for switches over closed unions. When a new union
 * member appears, calls to this function stop compiling until the switch
 * handles it; at runtime it throws instead of silently returning undefined.
 */
export function assertNever(value: never): never {
  throw new Error(`Unexpected value: ${String(value)}`);
}
