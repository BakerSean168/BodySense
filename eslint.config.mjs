import js from '@eslint/js';
import nx from '@nx/eslint-plugin';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  {
    ignores: [
      '**/dist/**',
      '**/node_modules/**',
      '**/.nx/**',
      '**/coverage/**',
      '**/generated/**',
      'tools/contracts/generated/**',
      '**/*.config.{js,mjs,cjs,ts,mts,cts}',
    ],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    plugins: {
      '@nx': nx,
    },
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],
      '@typescript-eslint/no-explicit-any': 'error',
    },
  },
  {
    files: [
      'apps/web/src/features/**/services/**/*.ts',
      'apps/web/src/features/workspace/api/**/*.ts',
      'apps/web/src/stores/*.ts',
      'apps/web/src/lib/clientDiagnostics.ts',
      'packages/contracts/src/**/*.ts',
    ],
    ignores: ['**/*.test.ts', '**/*.test.tsx'],
    rules: {
      '@typescript-eslint/no-unsafe-type-assertion': 'error',
    },
  },
  {
    files: ['**/*.tsx'],
    languageOptions: {
      globals: {
        console: 'readonly',
        document: 'readonly',
        EventSource: 'readonly',
        File: 'readonly',
        FormData: 'readonly',
        HTMLInputElement: 'readonly',
        HTMLTextAreaElement: 'readonly',
        confirm: 'readonly',
        localStorage: 'readonly',
        window: 'readonly',
      },
    },
  },
);
