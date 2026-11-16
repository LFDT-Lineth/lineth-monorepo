import { expect } from "chai";
import { encodeFunctionData } from "viem";

import { generateKeccak256 } from "../../../../common/helpers/hashing.js";
import { TEST_PUBLIC_VERIFIER_INDEX } from "../../common/constants/index.js";
import { getAccountsFixture } from "../../common/helpers/before.js";
import { getConnection } from "../../common/helpers/connection.js";
import { type AssertionTarget } from "../../common/helpers/expectations.js";
import {
  calculateLastFinalizedState,
  expectEventDirectFromReceiptData,
  generateFinalizationData,
  hex,
  proofDataToFinalizationParams,
  expectEvent,
  expectRevertWithCustomError,
} from "../../common/helpers/index.js";

import type { FailedFinalizeParams, SucceedFinalizeParams, SucceedFinalizeParamsCallForwardingProxy } from "./type.js";

export async function expectSuccessfulFinalize(params: SucceedFinalizeParams) {
  const { context, proofConfig, overrides = {} } = params;
  const { linethRollup, operator } = context;
  const { proofData } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await linethRollup.write.setRollingHash([BigInt(proofData.l1RollingHashMessageNumber), hex(proofData.l1RollingHash)]);

  const finalizeCompressedCall = linethRollup.write.finalizeBlocks(
    [hex(proofData.aggregatedProof), BigInt(TEST_PUBLIC_VERIFIER_INDEX), finalizationData],
    { account: operator.account },
  );

  await expectEvent(linethRollup, finalizeCompressedCall, "FinalizedStateUpdated", [
    finalizationData.endBlockNumber,
    finalizationData.finalTimestamp,
    finalizationData.l1RollingHashMessageNumber,
    finalizationData.finalForcedTransactionNumber,
  ]);

  await expectEvent(linethRollup, finalizeCompressedCall, "DataFinalizedV3", [
    BigInt(proofData.lastFinalizedBlockNumber) + 1n,
    finalizationData.endBlockNumber,
    hex(proofData.finalShnarf),
    finalizationData.parentStateRootHash,
    hex(proofData.finalStateRootHash),
  ]);

  const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
    linethRollup.read.stateRootHashes([finalizationData.endBlockNumber]),
    linethRollup.read.currentL2BlockNumber(),
    linethRollup.read.currentFinalizedState(),
  ]);

  expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
  expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
  expect(lastFinalizedState).to.equal(
    calculateLastFinalizedState(
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.l1RollingHash,
      BigInt(proofData.finalFtxNumber),
      hex(proofData.finalFtxRollingHash),
      finalizationData.finalTimestamp,
    ),
  );
}

export async function expectFailedCustomErrorFinalize(params: FailedFinalizeParams) {
  const { context, proofConfig, expectedError, overrides = {} } = params;
  const { linethRollup, operator } = context;
  const { proofData } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await linethRollup.write.setRollingHash([BigInt(proofData.l1RollingHashMessageNumber), hex(proofData.l1RollingHash)]);

  const finalizeCompressedCall = linethRollup.write.finalizeBlocks(
    [hex(proofData.aggregatedProof), BigInt(TEST_PUBLIC_VERIFIER_INDEX), finalizationData],
    { account: operator.account },
  );

  const target: AssertionTarget = linethRollup;
  await expectRevertWithCustomError(target, finalizeCompressedCall, expectedError.name, expectedError.args);
}

export async function expectSuccessfulFinalizeViaCallForwarder(params: SucceedFinalizeParamsCallForwardingProxy) {
  const { context, proofConfig, overrides = {} } = params;
  const { upgradedContract, callforwarderAddress } = context;
  const { proofData, blobParentShnarfIndex, shnarfDataGenerator, isMultiple } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await upgradedContract.write.setRollingHash([
    BigInt(proofData.l1RollingHashMessageNumber),
    hex(proofData.l1RollingHash),
  ]);

  const shnarfData = shnarfDataGenerator(blobParentShnarfIndex, isMultiple);

  const finalShnarf = generateKeccak256(
    ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
    [
      shnarfData.parentShnarf,
      shnarfData.snarkHash,
      shnarfData.finalStateRootHash,
      shnarfData.dataEvaluationPoint,
      shnarfData.dataEvaluationClaim,
    ],
    { encodePacked: false },
  );
  const blobShnarfExists = await upgradedContract.read.blobShnarfExists([finalShnarf]);
  expect(blobShnarfExists).to.equal(1n);

  await upgradedContract.write.setRollingHash([
    BigInt(proofData.l1RollingHashMessageNumber),
    hex(proofData.l1RollingHash),
  ]);

  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const { operator } = await getAccountsFixture();
  const txHash = await operator.sendTransaction({
    to: callforwarderAddress,
    data: encodeFunctionData({
      abi: upgradedContract.abi,
      functionName: "finalizeBlocks",
      args: [hex(proofData.aggregatedProof), BigInt(TEST_PUBLIC_VERIFIER_INDEX), { ...finalizationData, shnarfData }],
    }),
    gas: 10_000_000n,
  });
  const receipt = await publicClient.waitForTransactionReceipt({ hash: txHash });
  expect(receipt).is.not.null;

  const finalizedStateUpdatedLogIndex = 8;
  const dataFinalizedLogIndex = 9;

  expectEventDirectFromReceiptData(
    upgradedContract,
    receipt,
    "FinalizedStateUpdated",
    [
      finalizationData.endBlockNumber,
      finalizationData.finalTimestamp,
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.finalForcedTransactionNumber,
    ],
    finalizedStateUpdatedLogIndex,
  );

  expectEventDirectFromReceiptData(
    upgradedContract,
    receipt,
    "DataFinalizedV3",
    [
      BigInt(proofData.lastFinalizedBlockNumber) + 1n,
      finalizationData.endBlockNumber,
      proofData.finalShnarf,
      finalizationData.parentStateRootHash,
      proofData.finalStateRootHash,
    ],
    dataFinalizedLogIndex,
  );

  const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
    upgradedContract.read.stateRootHashes([finalizationData.endBlockNumber]),
    upgradedContract.read.currentL2BlockNumber(),
    upgradedContract.read.currentFinalizedState(),
  ]);

  expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
  expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
  expect(lastFinalizedState).to.equal(
    calculateLastFinalizedState(
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.l1RollingHash,
      finalizationData.finalForcedTransactionNumber,
      hex(proofData.finalFtxRollingHash),
      finalizationData.finalTimestamp,
    ),
  );
}
