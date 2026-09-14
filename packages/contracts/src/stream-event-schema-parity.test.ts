import { describe, expect, it } from "vitest";
import invalidEvents from "../fixtures/stream-events.invalid.v1.json";
import semanticProbes from "../fixtures/stream-events.semantic-probes.v1.json";
import realEvents from "../fixtures/stream-events.v1.json";
import {
  generatedStreamEventValidationErrors,
  validateGeneratedStreamEvent,
} from "./generated-stream-event";
import { parseStreamEvent } from "./stream-event-parser";

function parserAccepts(event: unknown): boolean {
  try {
    parseStreamEvent(event);
    return true;
  } catch {
    return false;
  }
}

describe("canonical generated StreamEvent authority", () => {
  it("accepts all real fixtures through both the generated validator and stable parser facade", () => {
    expect(realEvents).toHaveLength(34);
    for (const event of realEvents) {
      expect(validateGeneratedStreamEvent(event)).toBe(true);
      expect(parserAccepts(event)).toBe(true);
    }
  });

  it("rejects all 10 malformed fixtures through both entry points", () => {
    expect(invalidEvents).toHaveLength(10);
    for (const { name, event } of invalidEvents) {
      expect(
        validateGeneratedStreamEvent(event),
        `${name}: generated validator`,
      ).toBe(false);
      expect(parserAccepts(event), `${name}: parser facade`).toBe(false);
    }
  });

  it("matches the five spike-proven semantic probes", () => {
    expect(semanticProbes).toHaveLength(5);
    for (const { name, event, should_accept: shouldAccept } of semanticProbes) {
      expect(
        validateGeneratedStreamEvent(event),
        `${name}: generated validator`,
      ).toBe(shouldAccept);
      expect(parserAccepts(event), `${name}: parser facade`).toBe(shouldAccept);
    }
  });

  it("exposes stable validation diagnostics without leaking Ajv into the facade", () => {
    expect(validateGeneratedStreamEvent({ version: 2 })).toBe(false);
    expect(generatedStreamEventValidationErrors().length).toBeGreaterThan(0);
    expect(generatedStreamEventValidationErrors()[0]).toEqual(
      expect.objectContaining({
        instancePath: expect.any(String),
        schemaPath: expect.any(String),
        keyword: expect.any(String),
      }),
    );
  });
});
