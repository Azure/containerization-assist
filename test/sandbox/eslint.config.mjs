// eslint.config.js
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import eslintPluginSimpleImportSort from 'eslint-plugin-simple-import-sort';
import eslintConfigPrettier from 'eslint-config-prettier';
import eslintPluginPrettier from 'eslint-plugin-prettier';

export default [
    js.configs.recommended,
    ...tseslint.configs.recommended,
    {
        files: ['**/*.ts', '**/*.tsx'],
        languageOptions: {
            parser: tseslint.parser,
            parserOptions: {
                project: './tsconfig.json',
            },
        },
        plugins: {
            'simple-import-sort': eslintPluginSimpleImportSort,
            prettier: eslintPluginPrettier,
        },
        rules: {
            '@typescript-eslint/no-unused-vars': 'error',
        },
    },
    {
        ignores: ['dist/', 'node_modules/'],
    },
    eslintConfigPrettier,
];
