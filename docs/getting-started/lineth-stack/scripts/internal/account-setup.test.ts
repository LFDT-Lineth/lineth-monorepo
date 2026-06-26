import * as assert from "node:assert/strict";
import { createHash } from "node:crypto";

import { Wallet } from "ethers";

import { buildPrecomputedAddressPlan } from "./account-setup";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

function testPrivateKey(label: string): string {
  return `0x${createHash("sha256").update(`lineth-stack-account-setup-test:${label}`).digest("hex")}`;
}

function testWallet(label: string): Wallet {
  return new Wallet(testPrivateKey(label));
}

function testWallets() {
  return {
    L1_BLOB_SUBMITTER_PRIVATE_KEY: testWallet("l1-blob"),
    L1_FINALIZATION_SUBMITTER_PRIVATE_KEY: testWallet("l1-finalization"),
    L1_POSTMAN_PRIVATE_KEY: testWallet("l1-postman"),
    L2_DEPLOYER_PRIVATE_KEY: testWallet("l2-deployer"),
    L2_MESSAGE_ANCHORING_PRIVATE_KEY: testWallet("l2-anchoring"),
    L2_POSTMAN_PRIVATE_KEY: testWallet("l2-postman"),
  };
}

function buildPlan(latestL1Nonce: number, existing?: unknown) {
  const wallets = testWallets();
  return buildPrecomputedAddressPlan({
    existing,
    l1Mode: "local",
    l1ChainId: "31648428",
    latestL1Nonce,
    l1AccountSetupBlockNumber: 12,
    l1PostmanListenerStartBlock: 7,
    l1Deployer: testWallet("l1-deployer"),
    l2Deployer: wallets.L2_DEPLOYER_PRIVATE_KEY,
    wallets,
  });
}

const tests: TestCase[] = [
  {
    name: "reuses existing precomputed addresses when current deployer nonce is higher",
    run: () => {
      const first = buildPlan(5);
      const restarted = buildPlan(25, first.plan);

      assert.equal(first.reused, false);
      assert.equal(restarted.reused, true);
      assert.equal(restarted.plan._meta.l1DeployerStartNonce, 5);
      assert.equal(restarted.plan.l1.LineaRollupV8, first.plan.l1.LineaRollupV8);
      assert.equal(restarted.plan.l2.L2MessageService, first.plan.l2.L2MessageService);
    },
  },
  {
    name: "rejects incompatible existing runtime signer addresses",
    run: () => {
      const first = buildPlan(5);
      const broken = {
        ...first.plan,
        signers: {
          ...first.plan.signers,
          l1PostmanAddress: testWallet("different-l1-postman").address,
        },
      };

      assert.throws(() => buildPlan(25, broken), /l1PostmanAddress changed .* run \.\/scripts\/reset\.sh/);
    },
  },
  {
    name: "rejects incompatible existing deterministic deploy addresses",
    run: () => {
      const first = buildPlan(5);
      const broken = {
        ...first.plan,
        l1: {
          ...first.plan.l1,
          LineaRollupV8: testWallet("wrong-rollup-address").address,
        },
      };

      assert.throws(() => buildPlan(25, broken), /L1 LineaRollupV8 changed .* run \.\/scripts\/reset\.sh/);
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
