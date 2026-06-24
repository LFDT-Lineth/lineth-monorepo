import { ethers } from "ethers";
import * as assert from "node:assert/strict";

import {
  assertSingleFeeModel,
  feeBudgetPricePerGas,
  parseGasPriceWeiEnv,
  resolveOneModelFeeOverrides,
} from "./feeOverrides";

type FeeData = {
  gasPrice?: bigint | null;
  maxFeePerGas?: bigint | null;
  maxPriorityFeePerGas?: bigint | null;
};

function mockProvider(feeData: FeeData): ethers.Provider {
  return {
    getFeeData: async () => feeData,
  } as unknown as ethers.Provider;
}

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

const ENV_NAME = "LINETH_FEE_OVERRIDES_TEST_GAS_PRICE_WEI";

function withEnv(value: string | undefined, run: () => Promise<void> | void): Promise<void> | void {
  const previous = process.env[ENV_NAME];
  if (value === undefined) {
    delete process.env[ENV_NAME];
  } else {
    process.env[ENV_NAME] = value;
  }
  const restore = () => {
    if (previous === undefined) {
      delete process.env[ENV_NAME];
    } else {
      process.env[ENV_NAME] = previous;
    }
  };
  try {
    const result = run();
    if (result instanceof Promise) {
      return result.finally(restore);
    }
    restore();
    return result;
  } catch (error) {
    restore();
    throw error;
  }
}

const tests: TestCase[] = [
  {
    name: "parseGasPriceWeiEnv returns undefined when unset or empty",
    run: () => {
      withEnv(undefined, () => assert.equal(parseGasPriceWeiEnv(ENV_NAME), undefined));
      withEnv("", () => assert.equal(parseGasPriceWeiEnv(ENV_NAME), undefined));
    },
  },
  {
    name: "parseGasPriceWeiEnv parses integer wei and rejects non-integers",
    run: () => {
      withEnv("5000000000", () => assert.equal(parseGasPriceWeiEnv(ENV_NAME), 5_000_000_000n));
      withEnv("12.5", () => assert.throws(() => parseGasPriceWeiEnv(ENV_NAME), /must be an integer wei value/));
      withEnv("0x10", () => assert.throws(() => parseGasPriceWeiEnv(ENV_NAME), /must be an integer wei value/));
    },
  },
  {
    name: "assertSingleFeeModel accepts a single legacy or complete EIP-1559 model",
    run: () => {
      assert.deepEqual(assertSingleFeeModel({ gasPrice: 7n }), { gasPrice: 7n });
      assert.deepEqual(assertSingleFeeModel({ maxFeePerGas: 9n, maxPriorityFeePerGas: 2n }), {
        maxFeePerGas: 9n,
        maxPriorityFeePerGas: 2n,
      });
    },
  },
  {
    name: "assertSingleFeeModel rejects mixed, partial, and empty fee models",
    run: () => {
      assert.throws(() => assertSingleFeeModel({ gasPrice: 1n, maxFeePerGas: 2n }), /must not set both/);
      assert.throws(
        () => assertSingleFeeModel({ maxFeePerGas: 2n }),
        /require both maxFeePerGas and maxPriorityFeePerGas/,
      );
      assert.throws(
        () => assertSingleFeeModel({ maxPriorityFeePerGas: 2n }),
        /require both maxFeePerGas and maxPriorityFeePerGas/,
      );
      assert.throws(() => assertSingleFeeModel({}), /must set a legacy gasPrice or complete EIP-1559 fields/);
    },
  },
  {
    name: "feeBudgetPricePerGas returns gasPrice or the EIP-1559 ceiling",
    run: () => {
      assert.equal(feeBudgetPricePerGas({ gasPrice: 11n }), 11n);
      assert.equal(feeBudgetPricePerGas({ maxFeePerGas: 21n, maxPriorityFeePerGas: 3n }), 21n);
    },
  },
  {
    name: "resolveOneModelFeeOverrides prefers the explicit legacy gas-price env var",
    run: async () => {
      const fees = await withEnv("5000000000", () =>
        resolveOneModelFeeOverrides(mockProvider({ maxFeePerGas: 9n, maxPriorityFeePerGas: 2n }), ENV_NAME),
      )!;
      assert.deepEqual(fees, { gasPrice: 5_000_000_000n });
    },
  },
  {
    name: "resolveOneModelFeeOverrides returns complete EIP-1559 fields from the provider",
    run: async () => {
      const fees = await withEnv(undefined, () =>
        resolveOneModelFeeOverrides(
          mockProvider({ maxFeePerGas: 9n, maxPriorityFeePerGas: 2n, gasPrice: 7n }),
          ENV_NAME,
        ),
      )!;
      assert.deepEqual(fees, { maxFeePerGas: 9n, maxPriorityFeePerGas: 2n });
    },
  },
  {
    name: "resolveOneModelFeeOverrides falls back to legacy provider gas price",
    run: async () => {
      const fees = await withEnv(undefined, () =>
        resolveOneModelFeeOverrides(mockProvider({ gasPrice: 7n }), ENV_NAME),
      )!;
      assert.deepEqual(fees, { gasPrice: 7n });
    },
  },
  {
    name: "resolveOneModelFeeOverrides throws on partial EIP-1559 provider data",
    run: async () => {
      await withEnv(undefined, async () => {
        await assert.rejects(
          () => resolveOneModelFeeOverrides(mockProvider({ maxFeePerGas: 9n }), ENV_NAME),
          /incomplete EIP-1559 deploy fee data/,
        );
      });
    },
  },
  {
    name: "resolveOneModelFeeOverrides throws when the provider returns no fee data",
    run: async () => {
      await withEnv(undefined, async () => {
        await assert.rejects(
          () => resolveOneModelFeeOverrides(mockProvider({}), ENV_NAME),
          /no usable deploy fee data/,
        );
      });
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
