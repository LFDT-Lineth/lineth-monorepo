module.exports = {
  preset: "ts-jest/presets/default-esm",
  testEnvironment: "node",
  testRegex: "test.ts$",
  transform: {
    "^.+\\.ts$": ["ts-jest", { useESM: true }],
  },
  verbose: true,
  collectCoverage: true,
  moduleNameMapper: {
    "^(\\.{1,2}/.*)\\.js$": "$1",
  },
  extensionsToTreatAsEsm: [".ts"],
  collectCoverageFrom: ["src/**/*.ts"],
  coverageReporters: ["html", "lcov", "text"],
  testPathIgnorePatterns: [],
  coveragePathIgnorePatterns: [],
};
