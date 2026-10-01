import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { STATE_DATA_SUBMISSION_PAUSE_TYPE } from "contracts/common/constants";
import { TestLinethRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { getAccountsFixture, deployLinethRollupFixture } from "./../helpers";
import { GENERAL_PAUSE_TYPE, HASH_ZERO, OPERATOR_ROLE, EMPTY_CALLDATA, MAX_GAS_LIMIT } from "../../common/constants";
import {
  generateRandomBytes,
  generateCallDataSubmission,
  generateCallDataSubmissionWithShnarfs,
  generateParentAndExpectedShnarfForIndex,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectRevertWhenPaused,
  computeShnarfV2,
} from "../../common/helpers";

describe("Lineth Rollup contract: Calldata Submission", () => {
  let linethRollup: TestLinethRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ linethRollup } = await loadFixture(deployLinethRollupFixture));
    await linethRollup.setLastFinalizedBlock(0);
  });

  const [DATA_ONE] = generateCallDataSubmission(0, 1);
  const { parentShnarf: PARENT_ONE, expectedShnarf: EXPECTED_ONE } = generateParentAndExpectedShnarfForIndex(0);

  it("Fails when the compressed data is empty", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    submissionData.compressedData = EMPTY_CALLDATA;

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });
    await expectRevertWithCustomError(linethRollup, submitDataCall, "EmptySubmissionData");
  });

  it("Should fail when the parent shnarf does not exist", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);
    const wrongExpectedShnarf = computeShnarfV2(
      nonExistingParentShnarf,
      submissionData.blockHash,
      ethers.keccak256(submissionData.compressedData),
    );

    const asyncCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, nonExistingParentShnarf, wrongExpectedShnarf, {
        gasLimit: MAX_GAS_LIMIT,
      });

    await expectRevertWithCustomError(linethRollup, asyncCall, "ParentShnarfNotSubmitted", [nonExistingParentShnarf]);
  });

  it("Should succesfully submit 1 compressed data chunk setting values", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT }),
    ).to.not.be.reverted;

    const blobShnarfExists = await linethRollup.blobShnarfExists(EXPECTED_ONE);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
    const submissions = generateCallDataSubmissionWithShnarfs(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          { blockHash: submissions[0].blockHash, compressedData: submissions[0].compressedData },
          submissions[0].parentShnarf,
          submissions[0].expectedShnarf,
          { gasLimit: MAX_GAS_LIMIT },
        ),
    ).to.not.be.reverted;

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          { blockHash: submissions[1].blockHash, compressedData: submissions[1].compressedData },
          submissions[1].parentShnarf,
          submissions[1].expectedShnarf,
          {
            gasLimit: MAX_GAS_LIMIT,
          },
        ),
    ).to.not.be.reverted;

    const blobShnarfExists = await linethRollup.blobShnarfExists(submissions[0].expectedShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should emit an event while submitting 1 compressed data chunk", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });
    const eventArgs = [PARENT_ONE, EXPECTED_ONE, submissionData.blockHash];

    await expectEvent(linethRollup, submitDataCall, "DataSubmittedV4", eventArgs);
  });

  it("Should fail if the block hash yields a wrong expected shnarf", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    submissionData.blockHash = HASH_ZERO;

    const actualShnarf = computeShnarfV2(PARENT_ONE, HASH_ZERO, ethers.keccak256(submissionData.compressedData));

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [EXPECTED_ONE, actualShnarf]);
  });

  it("Should fail to submit where expected shnarf is wrong", async () => {
    const submissions = generateCallDataSubmissionWithShnarfs(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          { blockHash: submissions[0].blockHash, compressedData: submissions[0].compressedData },
          submissions[0].parentShnarf,
          submissions[0].expectedShnarf,
          { gasLimit: MAX_GAS_LIMIT },
        ),
    ).to.not.be.reverted;

    const wrongComputedShnarf = generateRandomBytes(32);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(
        { blockHash: submissions[1].blockHash, compressedData: submissions[1].compressedData },
        submissions[1].parentShnarf,
        wrongComputedShnarf,
        { gasLimit: MAX_GAS_LIMIT },
      );

    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      wrongComputedShnarf,
      submissions[1].expectedShnarf,
    ]);
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const submitDataCall = linethRollup
      .connect(nonAuthorizedAccount)
      .submitDataAsCalldata(DATA_ONE, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
  });

  const calldataSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  calldataSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      await linethRollup.connect(securityCouncil).pauseByType(pauseType);

      const submitDataCall = linethRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWhenPaused(linethRollup, submitDataCall, pauseType);
    });
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [EXPECTED_ONE]);
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same data twice", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    const [dataOneCopy] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(dataOneCopy, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [EXPECTED_ONE]);
  });
});
