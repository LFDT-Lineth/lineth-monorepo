import { node } from "@lfdt-lineth/eslint-config/node";

export default [
  {
    ignores: ["dist/**"],
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
