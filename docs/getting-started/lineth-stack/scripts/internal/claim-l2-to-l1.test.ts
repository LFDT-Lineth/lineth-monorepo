import * as assert from "node:assert/strict";

process.env.CLAIM_L2_TO_L1_DISABLE_MAIN = "true";

const PRIVATE_KEY = `0x${"11".repeat(32)}`;
const LINEA_ROLLUP = "0x1111111111111111111111111111111111111111";
const L2_MESSAGE_SERVICE = "0x2222222222222222222222222222222222222222";
const MESSAGE_HASH = `0x${"aa".repeat(32)}`;
const MESSAGE_SENDER = "0x3333333333333333333333333333333333333333";
const DESTINATION = "0x4444444444444444444444444444444444444444";
const CALLDATA = "0x1234";
const CLAIM_TX_HASH = `0x${"bb".repeat(32)}`;
const PROOF_ROOT = `0x${"cc".repeat(32)}`;
const CLAIMANT = "0x5555555555555555555555555555555555555555";

type TestCase = {
  name: string;
  run: () => Promise<void> | void;
};

function validEnv(overrides: Record<string, string | undefined> = {}) {
  return {
    L1_RPC_URL: "https://sepolia.example",
    L2_RPC_URL: "http://l2.example",
    L1_SIGNER_PRIVATE_KEY: PRIVATE_KEY,
    SMOKE_L1_CHAIN_ID: "11155111",
    SMOKE_L2_CHAIN_ID: "1337",
    SMOKE_LINEA_ROLLUP_ADDRESS: LINEA_ROLLUP,
    SMOKE_L2_MESSAGE_SERVICE_ADDRESS: L2_MESSAGE_SERVICE,
    SMOKE_MESSAGE_HASH: MESSAGE_HASH,
    SMOKE_MESSAGE_SENDER: MESSAGE_SENDER,
    SMOKE_DESTINATION: DESTINATION,
    SMOKE_FEE: "10",
    SMOKE_VALUE: "20",
    SMOKE_MESSAGE_NONCE: "30",
    SMOKE_CALLDATA: CALLDATA,
    SMOKE_SENT_BLOCK_NUMBER: "40",
    ...overrides,
  };
}

function fakeDeps(overrides: Record<string, unknown> = {}) {
  const calls: Record<string, unknown[]> = {
    publicClients: [],
    walletClients: [],
    statuses: [],
    proofs: [],
    claims: [],
  };
  const deps = {
    zeroAddress: "0x0000000000000000000000000000000000000000",
    http: (url: string) => ({ url }),
    privateKeyToAccount: (privateKey: string) => ({ address: CLAIMANT, privateKey }),
    createPublicClient: (client: unknown) => {
      calls.publicClients.push(client);
      return { kind: "public", client };
    },
    createWalletClient: (client: unknown) => {
      calls.walletClients.push(client);
      return { kind: "wallet", client };
    },
    getL2ToL1MessageStatus: async (...args: unknown[]) => {
      calls.statuses.push(args);
      return "CLAIMABLE";
    },
    getMessageProof: async (...args: unknown[]) => {
      calls.proofs.push(args);
      return { root: PROOF_ROOT, leafIndex: 7, proof: ["0x01", "0x02"] };
    },
    claimOnL1: async (...args: unknown[]) => {
      calls.claims.push(args);
      return CLAIM_TX_HASH;
    },
    ...overrides,
  };
  return { deps, calls };
}

async function expectRejectsMessage(run: () => Promise<unknown>, expected: RegExp) {
  await assert.rejects(run, (error) => {
    assert.ok(error instanceof Error);
    assert.match(error.message, expected);
    return true;
  });
}

const { claimL2ToL1 } = require("./claim-l2-to-l1");

const tests: TestCase[] = [
  {
    name: "required env validation fails clearly",
    run: async () => {
      const { deps } = fakeDeps();
      await expectRejectsMessage(
        () => claimL2ToL1(validEnv({ L1_RPC_URL: "" }), deps),
        /L1_RPC_URL is required/,
      );
    },
  },
  {
    name: "non-claimable SDK status fails clearly",
    run: async () => {
      const { deps } = fakeDeps({
        getL2ToL1MessageStatus: async () => "UNKNOWN",
      });

      await expectRejectsMessage(() => claimL2ToL1(validEnv(), deps), /L2->L1 message is UNKNOWN, not CLAIMABLE/);
    },
  },
  {
    name: "success passes exact message tuple and proof to claimOnL1",
    run: async () => {
      const { deps, calls } = fakeDeps();

      const result = await claimL2ToL1(validEnv(), deps);

      assert.deepEqual(result, {
        status: "CLAIMABLE",
        claimTxHash: CLAIM_TX_HASH,
        proofRoot: PROOF_ROOT,
        proofLeafIndex: 7,
        proofLength: 2,
        claimant: CLAIMANT,
      });
      assert.equal(calls.statuses.length, 1);
      assert.equal(calls.proofs.length, 1);
      assert.equal(calls.claims.length, 1);

      const [, common] = calls.statuses[0] as [unknown, Record<string, unknown>];
      assert.equal(common.messageHash, MESSAGE_HASH);
      assert.equal(common.lineaRollupAddress, LINEA_ROLLUP);
      assert.equal(common.l2MessageServiceAddress, L2_MESSAGE_SERVICE);
      assert.deepEqual(common.l2LogsBlockRange, { fromBlock: 40n, toBlock: 40n });

      const [, claim] = calls.claims[0] as [unknown, Record<string, unknown>];
      assert.equal(claim.from, MESSAGE_SENDER);
      assert.equal(claim.to, DESTINATION);
      assert.equal(claim.fee, 10n);
      assert.equal(claim.value, 20n);
      assert.equal(claim.messageNonce, 30n);
      assert.equal(claim.calldata, CALLDATA);
      assert.equal(claim.feeRecipient, "0x0000000000000000000000000000000000000000");
      assert.deepEqual(claim.messageProof, { root: PROOF_ROOT, leafIndex: 7, proof: ["0x01", "0x02"] });
      assert.equal(claim.lineaRollupAddress, LINEA_ROLLUP);
    },
  },
  {
    name: "SDK errors are redacted before they leave the helper",
    run: async () => {
      const { deps } = fakeDeps({
        getMessageProof: async () => {
          throw new Error(`upstream failure used ${PRIVATE_KEY}`);
        },
      });

      await assert.rejects(
        () => claimL2ToL1(validEnv(), deps),
        (error) => {
          assert.ok(error instanceof Error);
          assert.match(error.message, /upstream failure used \[REDACTED\]/);
          assert.equal(error.message.includes(PRIVATE_KEY), false);
          return true;
        },
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
  console.error(error);
  process.exit(1);
});
