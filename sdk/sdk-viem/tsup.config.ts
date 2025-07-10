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
  outDir: "dist",
});
