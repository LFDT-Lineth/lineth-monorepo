import { node } from "@consensys/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: ["bin"],
  },
  ...node,
  {
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
];
