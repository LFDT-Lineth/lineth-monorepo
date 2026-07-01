import * as assert from "node:assert/strict";

import { waitForFundedBalance } from "./fund-runtime-accounts";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

class FakeBalanceProvider {
  readonly balanceReads: Array<{ address: string; blockTag?: number }> = [];
  private balanceIndex = 0;

  constructor(
    private readonly options: {
      blockNumber: number;
      balances: bigint[];
    },
  ) {}

  async getBlockNumber(): Promise<number> {
    return this.options.blockNumber;
  }

  async getBalance(address: string, blockTag?: number): Promise<bigint> {
    this.balanceReads.push(blockTag === undefined ? { address } : { address, blockTag });
    const balance = this.options.balances[Math.min(this.balanceIndex, this.options.balances.length - 1)];
    this.balanceIndex += 1;
    if (balance === undefined) {
      throw new Error("missing fake balance");
    }
    return balance;
  }
}

const tests: TestCase[] = [
  {
    name: "retries post-receipt balance reads until RPC catches up",
    run: async () => {
      const provider = new FakeBalanceProvider({
        blockNumber: 123,
        balances: [0n, 500_000_000_000_000_000n],
      });

      const balance = await waitForFundedBalance({
        provider,
        label: "L1",
        targetLabel: "finalization submitter",
        address: "0x1111111111111111111111111111111111111111",
        minBalance: 400_000_000_000_000_000n,
        receiptBlockNumber: 123,
        timeoutMs: 100,
        pollIntervalMs: 1,
        requestTimeoutMs: 50,
      });

      assert.equal(balance, 500_000_000_000_000_000n);
      assert.equal(provider.balanceReads.length, 2);
      assert.deepEqual(
        provider.balanceReads.map((read) => read.blockTag),
        [123, 123],
      );
    },
  },
  {
    name: "preserves low-balance failure wording after retry timeout",
    run: async () => {
      const provider = new FakeBalanceProvider({
        blockNumber: 123,
        balances: [0n],
      });

      await assert.rejects(
        () =>
          waitForFundedBalance({
            provider,
            label: "L1",
            targetLabel: "finalization submitter",
            address: "0x1111111111111111111111111111111111111111",
            minBalance: 400_000_000_000_000_000n,
            receiptBlockNumber: 123,
            timeoutMs: 5,
            pollIntervalMs: 1,
            requestTimeoutMs: 50,
          }),
        /L1 funding finalization submitter left balance 0 below minimum 400000000000000000/,
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
