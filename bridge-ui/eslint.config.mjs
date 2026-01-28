import { nextjs } from "@consensys/eslint-config/nextjs";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: ["test/advancedFixtures.ts", "playwright-report/**"]
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
];
