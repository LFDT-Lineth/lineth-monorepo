import { defineConfig, globalIgnores } from "eslint/config";
import globals from "globals";

import base from "./base.js";

/**
 * Node.js backend ESLint flat config.
 * Includes Node.js globals and server-specific rules.
 * Consuming apps must configure their own parserOptions.project.
 */
export const node = defineConfig([
  globalIgnores([
    "**/dist",
    "**/node_modules",
    "**/coverage",
    "**/build",
    "**/cache",
    "**/typechain",
    "**/typechain-types",
    "**/jest.config.{js,ts,cjs,mjs}",
    "**/tsup.config.{js,ts}",
    "**/eslint.config.mjs",
    "**/prettier.config.mjs"
  ]),
  {
    extends: [base],
    files: ["**/*.ts", "**/*.cts", "**/*.mts", "**/*.js", "**/*.cjs", "**/*.mjs", "**/*.tsx", "**/*.jsx"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        ...globals.node,
        ...globals.es2022,
      },
    },
    rules: {
      "no-constant-condition": ["error", { "checkLoops": "all" }],
    }
  },
  {
    files: ["**/*.test.ts", "**/*.spec.ts", "**/__tests__/**/*.ts"],
    rules: {
      "@typescript-eslint/unbound-method": "off",
      // TODO: refine the following rules
      "@typescript-eslint/no-unused-vars": "off",
      "@typescript-eslint/no-explicit-any": "off"
    },
  },
]);

export default node;
