import * as assert from "node:assert/strict";

import { getBooleanEnvVar, getBooleanEnvVarOrDefault } from "./environment";

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
