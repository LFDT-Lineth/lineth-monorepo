import { expect } from "chai";
import { BaseContract, Transaction } from "ethers";
import { ethers } from "hardhat";

import { getWalletForIndex } from "./";
import { FailedFinalizeParams, SucceedFinalizeParams, SucceedFinalizeParamsCallForwardingProxy } from "./type";
import { TEST_PUBLIC_VERIFIER_INDEX } from "../../common/constants";
import {
  calculateLastFinalizedState,
  computeShnarfV2,
  expectEvent,
  expectEventDirectFromReceiptData,
  expectRevertWithCustomError,
  generateFinalizationData,
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

  const expectedFinalShnarf = computeShnarfV2(
    finalizationData.shnarfData.parentShnarf,
    finalizationData.finalBlockHash,
    finalizationData.finalBlobHash,
  );

  await expectEvent(lineaRollup, finalizeCompressedCall, "FinalizedStateUpdated", [
    finalizationData.endBlockNumber,
    finalizationData.finalTimestamp,
    finalizationData.l1RollingHashMessageNumber,
    finalizationData.finalForcedTransactionNumber,
  ]);

  await expectEvent(lineaRollup, finalizeCompressedCall, "DataFinalizedV4", [
    BigInt(proofData.lastFinalizedBlockNumber) + 1n,
    finalizationData.endBlockNumber,
    expectedFinalShnarf,
    finalizationData.parentBlockHash,
    finalizationData.finalBlockHash,
  ]);

  const [expectedFinalBlockHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
    lineaRollup.blockHashes(finalizationData.endBlockNumber),
    lineaRollup.currentL2BlockNumber(),
    lineaRollup.currentFinalizedState(),
  ]);

  expect(expectedFinalBlockHash).to.equal(finalizationData.finalBlockHash);
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
  const expectedFinalShnarf = computeShnarfV2(
    shnarfData.parentShnarf,
    finalizationData.finalBlockHash,
    finalizationData.finalBlobHash,
  );
  const blobShnarfExists = await upgradedContract.blobShnarfExists(expectedFinalShnarf);
  expect(blobShnarfExists).to.equal(1n);

  await upgradedContract.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

  const txData = [
    proofData.aggregatedProof,
    0,
    [
      finalizationData.parentStateRootHash,
      finalizationData.parentBlockHash,
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
      finalizationData.finalBlockHash,
      finalizationData.finalBlobHash,
      finalizationData.l2MerkleRoots,
      finalizationData.filteredAddresses,
      finalizationData.verifierKeys,
      finalizationData.l2MessagingBlocksOffsets,
    ],
  ];

  const encodedCall = ethers.concat([
    "0x8da8b592",
    ethers.AbiCoder.defaultAbiCoder().encode(
      [
        "bytes",
        "uint256",
        "tuple(bytes32,bytes32,uint256,tuple(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,uint256,uint256,bytes32,bytes32,bytes32,bytes32[],address[],bytes32[],bytes)",
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
    "DataFinalizedV4",
    [
      BigInt(proofData.lastFinalizedBlockNumber) + 1n,
      finalizationData.endBlockNumber,
      expectedFinalShnarf,
      finalizationData.parentBlockHash,
      finalizationData.finalBlockHash,
    ],
    dataFinalizedLogIndex,
  );

  const [expectedFinalBlockHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
    upgradedContract.blockHashes(finalizationData.endBlockNumber),
    upgradedContract.currentL2BlockNumber(),
    upgradedContract.currentFinalizedState(),
  ]);

  expect(expectedFinalBlockHash).to.equal(finalizationData.finalBlockHash);
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
