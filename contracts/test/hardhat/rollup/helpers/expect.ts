import { expect } from "chai";
import { BaseContract, Transaction } from "ethers";
import { ethers } from "hardhat";

import { getWalletForIndex } from "./";
import { FailedFinalizeParams, SucceedFinalizeParams, SucceedFinalizeParamsCallForwardingProxy } from "./type";
import { TEST_PUBLIC_VERIFIER_INDEX } from "../../common/constants";
import {
  calculateLastFinalizedState,
  expectEvent,
  expectEventDirectFromReceiptData,
  expectRevertWithCustomError,
  generateFinalizationData,
  generateKeccak256,
  proofDataToFinalizationParams,
} from "../../common/helpers";

export async function expectSuccessfulFinalize(params: SucceedFinalizeParams) {
  const { context, proofConfig, overrides = {} } = params;
  const { lineaRollup, operator } = context;
  const { proofData } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

  const finalizeCompressedCall = lineaRollup
    .connect(operator)
    .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

  await expectEvent(lineaRollup, finalizeCompressedCall, "FinalizedStateUpdated", [
    finalizationData.endBlockNumber,
    finalizationData.finalTimestamp,
    finalizationData.l1RollingHashMessageNumber,
    finalizationData.finalForcedTransactionNumber,
  ]);

  await expectEvent(lineaRollup, finalizeCompressedCall, "DataFinalizedV3", [
    BigInt(proofData.lastFinalizedBlockNumber) + 1n,
    finalizationData.endBlockNumber,
    proofData.finalShnarf,
    finalizationData.parentStateRootHash,
    proofData.finalStateRootHash,
  ]);

  const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
    lineaRollup.stateRootHashes(finalizationData.endBlockNumber),
    lineaRollup.currentL2BlockNumber(),
    lineaRollup.currentFinalizedState(),
  ]);

  expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
  expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
  expect(lastFinalizedState).to.equal(
    calculateLastFinalizedState(
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.l1RollingHash,
      BigInt(proofData.finalFtxNumber),
      proofData.finalFtxRollingHash,
      finalizationData.finalTimestamp,
    ),
  );
}

export async function expectFailedCustomErrorFinalize(params: FailedFinalizeParams) {
  const { context, proofConfig, expectedError, overrides = {} } = params;
  const { lineaRollup, operator } = context;
  const { proofData } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

  const finalizeCompressedCall = lineaRollup
    .connect(operator)
    .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

  await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, expectedError.name, expectedError.args ?? []);
}

export async function expectSuccessfulFinalizeViaCallForwarder(params: SucceedFinalizeParamsCallForwardingProxy) {
  const { context, proofConfig, overrides = {} } = params;
  const { upgradedContract, callforwarderAddress } = context;
  const { proofData, blobParentShnarfIndex, shnarfDataGenerator, isMultiple } = proofConfig;

  const finalizationData = await generateFinalizationData({
    ...proofDataToFinalizationParams(proofConfig),
    ...overrides,
  });

  await upgradedContract.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

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
  const blobShnarfExists = await upgradedContract.blobShnarfExists(finalShnarf);
  expect(blobShnarfExists).to.equal(1n);

  await upgradedContract.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

  const txData = [
    proofData.aggregatedProof,
    0,
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
  ];

  const encodedCall = ethers.concat([
    "0x755bc62f",
    ethers.AbiCoder.defaultAbiCoder().encode(
      [
        "bytes",
        "uint256",
        "tuple(bytes32,uint256,tuple(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,uint256,uint256,bytes32,bytes32[],address[],bytes)",
      ],
      txData,
    ),
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
  const operatorHDSigner = getWalletForIndex(2);
  const nonce = await operatorHDSigner.getNonce();

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: callforwarderAddress,
    chainId: (await ethers.provider.getNetwork()).chainId,
    type: 2,
    nonce,
    value: 0,
    gasLimit: 10_000_000,
  });

  const signedTx = await operatorHDSigner.signTransaction(transaction);

  const txResponse = await ethers.provider.broadcastTransaction(signedTx);
  const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);
  expect(receipt).is.not.null;

  const finalizedStateUpdatedLogIndex = 8;
  const dataFinalizedLogIndex = 9;

  expectEventDirectFromReceiptData(
    upgradedContract as BaseContract,
    receipt!,
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
    upgradedContract as BaseContract,
    receipt!,
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
    upgradedContract.stateRootHashes(finalizationData.endBlockNumber),
    upgradedContract.currentL2BlockNumber(),
    upgradedContract.currentFinalizedState(),
  ]);

  expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
  expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
  expect(lastFinalizedState).to.equal(
    calculateLastFinalizedState(
      finalizationData.l1RollingHashMessageNumber,
      finalizationData.l1RollingHash,
      finalizationData.finalForcedTransactionNumber,
      proofData.finalFtxRollingHash,
      finalizationData.finalTimestamp,
    ),
  );
}
