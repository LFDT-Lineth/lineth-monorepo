import { defineConfig } from "tsup";

export default defineConfig([
  // Library build (with .d.ts)
  {
    entry: ["src/index.ts", "src/tools.ts"],
    tsconfig: "tsconfig.build.json",
    format: ["esm", "cjs"],
    target: "esnext",
    dts: true,
    clean: true,
    sourcemap: true,
    minify: true,
    outDir: "dist",
  },
  // CLI builds (no .d.ts needed)
  {
    entry: ["src/cli.ts", "src/generate-schema-cli.ts"],
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
