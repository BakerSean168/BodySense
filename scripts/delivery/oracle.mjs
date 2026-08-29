#!/usr/bin/env node
import fs from 'node:fs';
import { evaluateOracle } from './lib.mjs';

function readInput() {
  const fileIndex = process.argv.indexOf('--input');
  if (fileIndex >= 0) {
    const file = process.argv[fileIndex + 1];
    if (!file) throw new Error('--input requires a file path');
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  }
  if (process.env.ORACLE_INPUT) return JSON.parse(process.env.ORACLE_INPUT);
  throw new Error('provide --input <json-file> or ORACLE_INPUT');
}

try {
  const input = readInput();
  const result = evaluateOracle(input);
  if (!result.ok) {
    for (const failure of result.failures) console.error(failure);
    process.exit(1);
  }
  console.log('DELIVERY_ORACLE=PASS');
} catch (error) {
  console.error(`delivery oracle failed: ${error.message}`);
  process.exit(1);
}
