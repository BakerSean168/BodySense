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

const fixtures = JSON.parse(fs.readFileSync('packages/contracts/fixtures/stream-events.v1.json','utf8'));
const script = '/home/dev/projects/bodysense-contract-codegen-jsonschema/experiments/contract-codegen/jsonschema/metrics/browser/j1-ajv.iife.js';
const exe = resolveChromium();
const browser = await chromium.launch({headless:true, executablePath:exe});
const page = await browser.newPage();
await page.setContent('<!doctype html><html><body></body></html>');
await page.addScriptTag({path:script});
console.log('event_count\tvalidator\tmedian_total_ms\tevents_per_sec');
for (const count of [100,1000,10000]) {
  const events = Array.from({length:count}, (_,i)=>structuredClone(fixtures[i % fixtures.length]));
  const r = await page.evaluate(({events}) => {
    const samples=[];
    const fn=globalThis.J1Validator.validateStreamEvent;
    for(let warm=0; warm<3; warm++) for(const e of events) fn(e);
    for(let round=0; round<7; round++) {
      const t=performance.now();
      for(const e of events) fn(e);
      samples.push(performance.now()-t);
    }
    samples.sort((a,b)=>a-b);
    const median=samples[Math.floor(samples.length/2)];
    return {median, eps: events.length/(median/1000)};
  }, {events});
  console.log(`${count}\tj1_ajv_chromium\t${r.median.toFixed(3)}\t${r.eps.toFixed(0)}`);
}
await browser.close();
