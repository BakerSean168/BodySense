#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import ts from 'typescript';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, '../..');
const generatedApiPrefix = '@/generated/api';
const retiredRuntimeIdentifiers = new Set([
  'consultationInternalEventChannels',
  'validateConsultationInternalEvent',
]);

function normalize(rel) {
  return rel.split(path.sep).join('/');
}

export function isAllowedGeneratedApiConsumer(relPath) {
  const rel = normalize(relPath);
  return (
    /^apps\/web\/src\/features\/[^/]+\/services\//.test(rel) ||
    rel.startsWith('apps/web/src/features/workspace/api/') ||
    rel.startsWith('apps/web/src/stores/') ||
    rel === 'apps/web/src/lib/clientDiagnostics.ts' ||
    rel === 'apps/web/src/features/consultation/runtime/pendingInteractionProjection.ts'
  );
}

function moduleSpecifier(node) {
  if (
    (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
    node.moduleSpecifier &&
    ts.isStringLiteralLike(node.moduleSpecifier)
  ) {
    return node.moduleSpecifier.text;
  }
  if (
    ts.isCallExpression(node) &&
    node.expression.kind === ts.SyntaxKind.ImportKeyword &&
    node.arguments.length === 1 &&
    ts.isStringLiteralLike(node.arguments[0])
  ) {
    return node.arguments[0].text;
  }
  return null;
}

export function analyzeTypeScriptSource(relPath, source) {
  const rel = normalize(relPath);
  if (rel.startsWith('apps/web/src/generated/')) return [];

  const kind = rel.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
  const sourceFile = ts.createSourceFile(rel, source, ts.ScriptTarget.Latest, true, kind);
  const violations = [];

  function visit(node) {
    if (ts.isIdentifier(node) && retiredRuntimeIdentifiers.has(node.text)) {
      const pos = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      violations.push(
        `${rel}:${pos.line + 1}:${pos.character + 1}: retired generic internal runtime authority resurfaced (${node.text})`,
      );
    }
    const specifier = moduleSpecifier(node);
    if (specifier?.startsWith(generatedApiPrefix) && !isAllowedGeneratedApiConsumer(rel)) {
      const pos = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      violations.push(
        `${rel}:${pos.line + 1}:${pos.character + 1}: generated browser API leaked outside an approved trust-boundary adapter (${specifier})`,
      );
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return violations;
}

function walk(dir, files = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    if (['node_modules', 'dist', 'generated'].includes(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(full, files);
    } else if (/\.tsx?$/.test(entry.name)) {
      files.push(full);
    }
  }
  return files;
}

export function scanTypeScriptBoundaries(root = repoRoot) {
  const webRoot = path.join(root, 'apps/web/src');
  const violations = [];
  for (const full of walk(webRoot)) {
    const rel = normalize(path.relative(root, full));
    violations.push(...analyzeTypeScriptSource(rel, fs.readFileSync(full, 'utf8')));
  }
  return violations;
}

const invoked = process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href;
if (invoked) {
  const rootArgIndex = process.argv.indexOf('--root');
  const scanRoot =
    rootArgIndex >= 0 && process.argv[rootArgIndex + 1]
      ? path.resolve(process.argv[rootArgIndex + 1])
      : repoRoot;
  const violations = scanTypeScriptBoundaries(scanRoot);
  if (violations.length) {
    console.error(`TypeScript architecture violations:\n${violations.join('\n')}`);
    process.exit(1);
  }
  console.log('TS_GENERATED_BOUNDARY=PASS');
}
