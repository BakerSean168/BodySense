#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, "../..");
const outDir = path.join(root, "docs/refactor/vnext/baseline");

function read(rel) {
  return fs.readFileSync(path.join(root, rel), "utf8");
}

function walk(rel, predicate = () => true) {
  const base = path.join(root, rel);
  const out = [];
  for (const entry of fs.readdirSync(base, { withFileTypes: true })) {
    const childRel = path.posix.join(rel, entry.name);
    if (entry.isDirectory()) out.push(...walk(childRel, predicate));
    else if (predicate(childRel)) out.push(childRel);
  }
  return out.sort();
}

function git(...args) {
  return execFileSync("git", args, { cwd: root, encoding: "utf8" }).trim();
}

function joinRoute(prefix, route) {
  const value = `${prefix || ""}${route || ""}`.replace(/\/+/g, "/");
  return value || "/";
}

function parseGoRoutes() {
  const rel = "apps/api/cmd/server/main.go";
  const lines = read(rel).split("\n");
  const groups = new Map([["r", ""]]);
  const routes = [];
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const group = line.match(/^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)/);
    if (group) {
      const [, name, parent, suffix] = group;
      if (groups.has(parent))
        groups.set(name, joinRoute(groups.get(parent), suffix));
    }
    const route = line.match(
      /^\s*(\w+)\.(GET|POST|PUT|PATCH|DELETE)\("([^"]*)"/,
    );
    if (route) {
      const [, owner, method, suffix] = route;
      if (!groups.has(owner)) continue;
      routes.push({
        method,
        path: joinRoute(groups.get(owner), suffix),
        source: `${rel}:${i + 1}`,
      });
    }
  }
  return routes;
}

function parseFastApiRoutes() {
  const routes = [];
  const routeFiles = walk("apps/ai-service/src/api/routes", (rel) =>
    rel.endsWith(".py"),
  );
  for (const rel of routeFiles) {
    const text = read(rel);
    const prefix = text.match(/APIRouter\(prefix="([^"]*)"/)?.[1] ?? "";
    const lines = text.split("\n");
    for (let i = 0; i < lines.length; i += 1) {
      const route = lines[i].match(
        /^@router\.(get|post|put|patch|delete)\("([^"]*)"/i,
      );
      if (route) {
        routes.push({
          method: route[1].toUpperCase(),
          path: joinRoute(prefix, route[2]),
          source: `${rel}:${i + 1}`,
        });
      }
    }
  }
  const mainLines = read("apps/ai-service/src/main.py").split("\n");
  for (let i = 0; i < mainLines.length; i += 1) {
    const route = mainLines[i].match(
      /^@app\.(get|post|put|patch|delete)\("([^"]*)"/i,
    );
    if (route) {
      routes.push({
        method: route[1].toUpperCase(),
        path: route[2],
        source: `apps/ai-service/src/main.py:${i + 1}`,
      });
    }
  }
  return routes.sort(
    (a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method),
  );
}

function parsePublicStreamEvents() {
  const rel = "packages/contracts/src/stream-events.ts";
  const text = read(rel);
  const re = /StreamEventBase<\s*"([^"]+)",\s*"([^"]+)"/g;
  const seen = new Set();
  const events = [];
  for (const match of text.matchAll(re)) {
    const key = `${match[1]}:${match[2]}`;
    if (seen.has(key)) continue;
    seen.add(key);
    events.push({ channel: match[1], type: match[2] });
  }
  return events;
}

function parseMigrations() {
  const files = fs
    .readdirSync(path.join(root, "apps/api/migrations"))
    .filter((name) => /^\d+_.+\.up\.sql$/.test(name))
    .sort();
  const versions = files.map((name) => Number(name.match(/^(\d+)_/)?.[1]));
  return {
    count: files.length,
    latestVersion: Math.max(...versions),
    files,
  };
}

function lineMatches(rel, regex) {
  const matches = [];
  for (const [index, line] of read(rel).split("\n").entries()) {
    if (regex.test(line))
      matches.push({
        source: `${rel}:${index + 1}`,
        text: line.trim().slice(0, 220),
      });
    regex.lastIndex = 0;
  }
  return matches;
}

function collectHotspot(regex, files) {
  const hits = [];
  for (const rel of files) hits.push(...lineMatches(rel, regex));
  const byFile = new Map();
  for (const hit of hits) {
    const file = hit.source.replace(/:\d+$/, "");
    byFile.set(file, (byFile.get(file) ?? 0) + 1);
  }
  return {
    count: hits.length,
    topFiles: [...byFile.entries()]
      .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
      .slice(0, 20)
      .map(([file, count]) => ({ file, count })),
  };
}

function collectEnvNames(files) {
  const names = new Set();
  const patterns = [
    /(?:os\.getenv|os\.environ\.get)\(["']([A-Z][A-Z0-9_]*)["']/g,
    /os\.environ\[["']([A-Z][A-Z0-9_]*)["']\]/g,
    /(?:os\.Getenv|os\.LookupEnv)\("([A-Z][A-Z0-9_]*)"/g,
    /(?:process\.env|import\.meta\.env)\.([A-Z][A-Z0-9_]*)/g,
  ];
  for (const rel of files) {
    const text = read(rel);
    for (const regex of patterns) {
      for (const match of text.matchAll(regex)) names.add(match[1]);
    }
  }
  return [...names].sort();
}

const sourceFiles = [
  ...walk(
    "apps",
    (rel) =>
      /\.(go|py|ts|tsx)$/.test(rel) &&
      !rel.includes("/generated/") &&
      !/(_test\.go|\/tests\/|\.test\.(ts|tsx)$)/.test(rel),
  ),
  ...walk(
    "packages",
    (rel) =>
      /\.(ts|tsx)$/.test(rel) &&
      !rel.includes("/generated/") &&
      !/\.test\.(ts|tsx)$/.test(rel),
  ),
];
const tsFiles = sourceFiles.filter((rel) => /\.(ts|tsx)$/.test(rel));
const pyFiles = sourceFiles.filter((rel) => rel.endsWith(".py"));
const goFiles = sourceFiles.filter((rel) => rel.endsWith(".go"));

const agentConfigs = fs
  .readdirSync(path.join(root, "apps/ai-service/config/agents"))
  .filter((name) => name.endsWith(".yaml"))
  .sort();

const legacyRuntime = collectHotspot(
  /\b(legacy|compatibility|deprecated)\b/i,
  sourceFiles,
);
const inventory = {
  schemaVersion: 1,
  capturedFrom: {
    commit: git("rev-parse", "HEAD"),
    branch: git("branch", "--show-current"),
  },
  publicApi: {
    goRoutes: parseGoRoutes(),
  },
  internalAiApi: {
    fastApiRoutes: parseFastApiRoutes(),
  },
  publicStream: {
    source: "packages/contracts/src/stream-events.ts",
    events: parsePublicStreamEvents(),
  },
  databaseMigrationLineage: parseMigrations(),
  agentConfigurations: agentConfigs,
  environmentVariables: collectEnvNames(sourceFiles),
  codeQualityHotspots: {
    typescriptDoubleAssertions: collectHotspot(
      /\bas\s+unknown\s+as\b/,
      tsFiles,
    ),
    typescriptExplicitAny: collectHotspot(/\bany\b/, tsFiles),
    pythonAny: collectHotspot(/\bAny\b/, pyFiles),
    pythonBroadException: collectHotspot(/\bexcept\s+Exception\b/, pyFiles),
    goAny: collectHotspot(/\bany\b/, goFiles),
    runtimeLegacyCompatibilityMarkers: legacyRuntime,
  },
};

const output = `${JSON.stringify(inventory, null, 2)}\n`;
fs.mkdirSync(outDir, { recursive: true });
fs.writeFileSync(path.join(outDir, "current-system-inventory.json"), output);

const summary =
  `# BodySense vNext Phase 00 — Current-System Baseline\n\n` +
  `> Generated by \`pnpm refactor:vnext:baseline\`. Do not hand-edit counts in this file; update the capture script or source and regenerate.\n\n` +
  `- Commit: \`${inventory.capturedFrom.commit}\`\n` +
  `- Branch: \`${inventory.capturedFrom.branch}\`\n` +
  `- Public Go routes: **${inventory.publicApi.goRoutes.length}**\n` +
  `- Internal FastAPI routes: **${inventory.internalAiApi.fastApiRoutes.length}**\n` +
  `- Public StreamEvent variants: **${inventory.publicStream.events.length}**\n` +
  `- Active migration up-files: **${inventory.databaseMigrationLineage.count}** (latest version **${inventory.databaseMigrationLineage.latestVersion}**)\n` +
  `- Agent configuration manifests: **${inventory.agentConfigurations.length}**\n` +
  `- Referenced application environment variables: **${inventory.environmentVariables.length}**\n\n` +
  `## Code-quality hotspot counts\n\n` +
  `| Class | Baseline count |\n|---|---:|\n` +
  `| TypeScript \`as unknown as\` | ${inventory.codeQualityHotspots.typescriptDoubleAssertions.count} |\n` +
  `| TypeScript \`any\` token (broad inventory signal, not all are defects) | ${inventory.codeQualityHotspots.typescriptExplicitAny.count} |\n` +
  `| Python \`Any\` token (broad inventory signal, not all are defects) | ${inventory.codeQualityHotspots.pythonAny.count} |\n` +
  `| Python broad \`except Exception\` | ${inventory.codeQualityHotspots.pythonBroadException.count} |\n` +
  `| Go \`any\` token (broad inventory signal, not all are defects) | ${inventory.codeQualityHotspots.goAny.count} |\n` +
  `| Runtime legacy/compatibility/deprecated markers | ${inventory.codeQualityHotspots.runtimeLegacyCompatibilityMarkers.count} |\n\n` +
  `These are discovery signals, not success metrics. Phase 05/07 review each occurrence at the relevant trust/runtime boundary rather than chasing zero mechanically.\n`;
fs.writeFileSync(path.join(outDir, "current-system-baseline.md"), summary);
