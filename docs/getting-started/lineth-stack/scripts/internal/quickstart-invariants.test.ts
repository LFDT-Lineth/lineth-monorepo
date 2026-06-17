import assert from "node:assert/strict";

import { computeBootPrecomputedAddresses, computeGenesisShnarf } from "./quickstart-invariants";

async function test(name: string, fn: () => void | Promise<void>) {
  try {
    await fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    console.error(`not ok - ${name}`);
    console.error(error instanceof Error ? error.stack || error.message : String(error));
    process.exitCode = 1;
  }
}

void (async () => {
  await test("computes genesis shnarf with the LineaRollup V8 helper shape", () => {
    assert.equal(
      computeGenesisShnarf("0x1111111111111111111111111111111111111111111111111111111111111111"),
      "0x6c66ebb91228a0e9f41ec5060e5b6fdf4d8310db928e3b84b2d2e609b426bd8c",
    );
  });

  await test("rejects malformed genesis state roots", () => {
    assert.throws(() => computeGenesisShnarf("0x1234"), /genesis state root must be a 32-byte hex value/);
  });

  await test("precomputes boot-critical addresses from deterministic deployer nonces", () => {
    assert.deepEqual(
      computeBootPrecomputedAddresses({
        l1DeployerAddress: "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
        l1DeployerStartNonce: 0,
        l2DeployerAddress: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
      }),
      {
        l1LineaRollup: "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
        l2MessageService: "0x948B3c65b89DF0B4894ABE91E6D02FE579834F8F",
      },
    );
  });

  await test("precomputes L1 LineaRollup from the latest deployer nonce", () => {
    assert.equal(
      computeBootPrecomputedAddresses({
        l1DeployerAddress: "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
        l1DeployerStartNonce: 10,
        l2DeployerAddress: "0x70997970c51812dc3a010c7d01b50e0d17dc79c8",
      }).l1LineaRollup,
      "0x9A676e781A523b5d0C0e43731313A708CB607508",
    );
  });
})();
