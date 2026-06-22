import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { STATE_DATA_SUBMISSION_PAUSE_TYPE } from "contracts/common/constants";
import { TestLinethRollup } from "contracts/typechain-types";

import { getAccountsFixture, deployLinethRollupFixture } from "./../helpers";
import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";
import secondCompressedDataContent from "../../_testData/compressedData/blocks-47-81.json";
import { GENERAL_PAUSE_TYPE, HASH_ZERO, OPERATOR_ROLE, EMPTY_CALLDATA, MAX_GAS_LIMIT } from "../../common/constants";
import {
  generateRandomBytes,
  generateCallDataSubmission,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectRevertWhenPaused,
  generateKeccak256,
} from "../../common/helpers";
import { CalldataSubmissionData } from "../../common/types";

describe("Linea Rollup contract: Calldata Submission", () => {
  let linethRollup: TestLinethRollup;

  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  const { prevShnarf, expectedShnarf } = firstCompressedDataContent;
  const { expectedShnarf: secondExpectedShnarf } = secondCompressedDataContent;

  before(async () => {
    ({ securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ linethRollup } = await loadFixture(deployLinethRollupFixture));
    await linethRollup.setLastFinalizedBlock(0);
  });

  const [DATA_ONE] = generateCallDataSubmission(0, 1);

  it("Fails when the compressed data is empty", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    submissionData.compressedData = EMPTY_CALLDATA;

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, prevShnarf, secondExpectedShnarf, { gasLimit: MAX_GAS_LIMIT });
    await expectRevertWithCustomError(linethRollup, submitDataCall, "EmptySubmissionData");
  });

  it("Should fail when the parent shnarf does not exist", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);
    const nonExistingParentShnarf = generateRandomBytes(32);

    const wrongExpectedShnarf = generateKeccak256(
      ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
      [HASH_ZERO, HASH_ZERO, submissionData.finalStateRootHash, HASH_ZERO, HASH_ZERO],
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
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
    ).to.not.be.reverted;

    const blobShnarfExists = await linethRollup.blobShnarfExists(expectedShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
    const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
    ).to.not.be.reverted;

    await expect(
      linethRollup.connect(operator).submitDataAsCalldata(secondSubmissionData, expectedShnarf, secondExpectedShnarf, {
        gasLimit: MAX_GAS_LIMIT,
      }),
    ).to.not.be.reverted;

    const blobShnarfExists = await linethRollup.blobShnarfExists(expectedShnarf);
    expect(blobShnarfExists).to.equal(1n);
  });

  it("Should emit an event while submitting 1 compressed data chunk", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });
    const eventArgs = [prevShnarf, expectedShnarf, submissionData.finalStateRootHash];

    await expectEvent(linethRollup, submitDataCall, "DataSubmittedV3", eventArgs);
  });

  it("Should fail if the final state root hash is empty", async () => {
    const [submissionData] = generateCallDataSubmission(0, 1);

    submissionData.finalStateRootHash = HASH_ZERO;

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      expectedShnarf,
      "0xd14582c1b7041f523ac1428657196f30d5260227668151e660840fb629cacba4",
    ]);
  });

  it("Should fail to submit where expected shnarf is wrong", async () => {
    const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

    await expect(
      linethRollup
        .connect(operator)
        .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
    ).to.not.be.reverted;

    const wrongComputedShnarf = generateRandomBytes(32);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(secondSubmissionData, expectedShnarf, wrongComputedShnarf, { gasLimit: MAX_GAS_LIMIT });

    const eventArgs = [wrongComputedShnarf, secondExpectedShnarf];

    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", eventArgs);
  });

  it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
    const submitDataCall = linethRollup
      .connect(nonAuthorizedAccount)
      .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
  });

  // Parameterized pause type tests for calldata submission
  const calldataSubmissionPauseTypes = [
    { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
    { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
  ];

  calldataSubmissionPauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      await linethRollup.connect(securityCouncil).pauseByType(pauseType);

      const submitDataCall = linethRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWhenPaused(linethRollup, submitDataCall, pauseType);
    });
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
  });

  it("Should revert with ShnarfAlreadySubmitted when submitting same data, differing block numbers", async () => {
    await linethRollup
      .connect(operator)
      .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    const [dataOneCopy] = generateCallDataSubmission(0, 1);

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(dataOneCopy, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    await expectRevertWithCustomError(linethRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
  });

  it("Should revert when snarkHash is zero hash", async () => {
    const submissionData: CalldataSubmissionData = {
      ...DATA_ONE,
      snarkHash: HASH_ZERO,
    };

    const submitDataCall = linethRollup
      .connect(operator)
      .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

    // TODO: Make the failure shnarf dynamic and computed
    await expectRevertWithCustomError(linethRollup, submitDataCall, "FinalShnarfWrong", [
      expectedShnarf,
      "0xa6b52564082728b51bb81a4fa92cfb4ec3af8de3f18b5d68ec27b89eead93293",
    ]);
  });
});
