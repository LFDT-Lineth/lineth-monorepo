import { defineConfig } from "tsup";

export default defineConfig([
  // Browser-safe build (no fs/path dependencies)
  {
    entry: {
      browser: "src/browser.ts",
      adapter: "src/adapter.ts",
    },
    tsconfig: "tsconfig.build.json",
    format: ["esm", "cjs"],
    target: "esnext",
    dts: true,
    clean: true,
    sourcemap: true,
    minify: true,
    outDir: "dist",
    // Mark Node.js built-ins as external to prevent bundling
    external: ["fs", "path"],
    // Don't split - creates self-contained bundles
    splitting: false,
    // Prevent importing node-only modules
    noExternal: [],
  },
  // Node.js build (includes fs/path dependencies)
  {
    entry: {
      index: "src/index.ts",
      "tools/index": "src/tools/index.ts",
    },
    tsconfig: "tsconfig.build.json",
    format: ["esm", "cjs"],
    target: "esnext",
    dts: true,
    clean: false, // Don't clean, we need browser files
    sourcemap: true,
    minify: true,
    outDir: "dist",
  },
]);
