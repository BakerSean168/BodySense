import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import { describe, expect, it } from "vitest";
import invalidEvents from "../fixtures/stream-events.invalid.v1.json";
import semanticProbes from "../fixtures/stream-events.semantic-probes.v1.json";
import realEvents from "../fixtures/stream-events.v1.json";
import schema from "../schemas/stream-event.v1.schema.json";
import { parseStreamEvent } from "./stream-event-parser";

const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  strictRequired: true,
});
addFormats(ajv);
const validateSchema = ajv.compile(schema);

function manualAccepts(event: unknown): boolean {
  try {
    parseStreamEvent(event);
    return true;
  } catch {
    return false;
  }
}

function schemaAccepts(event: unknown): boolean {
  return validateSchema(event) === true;
}

describe("canonical StreamEvent JSON Schema parity", () => {
  it("strict-compiles the canonical schema", () => {
    expect(validateSchema).toBeTypeOf("function");
  });

  it("accepts all 34 real fixtures exactly like the handwritten parser", () => {
    expect(realEvents).toHaveLength(34);
    for (const event of realEvents) {
      expect(manualAccepts(event)).toBe(true);
      expect(schemaAccepts(event)).toBe(true);
    }
  });

  it("rejects the 10 committed malformed fixtures exactly like the handwritten parser", () => {
    expect(invalidEvents).toHaveLength(10);
    for (const { name, event } of invalidEvents) {
      expect(manualAccepts(event), `${name}: handwritten parser`).toBe(false);
      expect(schemaAccepts(event), `${name}: canonical schema`).toBe(false);
    }
  });

  it("matches the five spike-proven semantic probes", () => {
    expect(semanticProbes).toHaveLength(5);
    for (const { name, event, should_accept: shouldAccept } of semanticProbes) {
      expect(manualAccepts(event), `${name}: handwritten parser`).toBe(
        shouldAccept,
      );
      expect(schemaAccepts(event), `${name}: canonical schema`).toBe(
        shouldAccept,
      );
    }
  });
});
