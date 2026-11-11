import { expect } from "chai";
import { concat, encodeAbiParameters, parseAbiParameters } from "viem";

import { getWalletForIndex } from "./index.js";
import { TEST_PUBLIC_VERIFIER_INDEX } from "../../common/constants/index.js";
import { getConnection } from "../../common/helpers/connection.js";
import { type AssertionTarget } from "../../common/helpers/expectations.js";
import {
  calculateLastFinalizedState,
  expectEventDirectFromReceiptData,
  generateFinalizationData,
  generateKeccak256,
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
  );
  const blobShnarfExists = await upgradedContract.read.blobShnarfExists([finalShnarf]);
  expect(blobShnarfExists).to.equal(1n);

  await upgradedContract.write.setRollingHash([
    BigInt(proofData.l1RollingHashMessageNumber),
    hex(proofData.l1RollingHash),
  ]);

  const txData = [
    hex(proofData.aggregatedProof),
    0n,
    [
      finalizationData.parentStateRootHash,
      BigInt(finalizationData.endBlockNumber),
      [
        shnarfData.parentShnarf,
        shnarfData.snarkHash,
        shnarfData.finalStateRootHash,
        shnarfData.dataEvaluationPoint,
        shnarfData.dataEvaluationClaim,
      ],
      finalizationData.lastFinalizedTimestamp,
      finalizationData.finalTimestamp,
      finalizationData.lastFinalizedL1RollingHash,
      finalizationData.l1RollingHash,
      finalizationData.lastFinalizedL1RollingHashMessageNumber,
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.l2MerkleTreesDepth,
      finalizationData.lastFinalizedForcedTransactionNumber,
      finalizationData.finalForcedTransactionNumber,
      finalizationData.lastFinalizedForcedTransactionRollingHash,
      finalizationData.l2MerkleRoots,
      finalizationData.filteredAddresses,
      finalizationData.l2MessagingBlocksOffsets,
    ],
  ] as const;

  const encodedCall = concat([
    "0x755bc62f",
    encodeAbiParameters(
      parseAbiParameters(
        "bytes, uint256, (bytes32,uint256,(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,uint256,uint256,bytes32,bytes32[],address[],bytes)",
      ),
      txData,
    ),
  ]);

  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const { maxFeePerGas, maxPriorityFeePerGas } = await publicClient.estimateFeesPerGas();
  const operatorHDSigner = await getWalletForIndex(2);
  const nonce = await publicClient.getTransactionCount({ address: operatorHDSigner.address, blockTag: "pending" });

  const signedTx = await operatorHDSigner.signTransaction({
    data: encodedCall,
    maxPriorityFeePerGas,
    maxFeePerGas,
    to: callforwarderAddress,
    chainId: await publicClient.getChainId(),
    type: "eip1559",
    nonce,
    value: 0n,
    gas: 10_000_000n,
  });

  const txHash = await publicClient.sendRawTransaction({ serializedTransaction: signedTx });
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
