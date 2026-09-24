import { describe, expect, it } from "vitest";
import type { HotUpdateOptions, Plugin } from "vite";
import {
  adaptTailwindPluginsForBundledDev,
  shouldUseBundledDev,
} from "./vite.config";

describe("Vite Bundled Dev lane", () => {
  it("enables bundled dev only for the explicit development serve lane", () => {
    expect(
      shouldUseBundledDev(
        { command: "serve", mode: "development" },
        { BODYSENSE_VITE_BUNDLED_DEV: "true" },
      ),
    ).toBe(true);

    expect(
      shouldUseBundledDev(
        { command: "build", mode: "development" },
        { BODYSENSE_VITE_BUNDLED_DEV: "true" },
      ),
    ).toBe(false);

    expect(
      shouldUseBundledDev(
        { command: "serve", mode: "production" },
        { BODYSENSE_VITE_BUNDLED_DEV: "true" },
      ),
    ).toBe(false);
  });

  it("keeps test and explicit fallback lanes on classic Vite", () => {
    expect(
      shouldUseBundledDev(
        { command: "serve", mode: "development" },
        {
          BODYSENSE_VITE_BUNDLED_DEV: "true",
          NODE_ENV: "test",
        },
      ),
    ).toBe(false);

    expect(
      shouldUseBundledDev(
        { command: "serve", mode: "development" },
        { BODYSENSE_VITE_BUNDLED_DEV: "false" },
      ),
    ).toBe(false);
  });

  it("skips Tailwind's classic-server hotUpdate helper only when bundled context has no server", () => {
    let calls = 0;
    const upstream: Plugin = {
      name: "@tailwindcss/vite:generate:serve",
      hotUpdate() {
        calls += 1;
        return [];
      },
    };

    const [adapted] = adaptTailwindPluginsForBundledDev([upstream], true);
    const hotUpdate = adapted.hotUpdate;
    expect(typeof hotUpdate).toBe("function");
    if (typeof hotUpdate !== "function") throw new Error("hotUpdate missing");
    const pluginContext = {} as ThisParameterType<typeof hotUpdate>;

    hotUpdate.call(pluginContext, {} as HotUpdateOptions);
    expect(calls).toBe(0);

    hotUpdate.call(
      pluginContext,
      { server: {} as HotUpdateOptions["server"] } as HotUpdateOptions,
    );
    expect(calls).toBe(1);
  });

  it("leaves classic Tailwind plugins untouched", () => {
    const upstream: Plugin = {
      name: "@tailwindcss/vite:generate:serve",
      hotUpdate() {
        return [];
      },
    };

    expect(adaptTailwindPluginsForBundledDev([upstream], false)[0]).toBe(
      upstream,
    );
  });
});
