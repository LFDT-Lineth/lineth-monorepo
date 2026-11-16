import { node } from "@lfdt-lineth/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: [
      "docs/**",
      "integrity-verifier/**",
      // Own workspace package with its own tsconfig and ESLint config (`pnpm --filter @lfdt-lineth/contract-signer-ui lint`).
      "signer-ui/**",
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
