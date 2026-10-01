import assert from "node:assert/strict";
import test from "node:test";

import { resolveGenesisTimestamp } from "../src/genesis";

test("derives the genesis timestamp from L2 block zero", async () => {
  const provider = {
    getBlock: async (blockNumber: number) => {
      assert.equal(blockNumber, 0);
      return { timestamp: 1_700_000_000 };
    },
  };

  assert.equal(await resolveGenesisTimestamp(provider, undefined), 1_700_000_000);
});

test("accepts a valid override without querying block zero", async () => {
  const provider = {
    getBlock: async () => {
      throw new Error("block zero must not be queried");
    },
  };

  assert.equal(await resolveGenesisTimestamp(provider, "1700000001"), 1_700_000_001);
});

test("rejects an invalid genesis timestamp override", async () => {
  await assert.rejects(() => resolveGenesisTimestamp({ getBlock: async () => null }, "-1"), /non-negative integer/);
});
