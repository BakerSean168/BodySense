#!/usr/bin/env node
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import YAML from "yaml";

const root = process.cwd();
const specPath = path.join(
  root,
  "packages/contracts/openapi/bodysense.v1.openapi.yaml",
);
const collectionPath = path.join(
  root,
  "postman/collections/BodySense Public API.postman_collection.json",
);
const environmentsDir = path.join(root, "postman/environments");
const checkMode = process.argv.includes("--check");
const HTTP_METHODS = new Set([
  "get",
  "post",
  "put",
  "patch",
  "delete",
  "head",
  "options",
]);
const FIXED_CLOCK_MS = Date.parse("2026-01-01T00:00:00.000Z");
const RANDOM_SEED = 0x42534e53;

function stableUuid(input) {
  const bytes = crypto
    .createHash("sha256")
    .update(`bodysense-postman-v1\0${input}`)
    .digest()
    .subarray(0, 16);
  bytes[6] = (bytes[6] & 0x0f) | 0x50;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = bytes.toString("hex");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function installDeterministicGeneratorRuntime() {
  let state = RANDOM_SEED >>> 0;
  Math.random = () => {
    state = (1664525 * state + 1013904223) >>> 0;
    return state / 0x1_0000_0000;
  };

  const RealDate = Date;
  class DeterministicDate extends RealDate {
    constructor(...args) {
      super(...(args.length ? args : [FIXED_CLOCK_MS]));
    }

    static now() {
      return FIXED_CLOCK_MS;
    }
  }
  globalThis.Date = DeterministicDate;
}

function normalizedRoutePath(routePath) {
  return (
    routePath
      .replace(/^\{\{baseUrl\}\}/, "")
      .split("?", 1)[0]
      .replace(/:[^/]+/g, ":*")
      .replace(/\{[^}]+\}/g, ":*")
      .replace(/\/$/, "") || "/"
  );
}

function requestRawUrl(request) {
  if (typeof request?.url === "string") return request.url;
  if (typeof request?.url?.raw === "string") return request.url.raw;
  if (Array.isArray(request?.url?.path))
    return `/${request.url.path.join("/")}`;
  throw new Error("generated Postman request is missing a usable URL");
}

function stabilizeCollectionIds(collection) {
  collection.info._postman_id = stableUuid("collection:public-api");

  function visit(items, parentKey) {
    for (const item of items ?? []) {
      if (item.request) {
        const method = String(item.request.method ?? "GET").toUpperCase();
        const routePath = normalizedRoutePath(requestRawUrl(item.request));
        const requestKey = `request:${method}:${routePath}`;
        item.id = stableUuid(requestKey);
        for (let index = 0; index < (item.response ?? []).length; index += 1) {
          const response = item.response[index];
          const responseKey = [
            requestKey,
            "response",
            String(response.code ?? "unknown"),
            String(response.name ?? "unnamed"),
            String(index),
          ].join(":");
          response.id = stableUuid(responseKey);
        }
        continue;
      }

      const folderKey = `folder:${parentKey}/${String(item.name ?? "unnamed")}`;
      item.id = stableUuid(folderKey);
      visit(item.item, folderKey);
    }
  }

  visit(collection.item, "root");
}

function setCollectionVariables(collection) {
  const variables = new Map(
    (collection.variable ?? []).map((entry) => [entry.key, entry]),
  );
  variables.set("baseUrl", {
    ...(variables.get("baseUrl") ?? {}),
    key: "baseUrl",
    value: "http://127.0.0.1:20101",
    type: "string",
  });
  variables.set("bearerToken", {
    ...(variables.get("bearerToken") ?? {}),
    key: "bearerToken",
    value: "",
    type: "string",
  });
  collection.variable = [...variables.values()];
}

function openApiOperations(spec) {
  const operations = new Map();
  for (const [routePath, pathItem] of Object.entries(spec.paths ?? {})) {
    for (const [method, operation] of Object.entries(pathItem ?? {})) {
      if (!HTTP_METHODS.has(method.toLowerCase())) continue;
      const key = `${method.toUpperCase()} ${normalizedRoutePath(routePath)}`;
      if (operations.has(key)) {
        throw new Error(
          `duplicate OpenAPI operation after normalization: ${key}`,
        );
      }
      operations.set(key, {
        method: method.toUpperCase(),
        path: routePath,
        operationId: operation?.operationId ?? null,
      });
    }
  }
  return operations;
}

function collectionOperations(collection) {
  const operations = new Map();
  function visit(items) {
    for (const item of items ?? []) {
      if (item.request) {
        const method = String(item.request.method ?? "GET").toUpperCase();
        const routePath = normalizedRoutePath(requestRawUrl(item.request));
        const key = `${method} ${routePath}`;
        if (operations.has(key))
          throw new Error(`duplicate generated Postman operation: ${key}`);
        operations.set(key, {
          method,
          path: routePath,
          id: item.id,
          name: item.name,
        });
      }
      visit(item.item);
    }
  }
  visit(collection.item);
  return operations;
}

function assertRouteParity(spec, collection) {
  const canonical = openApiOperations(spec);
  const generated = collectionOperations(collection);
  const missing = [...canonical.keys()].filter((key) => !generated.has(key));
  const extra = [...generated.keys()].filter((key) => !canonical.has(key));
  if (missing.length || extra.length) {
    throw new Error(
      `Postman/OpenAPI route parity failed: missing=${JSON.stringify(missing)}, extra=${JSON.stringify(extra)}`,
    );
  }
  return canonical.size;
}

function environment(name, baseUrl) {
  return {
    id: stableUuid(`environment:${name.toLowerCase()}`),
    name: `BodySense ${name}`,
    values: [
      { key: "baseUrl", value: baseUrl, type: "default", enabled: true },
      { key: "bearerToken", value: "", type: "secret", enabled: true },
    ],
    _postman_variable_scope: "environment",
  };
}

function stringify(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

function assertOrWrite(filePath, content) {
  if (checkMode) {
    if (
      !fs.existsSync(filePath) ||
      fs.readFileSync(filePath, "utf8") !== content
    ) {
      throw new Error(
        `stale generated Postman asset: ${path.relative(root, filePath)}`,
      );
    }
    return;
  }
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, content);
}

installDeterministicGeneratorRuntime();
const converterModule = await import("openapi-to-postmanv2");
const converter = converterModule.default ?? converterModule;
const source = fs.readFileSync(specPath, "utf8");
const spec = YAML.parse(source);
const options = {
  ...converter.getOptions("use"),
  folderStrategy: "Paths",
  requestNameSource: "Fallback",
  parametersResolution: "Schema",
  schemaFaker: true,
  includeAuthInfoInExample: true,
};

const conversion = await new Promise((resolve, reject) => {
  converter.convert(
    { type: "string", data: source },
    options,
    (error, result) => {
      if (error) return reject(error);
      if (!result?.result || !result.output?.[0]?.data) {
        return reject(
          new Error(
            `OpenAPI -> Postman conversion failed: ${result?.reason ?? "unknown error"}`,
          ),
        );
      }
      resolve(result.output[0].data);
    },
  );
});

conversion.info.name = "BodySense Public API";
conversion.info.description = [
  spec.info?.description ?? "BodySense public browser REST API.",
  "GENERATED from packages/contracts/openapi/bodysense.v1.openapi.yaml. Do not hand-edit this collection.",
  "Python AI-service and Agent runtime HTTP routes are internal service boundaries and intentionally excluded.",
].join("\n\n");
stabilizeCollectionIds(conversion);
setCollectionVariables(conversion);
const routeCount = assertRouteParity(spec, conversion);

assertOrWrite(collectionPath, stringify(conversion));
assertOrWrite(
  path.join(environmentsDir, "BodySense Dev.environment.json"),
  stringify(environment("Dev", "http://127.0.0.1:20101")),
);
assertOrWrite(
  path.join(environmentsDir, "BodySense Staging.environment.json"),
  stringify(environment("Staging", "")),
);
assertOrWrite(
  path.join(environmentsDir, "BodySense Production.environment.json"),
  stringify(environment("Production", "https://body.bakersean.top")),
);

console.log(
  `Postman ${checkMode ? "verification" : "generation"}: PASS routes=${routeCount} authority=${path.relative(root, specPath)}`,
);
