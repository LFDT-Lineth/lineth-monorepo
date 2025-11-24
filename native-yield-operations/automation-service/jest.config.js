// jest.config.mjs
/** @type {import('jest').Config} */
export default {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  rootDir: ".",

  // Prefer testMatch over a bare regex
  testMatch: ["**/__tests__/**/*.test.ts"],

  verbose: true,
  collectCoverage: true,
  collectCoverageFrom: ["src/**/*.ts"],
  coverageReporters: ["html", "lcov", "text"],
  testPathIgnorePatterns: ["src/run.ts", "src/utils/createApolloClient.ts", "src/core"],
  coveragePathIgnorePatterns: ["src/run.ts", "src/utils/createApolloClient.ts", "src/core"],

  // Tell Jest that .ts files are ESM and have ts-jest emit ESM
  extensionsToTreatAsEsm: [".ts"],
  transform: {
    "^.+\\.tsx?$": ["ts-jest", { useESM: true, tsconfig: "tsconfig.jest.json" }],
  },

  // If your source imports have ".js" suffix (ESM style), rewrite for TS
  moduleNameMapper: {
    "^(\\.{1,2}/.*)\\.js$": "$1",
  },

  // Only add node_modules here if you hit ESM-only deps later
  transformIgnorePatterns: ["/node_modules/(?!(@lidofinance/lsv-cli|some-esm-only-lib)/)"],
};
