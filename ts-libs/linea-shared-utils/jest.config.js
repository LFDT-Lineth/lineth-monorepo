module.exports = {
  preset: "ts-jest",
  testEnvironment: "node",
  rootDir: ".",
  testRegex: ".test.ts$",
  verbose: true,
  collectCoverage: true,
  collectCoverageFrom: ["src/**/*.ts"],
  coverageReporters: ["html", "lcov", "text"],
  testPathIgnorePatterns: ["src/index.ts", "src/core"],
  coveragePathIgnorePatterns: ["src/index.ts", "src/core", "src/utils/file.ts", "src/__tests__/helpers"],
};
