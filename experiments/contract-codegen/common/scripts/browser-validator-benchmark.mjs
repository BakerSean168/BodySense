import fs from 'node:fs';
import path from 'node:path';
import { chromium } from 'playwright';

function resolveChromium() {
  const preferred = chromium.executablePath();
  if (fs.existsSync(preferred)) return preferred;
  const cache = '/home/dev/.cache/ms-playwright';
  if (fs.existsSync(cache)) {
    const matches = fs.readdirSync(cache)
      .filter(name => name.startsWith('chromium_headless_shell-'))
      .sort().reverse();
    for (const name of matches) {
      const candidate = path.join(cache, name, 'chrome-headless-shell-linux64', 'chrome-headless-shell');
      if (fs.existsSync(candidate)) return candidate;
    }
  }
  throw new Error('No Playwright Chromium/headless-shell executable is available');
}

const base = JSON.parse(fs.readFileSync('experiments/contract-codegen/common/fixtures/health-workspace.valid.json', 'utf8'));
const template = base.actions[0] ?? { kind: 'probe', priority: 1, enabled: true, reason: 'probe' };
function makePayload(target) {
  const value = structuredClone(base);
  value.actions = [];
  let i = 0;
  do {
    value.actions.push({ ...template, kind: `${template.kind}-${i}`, priority: i });
    i += 1;
  } while (Buffer.byteLength(JSON.stringify(value)) < target);
  return value;
}

const candidates = [
  ['orval_zod_mini_chromium', '/home/dev/projects/bodysense-contract-codegen-orval/experiments/contract-codegen/orval/metrics/browser/orval-mini.iife.js', 'OrvalValidator.parseHealthWorkspace'],
  ['heyapi_zod_chromium', '/home/dev/projects/bodysense-contract-codegen-heyapi/experiments/contract-codegen/heyapi/metrics/browser/hey-zod.iife.js', 'HeyValidator.validate'],
];
const browser = await chromium.launch({ headless: true, executablePath: resolveChromium() });
const page = await browser.newPage();
console.log('target_bytes\tactual_bytes\tvalidator\tops_s\tp50_ms\tp95_ms');
for (const [name, script, expr] of candidates) {
  await page.setContent('<!doctype html><html><body></body></html>');
  await page.addScriptTag({ path: script });
  for (const [target, iterations] of [[1000, 2000], [10000, 800], [100000, 120]]) {
    const payload = makePayload(target);
    const result = await page.evaluate(({ payload, iterations, expr }) => {
      const parts = expr.split('.');
      let fn = globalThis;
      for (const part of parts) fn = fn[part];
      for (let i = 0; i < 20; i++) fn(payload);
      const samples = [];
      const started = performance.now();
      for (let i = 0; i < iterations; i++) {
        const t = performance.now();
        fn(payload);
        samples.push(performance.now() - t);
      }
      const total = performance.now() - started;
      samples.sort((a,b)=>a-b);
      const at = p => samples[Math.min(samples.length - 1, Math.floor((samples.length - 1) * p))];
      return { ops: iterations / (total / 1000), p50: at(0.5), p95: at(0.95) };
    }, { payload, iterations, expr });
    console.log(`${target}\t${Buffer.byteLength(JSON.stringify(payload))}\t${name}\t${result.ops.toFixed(0)}\t${result.p50.toFixed(4)}\t${result.p95.toFixed(4)}`);
  }
}
await browser.close();
