import { node } from "@consensys/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: [".solcover.js", "docs/**", "integrity-verifier/**"],
  },
  ...node,
  {
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // TODO: this plugin is disabled for now to avoid a lot of files changes
      "import/order": "off",
    }
  },
  {
    files: ["test/**/*.ts"],
    rules: {
      "@typescript-eslint/no-unused-expressions": "off",
    },
  },
];
