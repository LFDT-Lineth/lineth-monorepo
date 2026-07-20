import * as assert from "node:assert/strict";

import {
  envNumber,
  envValue,
  parseBoolean,
  parseDecimalWei,
  readDotEnvContents,
  readDotEnvFile,
  requiredEnvValue,
  requiredProcessEnv,
} from "./env";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const tests: TestCase[] = [
  {
    name: "envValue returns fallback for missing and empty values",
    run: () => {
      assert.equal(envValue("A", { A: "x" }), "x");
      assert.equal(envValue("A", { A: "" }, "fallback"), "fallback");
      assert.equal(envValue("A", {}, "fallback"), "fallback");
    },
  },
  {
    name: "requiredEnvValue throws the .env wording when unset",
    run: () => {
      assert.equal(requiredEnvValue("A", { A: "x" }), "x");
      assert.throws(() => requiredEnvValue("A", {}), /A must be set in \.env/);
    },
  },
  {
    name: "requiredProcessEnv reads process.env and throws when unset",
    run: () => {
      const key = "LINETH_LIB_ENV_TEST_VALUE";
      delete process.env[key];
      assert.throws(() => requiredProcessEnv(key), new RegExp(`${key} must be set`));
      process.env[key] = "";
      assert.throws(() => requiredProcessEnv(key), new RegExp(`${key} must be set`));
      process.env[key] = "present";
      assert.equal(requiredProcessEnv(key), "present");
      delete process.env[key];
    },
  },
  {
    name: "envNumber validates integers and falls back",
    run: () => {
      assert.equal(envNumber("N", { N: "42" }, 7), 42);
      assert.equal(envNumber("N", {}, 7), 7);
      assert.throws(() => envNumber("N", { N: "12abc" }, 7), /N must be an integer value/);
    },
  },
  {
    name: "parseDecimalWei and parseBoolean fail clearly",
    run: () => {
      assert.equal(parseDecimalWei("WEI", "1000"), 1000n);
      assert.throws(() => parseDecimalWei("WEI", "1.5"), /WEI must be an integer wei value/);
      assert.equal(parseBoolean("FLAG", "true"), true);
      assert.equal(parseBoolean("FLAG", "false"), false);
      assert.throws(() => parseBoolean("FLAG", "maybe"), /FLAG must be true or false \(got 'maybe'\)/);
    },
  },
  {
    name: "readDotEnvContents strips quotes and comments; readDotEnvFile guards missing files",
    run: () => {
      assert.deepEqual(readDotEnvContents("A=1\nB='two'\nC=\"three\"\n#D=4\n"), {
        A: "1",
        B: "two",
        C: "three",
      });
      assert.throws(
        () => readDotEnvFile("/nonexistent/lineth-lib-env.test.env"),
        /is missing; copy \.env\.example to \.env first/,
      );
    },
  },
];

async function main() {
  for (const test of tests) {
    await test.run();
    console.log(`ok - ${test.name}`);
  }
}

main().catch((error) => {
  console.error(error instanceof Error ? error.stack ?? error.message : error);
  process.exit(1);
});
