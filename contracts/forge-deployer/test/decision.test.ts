import assert from "node:assert/strict";
import test from "node:test";

import { buildAddressPlan } from "../src/address-plan";
import { createCheckpoint, DEPLOYMENT_PROFILE, SCHEMA_VERSION } from "../src/checkpoint";
import { decideStepAction } from "../src/decision";

const STATE_ROOT = `0x${"ab".repeat(32)}`;
const IMAGE_DIGEST = `sha256:${"ef".repeat(32)}`;
const TX_HASH = `0x${"cd".repeat(32)}`;
const L1_SIGNER = "0x1000000000000000000000000000000000000001";
const L2_SIGNER = "0x2000000000000000000000000000000000000002";

function fixture() {
  const plan = buildAddressPlan({
    l1Signer: L1_SIGNER,
    l2Signer: L2_SIGNER,
    l1StartingNonce: 5,
    l2StartingNonce: 9,
  });
  const checkpoint = createCheckpoint({
    profile: DEPLOYMENT_PROFILE,
    schemaVersion: SCHEMA_VERSION,
    artifactDigest: IMAGE_DIGEST,
    initialL2StateRootHash: STATE_ROOT,
    l2GenesisTimestamp: 1_700_000_000,
    chainIds: { l1: "1", l2: "1337" },
    signers: { l1: L1_SIGNER, l2: L2_SIGNER },
    configurationHash: `0x${"12".repeat(32)}`,
    bootstrapHash: `0x${"00".repeat(32)}`,
    startingNonces: { l1: 5, l2: 9 },
    plan,
  });
  return { checkpoint, step: plan[0]! };
}

test("skips only when all records and bytecode verify", async () => {
  const { checkpoint, step } = fixture();
  const codeByKey: Record<string, boolean> = {};
  for (const deployment of step.deployments) {
    codeByKey[deployment.key] = true;
    checkpoint.deployments[deployment.key] = {
      address: deployment.expectedAddress,
      transactionHash: TX_HASH,
      blockNumber: 100,
      chainId: "1",
      recovered: false,
    };
  }
  checkpoint.completedSteps.push(step.id);

  assert.deepEqual(await decideStepAction({ step, checkpoint, codeByKey, currentNonce: 10 }), { action: "skip" });
});

test("recovers a fully checkpointed step when only its completion marker is missing", async () => {
  const { checkpoint, step } = fixture();
  const codeByKey = Object.fromEntries(step.deployments.map((deployment) => [deployment.key, true]));
  for (const deployment of step.deployments) {
    checkpoint.deployments[deployment.key] = {
      address: deployment.expectedAddress,
      transactionHash: TX_HASH,
      blockNumber: 100,
      chainId: "1",
      recovered: false,
    };
  }

  assert.deepEqual(await decideStepAction({ step, checkpoint, codeByKey, currentNonce: 10 }), { action: "recover" });
});

test("fails closed when addresses contain code without durable deployment records", async () => {
  const { checkpoint, step } = fixture();
  const codeByKey = Object.fromEntries(step.deployments.map((deployment) => [deployment.key, true]));

  await assert.rejects(decideStepAction({ step, checkpoint, codeByKey, currentNonce: 10 }), /uncheckpointed code/);
});

test("deploys only when the range is empty and nonce has not drifted", async () => {
  const { checkpoint, step } = fixture();
  const codeByKey = Object.fromEntries(step.deployments.map((deployment) => [deployment.key, false]));

  assert.deepEqual(await decideStepAction({ step, checkpoint, codeByKey, currentNonce: 5 }), { action: "deploy" });
});

test("fails closed on partial deployment, nonce drift, or lost bytecode", async () => {
  const { checkpoint, step } = fixture();
  const partialCode = Object.fromEntries(step.deployments.map((deployment, index) => [deployment.key, index === 0]));
  const noCode = Object.fromEntries(step.deployments.map((deployment) => [deployment.key, false]));

  await assert.rejects(
    decideStepAction({ step, checkpoint, codeByKey: partialCode, currentNonce: 6 }),
    /partial deployment/,
  );
  await assert.rejects(decideStepAction({ step, checkpoint, codeByKey: noCode, currentNonce: 6 }), /nonce drift/);

  const first = step.deployments[0]!;
  checkpoint.deployments[first.key] = {
    address: first.expectedAddress,
    transactionHash: TX_HASH,
    blockNumber: 100,
    chainId: "1",
    recovered: false,
  };
  await assert.rejects(
    decideStepAction({ step, checkpoint, codeByKey: noCode, currentNonce: 5 }),
    /checkpointed deployment.*has no bytecode/,
  );
});

test("deterministic proxy step skips on existing factory code, ignoring nonce drift", async () => {
  const { checkpoint } = fixture();
  const plan = buildAddressPlan({ l1Signer: L1_SIGNER, l2Signer: L2_SIGNER, l1StartingNonce: 5, l2StartingNonce: 9 });
  const step = plan.find((candidate) => candidate.id === "l2-deterministic-proxy")!;
  const deployment = step.deployments[0]!;

  // Factory already deployed and checkpointed: skip even though the L2 deployer
  // nonce advanced past the funding-send nonce on the prior run.
  checkpoint.deployments[deployment.key] = {
    address: deployment.expectedAddress,
    transactionHash: TX_HASH,
    blockNumber: 200,
    chainId: "1337",
    recovered: false,
  };
  checkpoint.completedSteps.push(step.id);

  assert.deepEqual(
    await decideStepAction({ step, checkpoint, codeByKey: { [deployment.key]: true }, currentNonce: 18 }),
    { action: "skip" },
  );
});

test("adopts uncheckpointed well-known-address code when the verifier reports a match", async () => {
  const { checkpoint } = fixture();
  const plan = buildAddressPlan({ l1Signer: L1_SIGNER, l2Signer: L2_SIGNER, l1StartingNonce: 5, l2StartingNonce: 9 });
  const step = plan.find((candidate) => candidate.id === "l2-deterministic-proxy")!;
  const deployment = step.deployments[0]!;

  const result = await decideStepAction({
    step,
    checkpoint,
    codeByKey: { [deployment.key]: true },
    currentNonce: 18,
    verifyWellKnownCode: async () => "match",
  });

  assert.deepEqual(result, { action: "adopt" });
});

test("refuses to adopt uncheckpointed well-known-address code when the verifier reports a mismatch", async () => {
  const { checkpoint } = fixture();
  const plan = buildAddressPlan({ l1Signer: L1_SIGNER, l2Signer: L2_SIGNER, l1StartingNonce: 5, l2StartingNonce: 9 });
  const step = plan.find((candidate) => candidate.id === "l2-deterministic-proxy")!;
  const deployment = step.deployments[0]!;

  await assert.rejects(
    decideStepAction({
      step,
      checkpoint,
      codeByKey: { [deployment.key]: true },
      currentNonce: 18,
      verifyWellKnownCode: async () => "mismatch",
    }),
    /unexpected bytecode/,
  );
});
