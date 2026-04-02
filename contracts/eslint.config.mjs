import { node } from "@consensys/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: [
      ".solcover.js",
      "docs/**",
      "integrity-verifier/**",
      "signer-ui/.next/**",
      "signer-ui/node_modules/**",
    ],
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
  {
    files: ["test/**/*.ts"],
    rules: {
      "@typescript-eslint/no-unused-expressions": "off",
    },
  },
];
