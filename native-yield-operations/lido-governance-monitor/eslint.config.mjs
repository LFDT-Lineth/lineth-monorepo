import { node } from "@consensys/eslint-config/node";

export default [
  ...node,
  {
    ignores: ["prisma/client/**", "prisma.config.ts", "scripts/**"],
  },
  {
    languageOptions: {
      parserOptions: {
        project: "./tsconfig.json",
        tsconfigRootDir: import.meta.dirname,
      },
    },
  },
];
