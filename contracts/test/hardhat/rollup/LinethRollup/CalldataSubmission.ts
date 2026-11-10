import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import { getAccountsFixture, deployLinethRollupFixture } from "./../helpers/index.js";
import { STATE_DATA_SUBMISSION_PAUSE_TYPE } from "../../../../common/constants/index.js";
import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json" with { type: "json" };
import secondCompressedDataContent from "../../_testData/compressedData/blocks-47-81.json" with { type: "json" };
import {
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  OPERATOR_ROLE,
  EMPTY_CALLDATA,
  MAX_GAS_LIMIT,
} from "../../common/constants/index.js";
import {
  type TestAccount,
  hex,
  generateRandomBytes,
  generateCallDataSubmission,
  buildAccessErrorMessage,
  expectRevertWhenPaused,
  generateKeccak256,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
} from "../../common/helpers/index.js";

import type { CalldataSubmissionData } from "../../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Lineth Rollup contract: Calldata Submission", () => {
  let linethRollup: ContractReturnType<"TestLinethRollup">;

  let securityCouncil: TestAccount;
  let operator: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  const prevShnarf = hex(firstCompressedDataContent.prevShnarf);
  const expectedShnarf = hex(firstCompressedDataContent.expectedShnarf);
  const secondExpectedShnarf = hex(secondCompressedDataContent.expectedShnarf);

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ securityCouncil, operator, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
    await linethRollup.write.setLastFinalizedBlock([0n]);
  });

  const [DATA_ONE] = generateCallDataSubmission(0, 1);

  it("Fails when the compressed data is empty", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    submissionData.compressedData = EMPTY_CALLDATA;

    const submitDataCall = linethRollup.write.submitDataAsCalldata([submissionData, prevShnarf, secondExpectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });
    await expectRevertWithCustomError(linethRollup, submitDataCall, "EmptySubmissionData");
  });

  it("Should fail when the parent shnarf does not exist", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);

    const wrongExpectedShnarf = generateKeccak256(
      ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
      [HASH_ZERO, HASH_ZERO, submissionData.finalStateRootHash, HASH_ZERO, HASH_ZERO],
    );

    const asyncCall = linethRollup.write.submitDataAsCalldata(
      [submissionData, nonExistingParentShnarf, wrongExpectedShnarf],
      { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
    );

    await expectRevertWithCustomError(linethRollup, asyncCall, "ParentShnarfNotSubmitted", [nonExistingParentShnarf]);
  });

  it("Should succesfully submit 1 compressed data chunk setting values", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    await linethRollup.write.submitDataAsCalldata([submissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    const blobShnarfExists = await linethRollup.read.blobShnarfExists([expectedShnarf]);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
    const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

    await linethRollup.write.submitDataAsCalldata([firstSubmissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    await linethRollup.write.submitDataAsCalldata([secondSubmissionData, expectedShnarf, secondExpectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    const blobShnarfExists = await linethRollup.read.blobShnarfExists([expectedShnarf]);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should emit an event while submitting 1 compressed data chunk", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup.write.submitDataAsCalldata([submissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    await expectEvent(linethRollup, submitDataCall, "DataSubmittedV3", [
      prevShnarf,
      expectedShnarf,
      submissionData.finalStateRootHash,
    ]);
  });

  it("Should fail if the final state root hash is empty", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    submissionData.finalStateRootHash = HASH_ZERO;

    const submitDataCall = linethRollup.write.submitDataAsCalldata([submissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      expectedShnarf,
      "0xd14582c1b7041f523ac1428657196f30d5260227668151e660840fb629cacba4",
    ]);
  });

  it("Should fail to submit where expected shnarf is wrong", async () => {
    const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

    await linethRollup.write.submitDataAsCalldata([firstSubmissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    const wrongComputedShnarf = generateRandomBytes(32);

    const submitDataCall = linethRollup.write.submitDataAsCalldata(
      [secondSubmissionData, expectedShnarf, wrongComputedShnarf],
      { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
    );

    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      wrongComputedShnarf,
      secondExpectedShnarf,
    ]);
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const submitDataCall = linethRollup.write.submitDataAsCalldata([DATA_ONE, prevShnarf, expectedShnarf], {
      account: nonAuthorizedAccount.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
  });

  // Parameterized pause type tests for calldata submission
  const calldataSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  calldataSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      await linethRollup.write.pauseByType([pauseType], { account: securityCouncil.account });

      const submitDataCall = linethRollup.write.submitDataAsCalldata([DATA_ONE, prevShnarf, expectedShnarf], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      await expectRevertWhenPaused(linethRollup, submitDataCall, pauseType);
    });
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
    await linethRollup.write.submitDataAsCalldata([DATA_ONE, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    const submitDataCall = linethRollup.write.submitDataAsCalldata([DATA_ONE, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same data, differing block numbers", async () => {
    await linethRollup.write.submitDataAsCalldata([DATA_ONE, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    const [dataOneCopy] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup.write.submitDataAsCalldata([dataOneCopy, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
  });

  it("Should revert when snarkHash is zero hash", async () => {
    const submissionData: CalldataSubmissionData = {
      ...DATA_ONE,
      snarkHash: HASH_ZERO,
    };

    const submitDataCall = linethRollup.write.submitDataAsCalldata([submissionData, prevShnarf, expectedShnarf], {
      account: operator.account,
      gas: BigInt(MAX_GAS_LIMIT),
    });

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      expectedShnarf,
      "0xa6b52564082728b51bb81a4fa92cfb4ec3af8de3f18b5d68ec27b89eead93293",
    ]);
  });
});
