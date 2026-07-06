import { node } from "@lfdt-lineth/eslint-config/node";

/** @type {import("eslint").Linter.Config[]} */
export default [
  {
    ignores: [
      ".solcover.js",
      "artifacts/**",
      "build/**",
      "cache/**",
      "docs/**",
      "generated/**",
      "integrity-verifier/**",
      "signer-ui/**",
      "typechain-types/**",
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
