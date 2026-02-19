/** @type {import('jest').Config} */
export default {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  rootDir: ".",
  testMatch: ["**/__tests__/**/*.test.ts"],
  verbose: true,
  collectCoverage: true,
  collectCoverageFrom: ["src/**/*.ts"],
  coverageReporters: ["html", "lcov", "text"],
  testPathIgnorePatterns: ["src/run.ts", "src/core"],
  coveragePathIgnorePatterns: ["src/run.ts", "src/core", "src/application/main/LidoGovernanceMonitorBootstrap.ts"],
  extensionsToTreatAsEsm: [".ts"],
  transform: {
    "^.+\\.tsx?$": ["ts-jest", { useESM: true, tsconfig: "tsconfig.jest.json" }],
  },
  moduleNameMapper: {
    "^(\\.{1,2}/.*)\\.js$": "$1",
    "^@consensys/linea-shared-utils$": "<rootDir>/src/__mocks__/linea-shared-utils.ts",
    "^@anthropic-ai/sdk$": "<rootDir>/src/__mocks__/anthropic-sdk.ts",
  },
  transformIgnorePatterns: ["/node_modules/(?!(@anthropic-ai/sdk)/)"],
};
