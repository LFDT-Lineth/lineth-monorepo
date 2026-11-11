import * as assert from "node:assert/strict";

import { getBooleanEnvVar, getBooleanEnvVarOrDefault, getOptionalEnvVarList } from "./environment";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const ENV_NAME = "LINETH_BOOLEAN_ENV_VAR_TEST_FLAG";

function withEnv(value: string | undefined, run: () => void): void {
  const previous = process.env[ENV_NAME];
  if (value === undefined) {
    delete process.env[ENV_NAME];
  } else {
    process.env[ENV_NAME] = value;
  }
  try {
    run();
  } finally {
    if (previous === undefined) {
      delete process.env[ENV_NAME];
    } else {
      process.env[ENV_NAME] = previous;
    }
  }
}

const tests: TestCase[] = [
  {
    name: 'getBooleanEnvVarOrDefault parses "true" and "false"',
    run: () => {
      withEnv("true", () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, false), true));
      withEnv("false", () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, true), false));
    },
  },
  {
    name: "getBooleanEnvVarOrDefault returns the default when unset or empty",
    run: () => {
      withEnv(undefined, () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, true), true));
      withEnv(undefined, () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, false), false));
      withEnv("", () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, true), true));
      withEnv("", () => assert.equal(getBooleanEnvVarOrDefault(ENV_NAME, false), false));
    },
  },
  {
    name: "getBooleanEnvVarOrDefault throws on any other value",
    run: () => {
      withEnv("1", () =>
        assert.throws(() => getBooleanEnvVarOrDefault(ENV_NAME, false), /must be either "true" or "false"/),
      );
      withEnv("yes", () =>
        assert.throws(() => getBooleanEnvVarOrDefault(ENV_NAME, false), /must be either "true" or "false"/),
      );
      withEnv("TRUE", () =>
        assert.throws(() => getBooleanEnvVarOrDefault(ENV_NAME, false), /must be either "true" or "false"/),
      );
    },
  },
  {
    name: "getBooleanEnvVar throws when unset or empty",
    run: () => {
      withEnv(undefined, () => assert.throws(() => getBooleanEnvVar(ENV_NAME), /missing or empty/));
      withEnv("", () => assert.throws(() => getBooleanEnvVar(ENV_NAME), /missing or empty/));
    },
  },
  {
    name: "getBooleanEnvVar parses a set boolean value",
    run: () => {
      withEnv("true", () => assert.equal(getBooleanEnvVar(ENV_NAME), true));
      withEnv("false", () => assert.equal(getBooleanEnvVar(ENV_NAME), false));
      withEnv("nope", () => assert.throws(() => getBooleanEnvVar(ENV_NAME), /must be either "true" or "false"/));
    },
  },
  {
    name: "getOptionalEnvVarList returns an empty list when unset, empty or whitespace-only",
    run: () => {
      withEnv(undefined, () => assert.deepEqual(getOptionalEnvVarList(ENV_NAME), []));
      withEnv("", () => assert.deepEqual(getOptionalEnvVarList(ENV_NAME), []));
      withEnv("  ", () => assert.deepEqual(getOptionalEnvVarList(ENV_NAME), []));
    },
  },
  {
    name: "getOptionalEnvVarList splits on commas, trims entries and drops empty ones",
    run: () => {
      withEnv("0xaa", () => assert.deepEqual(getOptionalEnvVarList(ENV_NAME), ["0xaa"]));
      withEnv(" 0xaa , 0xbb,,0xcc, ", () =>
        assert.deepEqual(getOptionalEnvVarList(ENV_NAME), ["0xaa", "0xbb", "0xcc"]),
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
  console.error(error instanceof Error ? error.stack || error.message : String(error));
  process.exit(1);
});
