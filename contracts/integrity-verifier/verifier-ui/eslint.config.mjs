import { nextjs } from "@consensys/eslint-config/nextjs";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: ["next.config.mjs"],
  },
  ...nextjs,
  {
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
  {
    rules: {
      "@typescript-eslint/no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
    },
  },
];
