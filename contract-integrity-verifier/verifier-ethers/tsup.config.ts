import { defineConfig } from "tsup";

export default defineConfig([
  // Library build (with .d.ts)
  {
    entry: ["src/index.ts"],
    tsconfig: "tsconfig.build.json",
    format: ["esm", "cjs"],
    target: "esnext",
    dts: true,
    clean: true,
    sourcemap: true,
    minify: true,
    outDir: "dist",
  },
  // CLI build (no .d.ts needed)
  {
    entry: ["src/cli.ts"],
    tsconfig: "tsconfig.build.json",
    format: ["esm"],
    target: "esnext",
    dts: false,
    clean: false,
    sourcemap: true,
    minify: false,
    outDir: "dist",
  },
]);
