import { node } from "@lfdt-lineth/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
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
