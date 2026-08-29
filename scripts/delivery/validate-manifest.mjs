#!/usr/bin/env node
import fs from 'node:fs';
import { validateManifest } from './lib.mjs';

const file = process.argv[2] ?? process.env.DELIVERY_MANIFEST_PATH;
if (!file) {
  console.error('usage: validate-manifest.mjs <manifest.json>');
  process.exit(2);
}

try {
  const manifest = JSON.parse(fs.readFileSync(file, 'utf8'));
  const errors = validateManifest(manifest);
  if (errors.length > 0) {
    console.error(errors.join('\n'));
    process.exit(1);
  }
  console.log(`DELIVERY_MANIFEST=PASS digest=${manifest.digest} risk=${manifest.risk}`);
} catch (error) {
  console.error(`delivery manifest validation failed: ${error.message}`);
  process.exit(1);
}
