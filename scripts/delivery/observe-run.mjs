#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { createRunSummary } from './lib.mjs';

function readJson(file, fallback = {}) {
  if (!file) return fallback;
  return JSON.parse(fs.readFileSync(file, 'utf8'));
}

function arg(name) {
  const index = process.argv.indexOf(`--${name}`);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

try {
  const manifestFile = arg('manifest') ?? process.env.DELIVERY_MANIFEST_PATH;
  if (!manifestFile) throw new Error('--manifest is required');
  const output = path.resolve(
    arg('output') ?? process.env.DELIVERY_SUMMARY_OUTPUT ?? 'reports/delivery/delivery-run-summary-v1.json',
  );
  const summary = createRunSummary({
    manifest: readJson(manifestFile),
    laneResults: readJson(arg('lanes') ?? process.env.DELIVERY_LANE_RESULTS, {}),
    oracleResults: readJson(arg('oracles') ?? process.env.DELIVERY_ORACLE_RESULTS, {}),
  });
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, `${JSON.stringify(summary, null, 2)}\n`);
  console.log(`DELIVERY_OBSERVATION=PASS digest=${summary.digest}`);
} catch (error) {
  console.error(`delivery observation failed: ${error.message}`);
  process.exit(1);
}
