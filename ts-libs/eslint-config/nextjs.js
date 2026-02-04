import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";

import node from "./node.js";

export const nextjs = defineConfig([
  globalIgnores([
    ".cache-synpress",
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    "postcss.config.mjs",
  ]),
  ...nextVitals,
  ...node,
]);

export default nextjs;
