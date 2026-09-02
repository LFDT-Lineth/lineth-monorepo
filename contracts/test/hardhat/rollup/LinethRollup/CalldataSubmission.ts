import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { STATE_DATA_SUBMISSION_PAUSE_TYPE } from "contracts/common/constants";
import { TestLinethRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { getAccountsFixture, deployLinethRollupFixture } from "./../helpers";
import { GENERAL_PAUSE_TYPE, OPERATOR_ROLE, EMPTY_CALLDATA, MAX_GAS_LIMIT } from "../../common/constants";
import {
  generateRandomBytes,
  generateCallDataSubmission,
  generateCallDataSubmissionWithHashes,
  generateParentAndExpectedDataRollingHashForIndex,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectRevertWhenPaused,
  computeDataRollingHash,
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
  const { parentDataRollingHash: PARENT_ONE, expectedDataRollingHash: EXPECTED_ONE } =
    generateParentAndExpectedDataRollingHashForIndex(0);

  it("Fails when the compressed data is empty", async () => {
    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(EMPTY_CALLDATA, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });
    await expectRevertWithCustomError(linethRollup, submitDataCall, "EmptySubmissionData");
  });

  it("Should fail when the parent dataRollingHash is not anchored", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    const nonExistingParent = generateRandomBytes(32);
    const wrongExpected = computeDataRollingHash(nonExistingParent, ethers.keccak256(submissionData.compressedData));

    const asyncCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData.compressedData, nonExistingParent, wrongExpected, {
        gasLimit: MAX_GAS_LIMIT,
      });

    await expectRevertWithCustomError(linethRollup, asyncCall, "ParentDataRollingHashNotAnchored", [nonExistingParent]);
  });

  it("Should succesfully submit 1 compressed data chunk setting values", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT }),
    ).to.not.be.reverted;

    const dataRollingHashExists = await linethRollup.blobShnarfExists(EXPECTED_ONE);
    expect(dataRollingHashExists).to.equal(1n);
  });

  it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
    const submissions = generateCallDataSubmissionWithHashes(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          submissions[0].compressedData,
          submissions[0].parentDataRollingHash,
          submissions[0].expectedDataRollingHash,
          { gasLimit: MAX_GAS_LIMIT },
        ),
    ).to.not.be.reverted;

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          submissions[1].compressedData,
          submissions[1].parentDataRollingHash,
          submissions[1].expectedDataRollingHash,
          {
            gasLimit: MAX_GAS_LIMIT,
          },
        ),
    ).to.not.be.reverted;

    const dataRollingHashExists = await linethRollup.blobShnarfExists(submissions[0].expectedDataRollingHash);
    expect(dataRollingHashExists).to.equal(1n);
  });

  it("Should emit an event while submitting 1 compressed data chunk", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });
    const eventArgs = [PARENT_ONE, EXPECTED_ONE];

    await expectEvent(linethRollup, submitDataCall, "DataSubmittedV4", eventArgs);
  });

  it("Should fail if the compressed data yields a wrong expected dataRollingHash", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    const wrongCompressedData = generateRandomBytes(64);
    const wrongExpected = computeDataRollingHash(PARENT_ONE, ethers.keccak256(wrongCompressedData));

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData.compressedData, PARENT_ONE, wrongExpected, { gasLimit: MAX_GAS_LIMIT });

    const actualDataRollingHash = computeDataRollingHash(PARENT_ONE, ethers.keccak256(submissionData.compressedData));
    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalDataRollingHashWrong", [
      wrongExpected,
      actualDataRollingHash,
    ]);
  });

  it("Should fail to submit where expected dataRollingHash is wrong", async () => {
    const submissions = generateCallDataSubmissionWithHashes(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(
          submissions[0].compressedData,
          submissions[0].parentDataRollingHash,
          submissions[0].expectedDataRollingHash,
          { gasLimit: MAX_GAS_LIMIT },
        ),
    ).to.not.be.reverted;

    const wrongComputed = generateRandomBytes(32);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissions[1].compressedData, submissions[1].parentDataRollingHash, wrongComputed, {
        gasLimit: MAX_GAS_LIMIT,
      });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalDataRollingHashWrong", [
      wrongComputed,
      submissions[1].expectedDataRollingHash,
    ]);
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const submitDataCall = linethRollup
      .connect(nonAuthorizedAccount)
      .submitDataAsCalldata(DATA_ONE.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

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
        .submitDataAsCalldata(DATA_ONE.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWhenPaused(linethRollup, submitDataCall, pauseType);
    });
  });

  it("Should revert with DataRollingHashAlreadyAnchored when submitting same compressed data twice in 2 separate transactions", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "DataRollingHashAlreadyAnchored", [EXPECTED_ONE]);
  });

  it("Should revert with DataRollingHashAlreadyAnchored when submitting same data twice", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    const [dataOneCopy] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(dataOneCopy.compressedData, PARENT_ONE, EXPECTED_ONE, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "DataRollingHashAlreadyAnchored", [EXPECTED_ONE]);
  });
});
