import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const stylesDir = dirname(fileURLToPath(import.meta.url));
const manifest = resolve(stylesDir, "../body-canvas.css");

describe("BodyCanvas ordered CSS manifest", () => {
  it("preserves a single ordered stylesheet entry without duplicate imports", () => {
    const source = readFileSync(manifest, "utf8");
    const names = [...source.matchAll(/@import "\.\/styles\/([^"]+)";/g)].map(
      (match) => match[1],
    );
    expect(names).toHaveLength(10);
    expect(new Set(names).size).toBe(names.length);
    expect(names).toEqual(names.toSorted());
    for (const name of names) {
      expect(existsSync(resolve(stylesDir, name))).toBe(true);
    }
  });

  it("resolves every relative CSS asset URL from its new module directory", () => {
    const source = readFileSync(manifest, "utf8");
    const names = [...source.matchAll(/@import "\.\/styles\/([^"]+)";/g)].map(
      (match) => match[1],
    );
    const referenced = [];
    for (const name of names) {
      const cssFile = resolve(stylesDir, name);
      const css = readFileSync(cssFile, "utf8");
      for (const match of css.matchAll(
        /url\(["']?(\.{1,2}\/[^"')]+)["']?\)/g,
      )) {
        referenced.push(match[1]);
        expect(existsSync(resolve(dirname(cssFile), match[1]))).toBe(true);
      }
    }
    expect(referenced.length).toBeGreaterThan(0);
  });
});
