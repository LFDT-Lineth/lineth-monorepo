import { getCreateAddress } from "ethers";
import assert from "node:assert/strict";
import test from "node:test";

import { buildAddressPlan } from "../src/address-plan";

const L1_SIGNER = "0x1000000000000000000000000000000000000001";
const L2_SIGNER = "0x2000000000000000000000000000000000000002";

test("builds the four-step deterministic deployment address plan", () => {
  const plan = buildAddressPlan({
    l1Signer: L1_SIGNER,
    l2Signer: L2_SIGNER,
    l1StartingNonce: 10,
    l2StartingNonce: 20,
  });

  assert.deepEqual(
    plan.map((step) => [step.id, step.deployments.length]),
    [
      ["l1-rollup", 5],
      ["l2-message-service", 3],
      ["l1-token-bridge", 5],
      ["l2-token-bridge", 5],
    ],
  );
  assert.equal(plan[0]?.deployments.at(-1)?.nonce, 14);
  assert.equal(plan[1]?.deployments.at(-1)?.nonce, 22);
  assert.equal(plan[2]?.deployments.at(-1)?.nonce, 19);
  assert.equal(plan[3]?.deployments.at(-1)?.nonce, 27);
  assert.equal(plan[0]?.deployments.at(-1)?.expectedAddress, getCreateAddress({ from: L1_SIGNER, nonce: 14 }));
  assert.equal(plan[3]?.deployments.at(-1)?.expectedAddress, getCreateAddress({ from: L2_SIGNER, nonce: 27 }));
});

test("uses distinct stable keys for repeated infrastructure contracts", () => {
  const plan = buildAddressPlan({
    l1Signer: L1_SIGNER,
    l2Signer: L2_SIGNER,
    l1StartingNonce: 0,
    l2StartingNonce: 0,
  });
  const keys = plan.flatMap((step) => step.deployments.map((deployment) => deployment.key));

  assert.equal(new Set(keys).size, keys.length);
  assert.ok(keys.includes("l1-token-bridge.proxy-admin"));
  assert.ok(keys.includes("l2-token-bridge.proxy-admin"));
});
