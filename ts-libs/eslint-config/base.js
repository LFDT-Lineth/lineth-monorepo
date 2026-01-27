import eslint from "@eslint/js";
import { defineConfig } from "eslint/config";

import tseslint from "typescript-eslint";
import eslintConfigPrettier from "eslint-config-prettier";
import importPlugin from "eslint-plugin-import";
import prettierPlugin from "eslint-plugin-prettier";

/**
 * Base ESLint flat config for all TypeScript projects.
 * Does NOT include environment-specific globals or parserOptions.project.
 * Consuming apps must configure their own TypeScript project context.
 */
export const base = defineConfig(
  {
    extends: [eslint.configs.recommended, tseslint.configs.recommended, eslintConfigPrettier],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
    },
    plugins: {
      import: importPlugin,
      prettier: prettierPlugin
    },
    rules: {
        "prettier/prettier": "error",
        "import/order": [
          "error",
          {
            groups: [
              ["builtin", "external"],
              ["internal"],
              ["parent", "sibling", "index"],
              ["type"],
            ],
            pathGroups: [
              {
                pattern: "react",
                group: "external",
                position: "before", 
              },
              {
                pattern: "@/**",
                group: "internal",
              },
            ],
            pathGroupsExcludedImportTypes: ["react"],
            "newlines-between": "always",
            alphabetize: { order: "asc", caseInsensitive: true },
          },
        ]
    },
  },
);

export default base;
