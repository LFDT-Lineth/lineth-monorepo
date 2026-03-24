module.exports = {
  collectCoverage: true,
  collectCoverageFrom: ["./src/**/*.ts"],
  coverageDirectory: "coverage",
  coverageProvider: "babel",
  coverageReporters: ["html", "json-summary", "text"],
  coverageThreshold: {
    global: {
      branches: 91.66,
      functions: 100,
      lines: 93.02,
      statements: 93.18,
    },
  },
  preset: "ts-jest",
  resetMocks: true,
  restoreMocks: true,
  testTimeout: 2500,
  testPathIgnorePatterns: ["src/scripts", "src/index.ts"],
  coveragePathIgnorePatterns: ["src/scripts", "src/index.ts"],
};
