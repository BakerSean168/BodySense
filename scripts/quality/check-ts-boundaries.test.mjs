import assert from 'node:assert/strict';
import test from 'node:test';
import { analyzeTypeScriptSource } from './check-ts-boundaries.mjs';

test('generated browser API is allowed in service boundary adapters', () => {
  const violations = analyzeTypeScriptSource(
    'apps/web/src/features/profile/services/example.ts',
    'import { getUserProfile } from "@/generated/api/bodysense";\nexport { getUserProfile };\n',
  );
  assert.deepEqual(violations, []);
});

test('generated browser API fails when it leaks into a UI component', () => {
  const violations = analyzeTypeScriptSource(
    'apps/web/src/features/profile/components/LeakyComponent.tsx',
    'import { getUserProfile } from "@/generated/api/bodysense";\nexport const LeakyComponent = () => null;\n',
  );
  assert.equal(violations.length, 1);
  assert.match(violations[0], /generated browser API leaked outside an approved trust-boundary adapter/);
});

test('retired generic runtime authority cannot reappear', () => {
  const violations = analyzeTypeScriptSource(
    'apps/web/src/features/consultation/runtime/leaky.ts',
    'const consultationInternalEventChannels = [];\nexport { consultationInternalEventChannels };\n',
  );
  assert.ok(violations.length >= 1);
  assert.match(violations[0], /retired generic internal runtime authority resurfaced/);
});

test('non-generated imports remain unrestricted by the generated-boundary policy', () => {
  const violations = analyzeTypeScriptSource(
    'apps/web/src/features/profile/components/ProfileCard.tsx',
    'import { useMemo } from "react";\nexport const ProfileCard = () => useMemo(() => null, []);\n',
  );
  assert.deepEqual(violations, []);
});
