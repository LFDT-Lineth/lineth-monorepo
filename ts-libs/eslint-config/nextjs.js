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
  {
    rules: {
       // TODO: remove this rules after fix bridge ui typing
      "@typescript-eslint/no-explicit-any": "off",
      "react-hooks/refs": "off",
      "react-hooks/set-state-in-effect": "off",
      "react-hooks/immutability": "off"
    },
  },
]);

export default nextjs;
