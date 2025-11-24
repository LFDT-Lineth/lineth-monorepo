import { defineConfig } from "tsup";

export default defineConfig({
  entry: ["src/index.ts"],
  tsconfig: "tsconfig.build.json",
  format: ["esm", "cjs"],
  target: "esnext",
  dts: true,
  clean: true,
  sourcemap: true,
  minify: true,
  splitting: false,
  outDir: "dist",
  external: ["node-forge", "crypto", "viem"], // Avoid 'Error: Dynamic require of "crypto" is not supported' when used from native-yield-automation-service
});
