import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { config, ethers, upgrades } from "hardhat";
import { HardhatNetworkHDAccountsConfig } from "hardhat/types";
import { BaseContract, Contract, HDNodeWallet, Transaction, Wallet, ZeroAddress } from "ethers";
import { CallForwardingProxy, TestLineaRollup, TestLineaRollupV5 } from "../typechain-types";
import calldataAggregatedProof1To155 from "./testData/compressedData/aggregatedProof-1-155.json";
import blobAggregatedProof1To155 from "./testData/compressedDataEip4844/aggregatedProof-1-155.json";
import blobMultipleAggregatedProof1To81 from "./testData/compressedDataEip4844/multipleProofs/aggregatedProof-1-81.json";
import firstCompressedDataContent from "./testData/compressedData/blocks-1-46.json";
import secondCompressedDataContent from "./testData/compressedData/blocks-47-81.json";
import fourthCompressedDataContent from "./testData/compressedData/blocks-115-155.json";
import fourthMultipleCompressedDataContent from "./testData/compressedData/multipleProofs/blocks-120-153.json";
import {
  ADDRESS_ZERO,
  GENERAL_PAUSE_TYPE,
  HASH_WITHOUT_ZERO_FIRST_BYTE,
  HASH_ZERO,
  INITIAL_MIGRATION_BLOCK,
  INITIAL_WITHDRAW_LIMIT,
  ONE_DAY_IN_SECONDS,
  OPERATOR_ROLE,
  TEST_PUBLIC_VERIFIER_INDEX,
  VERIFIER_SETTER_ROLE,
  VERIFIER_UNSETTER_ROLE,
  GENESIS_L2_TIMESTAMP,
  EMPTY_CALLDATA,
  INITIALIZED_ALREADY_MESSAGE,
  CALLDATA_SUBMISSION_PAUSE_TYPE,
  BLOB_SUBMISSION_PAUSE_TYPE,
  FINALIZATION_PAUSE_TYPE,
  PAUSE_ALL_ROLE,
  DEFAULT_ADMIN_ROLE,
  UNPAUSE_ALL_ROLE,
  PAUSE_BLOB_SUBMISSION_ROLE,
  UNPAUSE_BLOB_SUBMISSION_ROLE,
  PAUSE_FINALIZATION_ROLE,
  UNPAUSE_FINALIZATION_ROLE,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  SIX_MONTHS_IN_SECONDS,
  USED_RATE_LIMIT_RESETTER_ROLE,
  PAUSE_L1_L2_ROLE,
  PAUSE_L2_L1_ROLE,
  UNPAUSE_L1_L2_ROLE,
  UNPAUSE_L2_L1_ROLE,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
} from "./common/constants";
import { deployUpgradableFromFactory } from "./common/deployment";
import {
  calculateRollingHash,
  encodeData,
  generateFinalizationData,
  generateRandomBytes,
  generateCallDataSubmission,
  generateCallDataSubmissionMultipleProofs,
  generateKeccak256,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateParentAndExpectedShnarfForIndex,
  generateParentAndExpectedShnarfForMulitpleIndex,
  generateParentShnarfData,
  generateBlobDataSubmission,
  generateBlobParentShnarfData,
  convertStringToPaddedHexBytes,
  calculateLastFinalizedState,
  expectEvents,
  expectEventDirectFromReceiptData,
} from "./common/helpers";
import { CalldataSubmissionData, ShnarfDataGenerator } from "./common/types";
import aggregatedProof1To81 from "./testData/compressedData/multipleProofs/aggregatedProof-1-81.json";
import aggregatedProof82To153 from "./testData/compressedData/multipleProofs/aggregatedProof-82-153.json";
import * as kzg from "c-kzg";
import {
  LINEA_ROLLUP_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_ROLES,
  LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
} from "contracts/common/constants";
import { generateRoleAssignments } from "contracts/common/helpers";

kzg.loadTrustedSetup(`${__dirname}/testData/trusted_setup.txt`);

describe("Linea Rollup contract", () => {
  let lineaRollup: TestLineaRollup;
  let lineaRollupV5: TestLineaRollupV5;
  let revertingVerifier: string;
  let callForwardingProxy: CallForwardingProxy;

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let verifier: string;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];

  const fallbackoperatorAddress = "0xcA11bde05977b3631167028862bE2a173976CA11";

  const { compressedData, prevShnarf, expectedShnarf, expectedX, expectedY, parentDataHash, parentStateRootHash } =
    firstCompressedDataContent;
  const { expectedShnarf: secondExpectedShnarf } = secondCompressedDataContent;

  async function deployRevertingVerifier(scenario: bigint) {
    const revertingVerifierFactory = await ethers.getContractFactory("RevertingVerifier");
    const verifier = await revertingVerifierFactory.deploy(scenario);
    await verifier.waitForDeployment();
    revertingVerifier = await verifier.getAddress();
  }

  async function deployCallForwardingProxy(target: string) {
    const callForwardingProxyFactory = await ethers.getContractFactory("CallForwardingProxy");
    callForwardingProxy = await callForwardingProxyFactory.deploy(target);
    return await callForwardingProxy.waitForDeployment();
  }

  async function deployLineaRollupFixture() {
    const plonkVerifierFactory = await ethers.getContractFactory("TestPlonkVerifierForDataAggregation");
    const plonkVerifier = await plonkVerifierFactory.deploy();
    await plonkVerifier.waitForDeployment();

    verifier = await plonkVerifier.getAddress();

    const initializationData = {
      initialStateRootHash: parentStateRootHash,
      initialL2BlockNumber: 0,
      genesisTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
      defaultVerifier: verifier,
      rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
      rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
      roleAddresses,
      pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
      unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
      fallbackOperator: fallbackoperatorAddress,
      defaultAdmin: securityCouncil.address,
    };

    const lineaRollup = (await deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
      initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor"],
    })) as unknown as TestLineaRollup;

    return lineaRollup;
  }

  const getWalletForIndex = (index: number) => {
    const accounts = config.networks.hardhat.accounts as HardhatNetworkHDAccountsConfig;
    const signer = HDNodeWallet.fromPhrase(accounts.mnemonic, "", `m/44'/60'/0'/0/${index}`);
    return new Wallet(signer.privateKey, ethers.provider);
  };

  before(async () => {
    [admin, securityCouncil, operator, nonAuthorizedAccount] = await ethers.getSigners();
    const securityCouncilAddress = securityCouncil.address;

    roleAddresses = generateRoleAssignments(LINEA_ROLLUP_ROLES, securityCouncilAddress, [
      {
        role: OPERATOR_ROLE,
        addresses: [operator.address],
      },
    ]);
  });

  beforeEach(async () => {
    lineaRollup = await loadFixture(deployLineaRollupFixture);
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: string) => {
      return admin.sendTransaction({ to: await lineaRollup.getAddress(), value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the lineaRollup contract through the fallback", async () => {
      await expect(sendEthToContract(EMPTY_CALLDATA)).to.be.reverted;
    });

    it("Should fail to send eth to the lineaRollup contract through the receive function", async () => {
      await expect(sendEthToContract("0x1234")).to.be.reverted;
    });
  });

  describe("Initialisation", () => {
    it("Should revert if verifier address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: ADDRESS_ZERO,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: securityCouncil.address,
      };

      const deployCall = deployUpgradableFromFactory("contracts/LineaRollup.sol:LineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if the fallback operator address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses.slice(1)],
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: ADDRESS_ZERO,
        defaultAdmin: securityCouncil.address,
      };

      const deployCall = deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if the default admin address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses.slice(1)],
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if an operator address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [{ addressWithRole: ADDRESS_ZERO, role: DEFAULT_ADMIN_ROLE }, ...roleAddresses.slice(1)],
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: securityCouncil.address,
      };

      const deployCall = deployUpgradableFromFactory("TestLineaRollup", [initializationData], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should store verifier address in storage", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      expect(await lineaRollup.verifiers(0)).to.be.equal(verifier);
    });

    it("Should assign the OPERATOR_ROLE to operator addresses", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      expect(await lineaRollup.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the VERIFIER_SETTER_ROLE to securityCouncil addresses", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
    });

    it("Should assign the VERIFIER_UNSETTER_ROLE to securityCouncil addresses", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      expect(await lineaRollup.hasRole(VERIFIER_UNSETTER_ROLE, securityCouncil.address)).to.be.true;
    });

    it("Should store the startingRootHash in storage for the first block number", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: securityCouncil.address,
      };

      const lineaRollup = await deployUpgradableFromFactory(
        "contracts/LineaRollup.sol:LineaRollup",
        [initializationData],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );

      expect(await lineaRollup.stateRootHashes(INITIAL_MIGRATION_BLOCK)).to.be.equal(parentStateRootHash);
    });

    it("Should assign the VERIFIER_SETTER_ROLE to both SecurityCouncil and Operator", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: securityCouncil.address,
      };

      const lineaRollup = await deployUpgradableFromFactory(
        "contracts/LineaRollup.sol:LineaRollup",
        [initializationData],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );

      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, operator.address)).to.be.true;
    });

    it("Should revert if the initialize function is called a second time", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      const initializeCall = lineaRollup.initialize({
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackOperator: fallbackoperatorAddress,
        defaultAdmin: securityCouncil.address,
      });

      await expectRevertWithReason(initializeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Change verifier address", () => {
    it("Should revert if the caller has not the VERIFIER_SETTER_ROLE", async () => {
      const setVerifierCall = lineaRollup.connect(nonAuthorizedAccount).setVerifierAddress(verifier, 2);

      await expectRevertWithReason(
        setVerifierCall,
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_SETTER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      await lineaRollup.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      const setVerifierCall = lineaRollup.connect(securityCouncil).setVerifierAddress(ADDRESS_ZERO, 2);
      await expectRevertWithCustomError(lineaRollup, setVerifierCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new verifier address", async () => {
      await lineaRollup.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      await lineaRollup.connect(securityCouncil).setVerifierAddress(verifier, 2);
      expect(await lineaRollup.verifiers(2)).to.be.equal(verifier);
    });

    it("Should remove verifier address in storage ", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);
      await lineaRollup.connect(securityCouncil).unsetVerifierAddress(0);

      expect(await lineaRollup.verifiers(0)).to.be.equal(ADDRESS_ZERO);
    });

    it("Should revert when removing verifier address if the caller has not the VERIFIER_UNSETTER_ROLE ", async () => {
      lineaRollup = await loadFixture(deployLineaRollupFixture);

      await expect(lineaRollup.connect(nonAuthorizedAccount).unsetVerifierAddress(0)).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_UNSETTER_ROLE),
      );
    });

    it("Should emit the correct event", async () => {
      await lineaRollup.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      const oldVerifierAddress = await lineaRollup.verifiers(2);

      const setVerifierCall = lineaRollup.connect(securityCouncil).setVerifierAddress(verifier, 2);
      let expectedArgs = [verifier, 2, securityCouncil.address, oldVerifierAddress];

      await expectEvent(lineaRollup, setVerifierCall, "VerifierAddressChanged", expectedArgs);

      await lineaRollup.connect(securityCouncil).unsetVerifierAddress(2);

      const unsetVerifierCall = lineaRollup.connect(securityCouncil).unsetVerifierAddress(2);
      expectedArgs = [ADDRESS_ZERO, 2, securityCouncil.address, oldVerifierAddress];

      await expectEvent(lineaRollup, unsetVerifierCall, "VerifierAddressChanged", expectedArgs);
    });
  });

  describe("Data submission tests", () => {
    beforeEach(async () => {
      await lineaRollup.setLastFinalizedBlock(0);
    });

    const [DATA_ONE] = generateCallDataSubmission(0, 1);

    it("Fails when the compressed data is empty", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);
      submissionData.compressedData = EMPTY_CALLDATA;

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, secondExpectedShnarf, { gasLimit: 30_000_000 });
      await expectRevertWithCustomError(lineaRollup, submitDataCall, "EmptySubmissionData");
    });

    it("Should fail when the parent shnarf does not exist", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);
      const nonExistingParentShnarf = generateRandomBytes(32);
      const asyncCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, nonExistingParentShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithCustomError(lineaRollup, asyncCall, "ParentBlobNotSubmitted", [nonExistingParentShnarf]);
    });

    it("Should succesfully submit 1 compressed data chunk setting values", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      await expect(
        lineaRollup
          .connect(operator)
          .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 }),
      ).to.not.be.reverted;

      const blobShnarfExists = await lineaRollup.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await expect(
        lineaRollup
          .connect(operator)
          .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 }),
      ).to.not.be.reverted;

      await expect(
        lineaRollup.connect(operator).submitDataAsCalldata(secondSubmissionData, expectedShnarf, secondExpectedShnarf, {
          gasLimit: 30_000_000,
        }),
      ).to.not.be.reverted;

      const blobShnarfExists = await lineaRollup.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should emit an event while submitting 1 compressed data chunk", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });
      const eventArgs = [prevShnarf, expectedShnarf, submissionData.finalStateRootHash];

      await expectEvent(lineaRollup, submitDataCall, "DataSubmittedV3", eventArgs);
    });

    it("Should fail if the final state root hash is empty", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      submissionData.finalStateRootHash = HASH_ZERO;

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(lineaRollup, submitDataCall, "FinalShnarfWrong", [
        expectedShnarf,
        "0xf53c28b2287f506b4df1b9de48cf3601392d54a73afe400a6f8f4ded2e0929ad",
      ]);
    });

    it("Should fail to submit where expected shnarf is wrong", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await expect(
        lineaRollup
          .connect(operator)
          .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 }),
      ).to.not.be.reverted;

      const wrongComputedShnarf = generateRandomBytes(32);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(secondSubmissionData, expectedShnarf, wrongComputedShnarf, { gasLimit: 30_000_000 });

      const eventArgs = [wrongComputedShnarf, secondExpectedShnarf];

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "FinalShnarfWrong", eventArgs);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const submitDataCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if CALLDATA_SUBMISSION_PAUSE_TYPE is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(CALLDATA_SUBMISSION_PAUSE_TYPE);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "IsPaused", [CALLDATA_SUBMISSION_PAUSE_TYPE]);
    });

    it("Should revert with DataAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
      await lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "DataAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert with DataAlreadySubmitted when submitting same data, differing block numbers", async () => {
      await lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      const [dataOneCopy] = generateCallDataSubmission(0, 1);
      dataOneCopy.endBlockNumber = 234253242n;

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(dataOneCopy, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "DataAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert with SnarkHashIsZeroHash when snarkHash is zero hash", async () => {
      const submissionData: CalldataSubmissionData = {
        ...DATA_ONE,
        snarkHash: HASH_ZERO,
      };

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: 30_000_000 });

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(lineaRollup, submitDataCall, "FinalShnarfWrong", [
        expectedShnarf,
        "0xa6b52564082728b51bb81a4fa92cfb4ec3af8de3f18b5d68ec27b89eead93293",
      ]);
    });
  });

  describe("EIP-4844 Blob submission tests", () => {
    beforeEach(async () => {
      await lineaRollup.setLastFinalizedBlock(0);
      await lineaRollup.setupParentShnarf(prevShnarf);
      await lineaRollup.setupParentDataShnarf(parentDataHash, prevShnarf);
      await lineaRollup.setupParentFinalizedStateRoot(parentDataHash, parentStateRootHash);
    });

    it("Should successfully submit blobs", async () => {
      const operatorHDSigner = getWalletForIndex(2);
      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      const txResponse = await ethers.provider.broadcastTransaction(signedTx);
      const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);
      expect(receipt).is.not.null;

      const expectedEventArgs = [
        parentShnarf,
        finalShnarf,
        blobDataSubmission[blobDataSubmission.length - 1].finalStateRootHash,
      ];

      expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);

      const blobShnarfExists = await lineaRollup.blobShnarfExists(finalShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Fails the blob submission when the parent shnarf is missing", async () => {
      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, finalShnarf } = generateBlobDataSubmission(0, 1);
      const nonExistingParentShnarf = generateRandomBytes(32);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        nonExistingParentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "ParentBlobNotSubmitted",
        [nonExistingParentShnarf],
      );
    });

    it("Fails when the blob submission data is missing", async () => {
      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [[], parentShnarf, finalShnarf]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "BlobSubmissionDataIsMissing",
      );
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      await expectRevertWithReason(
        lineaRollup.connect(nonAuthorizedAccount).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
        buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE),
      );
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.connect(operator).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert if BLOB_SUBMISSION_PAUSE_TYPE is enabled", async () => {
      const { blobDataSubmission, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      await lineaRollup.connect(securityCouncil).pauseByType(BLOB_SUBMISSION_PAUSE_TYPE);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.connect(operator).submitBlobs(blobDataSubmission, parentShnarf, finalShnarf),
        "IsPaused",
        [BLOB_SUBMISSION_PAUSE_TYPE],
      );
    });

    it("Should revert if the blob data is empty at any index", async () => {
      const operatorHDSigner = getWalletForIndex(2);
      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: [compressedBlobs[0]],
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "EmptyBlobDataAtIndex",
        [1n],
      );
    });
    it("Should fail if the final state root hash is empty", async () => {
      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      blobDataSubmission[0].finalStateRootHash = HASH_ZERO;

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "FinalShnarfWrong",
        [finalShnarf, "0x22f8fb954df8328627fe9c48b60192f4d970a92891417aaadea39300ca244d36"],
      );
    });

    it("Should revert with SnarkHashIsZeroHash when snarkHash is zero hash", async () => {
      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      // Set the snarkHash to HASH_ZERO for a specific index
      const emptyDataIndex = 0;
      blobDataSubmission[emptyDataIndex].snarkHash = generateRandomBytes(32);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "PointEvaluationFailed",
      );
    });

    it("Should revert if the final shnarf is wrong", async () => {
      const operatorHDSigner = getWalletForIndex(2);
      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 2);
      const badFinalShnarf = generateRandomBytes(32);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        badFinalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "FinalShnarfWrong",
        [badFinalShnarf, finalShnarf],
      );
    });

    it("Should revert if the data has already been submitted", async () => {
      await sendBlobTransaction(0, 1);

      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      // Try to submit the same blob data again
      const encodedCall2 = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas: maxFeePerGas2, maxPriorityFeePerGas: maxPriorityFeePerGas2 } =
        await ethers.provider.getFeeData();
      const nonce2 = await operatorHDSigner.getNonce();

      const transaction2 = Transaction.from({
        data: encodedCall2,
        maxPriorityFeePerGas: maxPriorityFeePerGas2!,
        maxFeePerGas: maxFeePerGas2!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce: nonce2,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx2 = await operatorHDSigner.signTransaction(transaction2);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx2),
        "DataAlreadySubmitted",
        [finalShnarf],
      );
    });

    it("Should revert with PointEvaluationFailed when point evaluation fails", async () => {
      const operatorHDSigner = getWalletForIndex(2);

      const lineaRollupAddress = await lineaRollup.getAddress();
      const { blobDataSubmission, compressedBlobs, parentShnarf, finalShnarf } = generateBlobDataSubmission(0, 1);

      // Modify the kzgProof to an invalid value to trigger the PointEvaluationFailed revert
      blobDataSubmission[0].kzgProof = HASH_ZERO;

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        blobDataSubmission,
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);

      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "PointEvaluationFailed",
      );
    });

    it("Should submit 2 blobs, then submit another 2 blobs and finalize", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(2, 4);
      // Finalize 4 blobs
      await expectSuccessfulFinalize(
        blobAggregatedProof1To155,
        4,
        fourthCompressedDataContent.finalStateRootHash,
        generateBlobParentShnarfData,
      );
    });

    it("Should revert if there is less data than blobs", async () => {
      const operatorHDSigner = getWalletForIndex(2);
      const lineaRollupAddress = await lineaRollup.getAddress();

      const {
        blobDataSubmission: blobSubmission,
        compressedBlobs: compressedBlobs,
        parentShnarf: parentShnarf,
        finalShnarf: finalShnarf,
      } = generateBlobDataSubmission(0, 2, true);

      const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
        [blobSubmission[0]],
        parentShnarf,
        finalShnarf,
      ]);

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
      const nonce = await operatorHDSigner.getNonce();

      const transaction = Transaction.from({
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: (await ethers.provider.getNetwork()).chainId,
        type: 3,
        nonce: nonce,
        value: 0,
        gasLimit: 5_000_000,
        kzg,
        maxFeePerBlobGas: 1n,
        blobs: compressedBlobs,
      });

      const signedTx = await operatorHDSigner.signTransaction(transaction);
      await expectRevertWithCustomError(
        lineaRollup,
        ethers.provider.broadcastTransaction(signedTx),
        "BlobSubmissionDataEmpty",
        [1],
      );
    });

    it("Should fail to finalize with not enough gas for the rollup (pre-verifier)", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(2, 4);

      // Finalize 4 blobs
      const finalizationData = await generateFinalizationData({
        l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
        shnarfData: generateBlobParentShnarfData(4, false),
      });
      finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

      await lineaRollup.setRollingHash(
        blobAggregatedProof1To155.l1RollingHashMessageNumber,
        blobAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCompressedCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
          gasLimit: 50000,
        });

      // there is no reason
      await expect(finalizeCompressedCall).to.be.reverted;
    });

    it("Should fail to finalize with not enough gas to verify", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(0, 2);
      // Submit another 2 blobs
      await sendBlobTransaction(2, 4);

      // Finalize 4 blobs
      const finalizationData = await generateFinalizationData({
        l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
        shnarfData: generateBlobParentShnarfData(4, false),
      });
      finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

      await lineaRollup.setRollingHash(
        blobAggregatedProof1To155.l1RollingHashMessageNumber,
        blobAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCompressedCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
          gasLimit: 400000,
        });

      await expectRevertWithCustomError(
        lineaRollup,
        finalizeCompressedCall,
        "InvalidProofOrProofVerificationRanOutOfGas",
        ["error pairing"],
      );
    });

    const testCases = [
      { revertScenario: 0n, title: "Should fail to finalize via EMPTY_REVERT scenario with 'Unknown'" },
      { revertScenario: 1n, title: "Should fail to finalize via GAS_GUZZLE scenario with 'Unknown'" },
    ];

    testCases.forEach(({ revertScenario, title }) => {
      it(title, async () => {
        await deployRevertingVerifier(revertScenario);
        await lineaRollup.connect(securityCouncil).setVerifierAddress(revertingVerifier, 0);

        // Submit 2 blobs
        await sendBlobTransaction(0, 2);
        // Submit another 2 blobs
        await sendBlobTransaction(2, 4);

        // Finalize 4 blobs
        const finalizationData = await generateFinalizationData({
          l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
          l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
          lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
          endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: blobAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
          shnarfData: generateBlobParentShnarfData(4, false),
        });
        finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
        finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

        await lineaRollup.setRollingHash(
          blobAggregatedProof1To155.l1RollingHashMessageNumber,
          blobAggregatedProof1To155.l1RollingHash,
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
            gasLimit: 400000,
          });

        await expectRevertWithCustomError(
          lineaRollup,
          finalizeCompressedCall,
          "InvalidProofOrProofVerificationRanOutOfGas",
          ["Unknown"],
        );
      });
    });

    it("Should successfully submit 2 blobs twice then finalize in two separate finalizations", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(2, 4, true);
      // Finalize first 2 blobs
      await expectSuccessfulFinalize(
        blobMultipleAggregatedProof1To81,
        2,
        secondCompressedDataContent.finalStateRootHash,
        generateBlobParentShnarfData,
        true,
      );
    });

    it("Should fail to prove if last finalized is higher than proving range", async () => {
      // Submit 2 blobs
      await sendBlobTransaction(0, 2, true);
      // Submit another 2 blobs
      await sendBlobTransaction(2, 4, true);

      await lineaRollup.setLastFinalizedBlock(10_000_000);

      const finalizationData = await generateFinalizationData({
        l1RollingHash: blobAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(blobAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(blobAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(blobAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: HASH_ZERO, // Manipulate for bypass
        finalTimestamp: BigInt(blobAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: blobAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(blobAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: blobAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: blobAggregatedProof1To155.aggregatedProof,
        shnarfData: generateBlobParentShnarfData(4, false),
        lastFinalizedL1RollingHash: HASH_ZERO,
        lastFinalizedL1RollingHashMessageNumber: 0n,
      });

      await lineaRollup.setRollingHash(
        blobAggregatedProof1To155.l1RollingHashMessageNumber,
        blobAggregatedProof1To155.l1RollingHash,
      );

      await lineaRollup.setLastFinalizedBlock(10_000_000);

      expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(operator)
          .finalizeBlocks(blobAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData),
        "InvalidProof",
      );
    });
  });

  describe("Blocks finalization with proof", () => {
    const messageHash = generateRandomBytes(32);

    beforeEach(async () => {
      await lineaRollup.addRollingHash(10, messageHash);
      await lineaRollup.setLastFinalizedBlock(0);
    });

    describe("With and without submission data", () => {
      it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 0n,
          l1RollingHash: generateRandomBytes(32),
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "MissingMessageNumberForRollingHash", [
          finalizationData.l1RollingHash,
        ]);
      });

      it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 1n,
          l1RollingHash: HASH_ZERO,
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "MissingRollingHashForMessageNumber", [
          finalizationData.l1RollingHashMessageNumber,
        ]);
      });

      it("Should revert if l1RollingHash does not exist on L1", async () => {
        const finalizationData = await generateFinalizationData({
          l1RollingHashMessageNumber: 1n,
          l1RollingHash: generateRandomBytes(32),
        });

        const lastFinalizedBlockNumber = await lineaRollup.currentL2BlockNumber();
        const parentStateRootHash = await lineaRollup.stateRootHashes(lastFinalizedBlockNumber);
        finalizationData.parentStateRootHash = parentStateRootHash;

        const proof = calldataAggregatedProof1To155.aggregatedProof;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(proof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "L1RollingHashDoesNotExistOnL1", [
          finalizationData.l1RollingHashMessageNumber,
          finalizationData.l1RollingHash,
        ]);
      });

      it("Should revert if timestamps are not in sequence", async () => {
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: 30_000_000,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        finalizationData.lastFinalizedTimestamp = finalizationData.finalTimestamp + 1n;

        const expectedHashValue = generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.lastFinalizedL1RollingHashMessageNumber,
            finalizationData.lastFinalizedL1RollingHash,
            finalizationData.lastFinalizedTimestamp,
          ],
        );
        const actualHashValue = generateKeccak256(
          ["uint256", "bytes32", "uint256"],
          [
            finalizationData.lastFinalizedL1RollingHashMessageNumber,
            finalizationData.lastFinalizedL1RollingHash,
            DEFAULT_LAST_FINALIZED_TIMESTAMP,
          ],
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalizationStateIncorrect", [
          expectedHashValue,
          actualHashValue,
        ]);
      });

      it("Should revert if the final shnarf does not exist", async () => {
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: 30_000_000,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        finalizationData.shnarfData.snarkHash = generateRandomBytes(32);

        const { dataEvaluationClaim, dataEvaluationPoint, finalStateRootHash, parentShnarf, snarkHash } =
          finalizationData.shnarfData;
        const expectedMissingBlobShnarf = generateKeccak256(
          ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
          [parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim],
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalBlobNotSubmitted", [
          expectedMissingBlobShnarf,
        ]);
      });

      it("Should revert if finalizationData.finalTimestamp is greater than the block.timestamp", async () => {
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: 30_000_000,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(new Date(new Date().setHours(new Date().getHours() + 2)).getTime()), // Set to 2 hours in the future
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        const finalizeCompressedCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCompressedCall, "FinalizationInTheFuture", [
          finalizationData.finalTimestamp,
          (await networkTime.latest()) + 1,
        ]);
      });
    });

    describe("Without submission data", () => {
      it("Should revert if the final block state equals the zero hash", async () => {
        const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
        let index = 0;
        for (const data of submissionDataBeforeFinalization) {
          const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
          await lineaRollup
            .connect(operator)
            .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
              gasLimit: 30_000_000,
            });
          index++;
        }

        const finalizationData = await generateFinalizationData({
          l1RollingHash: calculateRollingHash(HASH_ZERO, messageHash),
          l1RollingHashMessageNumber: 10n,
          lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
          endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
          parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
          finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
          l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
          l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
          l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
          aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
          shnarfData: generateParentShnarfData(index),
        });

        await lineaRollup.setRollingHash(
          calldataAggregatedProof1To155.l1RollingHashMessageNumber,
          calldataAggregatedProof1To155.l1RollingHash,
        );

        // Set the final state root hash to zero
        finalizationData.shnarfData.finalStateRootHash = HASH_ZERO;

        const finalizeCall = lineaRollup
          .connect(operator)
          .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

        await expectRevertWithCustomError(lineaRollup, finalizeCall, "FinalBlockStateEqualsZeroHash");
      });
    });
  });

  describe("Compressed data finalization with proof", () => {
    beforeEach(async () => {
      await lineaRollup.setLastFinalizedBlock(0);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const finalizationData = await generateFinalizationData();

      const finalizeCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithReason(finalizeCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const finalizationData = await generateFinalizationData();

      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if FINALIZATION_PAUSE_TYPE is enabled", async () => {
      const finalizationData = await generateFinalizationData();

      await lineaRollup.connect(securityCouncil).pauseByType(FINALIZATION_PAUSE_TYPE);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "IsPaused", [FINALIZATION_PAUSE_TYPE]);
    });

    it("Should revert if the proof is empty", async () => {
      const finalizationData = await generateFinalizationData();

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(EMPTY_CALLDATA, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "ProofIsEmpty");
    });

    it("Should revert when finalization parentStateRootHash is different than last finalized state root hash", async () => {
      // Submit 4 sets of compressed data setting the correct shnarf in storage
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);

      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        lastFinalizedTimestamp: DEFAULT_LAST_FINALIZED_TIMESTAMP,
        parentStateRootHash: generateRandomBytes(32),
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
      });

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData, {
          gasLimit: 30_000_000,
        });
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "StartingRootHashDoesNotMatch");
    });

    it("Should successfully finalize with only previously submitted data", async () => {
      // Submit 4 sets of compressed data setting the correct shnarf in storage
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      await expectSuccessfulFinalize(
        calldataAggregatedProof1To155,
        index,
        fourthCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
      );
    });

    it("Should revert when proofType is invalid", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, 99, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProofType");
    });

    it("Should revert when using a proofType index that was removed", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      // removing the verifier index
      await lineaRollup.connect(securityCouncil).unsetVerifierAddress(TEST_PUBLIC_VERIFIER_INDEX);

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProofType");
    });

    it("Should fail when proof does not match", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(index),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      // aggregatedProof1To81.aggregatedProof, wrong proof on purpose
      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(aggregatedProof1To81.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });

    it("Should fail if shnarf does not exist when finalizing", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmission(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const finalizationData = await generateFinalizationData({
        l1RollingHash: calldataAggregatedProof1To155.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(calldataAggregatedProof1To155.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(calldataAggregatedProof1To155.parentAggregationLastBlockTimestamp),
        endBlockNumber: BigInt(calldataAggregatedProof1To155.finalBlockNumber),
        parentStateRootHash: calldataAggregatedProof1To155.parentStateRootHash,
        finalTimestamp: BigInt(calldataAggregatedProof1To155.finalTimestamp),
        l2MerkleRoots: calldataAggregatedProof1To155.l2MerkleRoots,
        l2MerkleTreesDepth: BigInt(calldataAggregatedProof1To155.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: calldataAggregatedProof1To155.l2MessagingBlocksOffsets,
        aggregatedProof: calldataAggregatedProof1To155.aggregatedProof,
        shnarfData: generateParentShnarfData(1),
      });

      await lineaRollup.setRollingHash(
        calldataAggregatedProof1To155.l1RollingHashMessageNumber,
        calldataAggregatedProof1To155.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(calldataAggregatedProof1To155.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);
      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });

    it("Should successfully finalize 1-81 and then 82-153 in two separate finalizations", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      await expectSuccessfulFinalize(
        aggregatedProof1To81,
        2,
        secondCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
        true,
      );

      await expectSuccessfulFinalize(
        aggregatedProof82To153,
        4,
        fourthMultipleCompressedDataContent.finalStateRootHash,
        generateParentShnarfData,
        true,
        aggregatedProof1To81.l1RollingHash,
        BigInt(aggregatedProof1To81.l1RollingHashMessageNumber),
      );
    });

    it("Should succeed when sending with pure calldata", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }
      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      const encodedCall =
        "0x5603c65f0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003e00000000000000000000000000000000000000000000000000000000000000360008e483358dc0ac1d3f6a27b43f6332e88c4196151b2a1ce6c8575013cec2cbe2f666e5649165dfc5a88ede985203341f5910ec2f4e3d4e9fa2a0a2621e4b0a70a91e0f995a8d754a3ddec1698800be83da8815650488540d2ffb699eaf3518d14bff479974f1f5f8bef7687f89c6301247cd92a4ccfa76a868bd24090a670f20eb06513431e74411648ab8c02a270353146bc759451d00cf2a59477d6106b9d11df3e18bca0d3a36951e31699b21a73e22a77fb7fe43a33b717a23dd06a889a102e4143d1e024e053e4318f903c3dbd0ec0f6e49672dd0ccb10749ac00adaf311149a24bf2c2ee4cfe7ed1db713526c6f7c0893f2ec153e5f55a4b390ce135a0d3ebe372c1beea8142afebdc96fe2764b8b5a33efc0457b9ad65f678c8b9ede196eb1fe34a82e0f370fe1d53e5cfc34bc77f38a64eddbb74f6530a82feb0e0a148de4d309b179697ff8a3c97a5516a1bbffc36d8cf153e9f5ee8fdc98002fd1136a2a12603aa1c7a985b34c5dc6774a5efd6270eac180406b4f03c7ef08eee428bb45d04a3c6635a0225aae76c0522049df5080045629cd6a686c2d50d8b7fe10e6facded652c8d75d4a67d5f8f4c3c6b491f44cc23c307064ec94f7c3a44eb1d1dcd369464960e54a88ad3f9edbdd818f2ac711d6c8f49efe224fb2616f2ac0005ca0c753c801d9e2df1133cdd4de5e54554197ac3e2f42d0b140261a12796050d64ec14bb22355173d157a7a2a9a5c0a20d47430819b4c106fda54923f9bf273cbc4145962e07b853279e64e198d3da5919814935c4cf12ffc03128d0e4fc24e8f35180d257253c0ce33725cb665a81141f16418fb20cf2abb96fa65a03bc0fa8e1e842123e2a2590f423e34c1db2c51c9e872ea41942d44d94d735cf8b4b0ccc265c6bdc41dd6f1167da133450a5ea0c07ea4704c9ca377cd201d8a63f932f7d4c9edb88269008bfe08d91d576cbf1a46c2da88b13d2a38e68f66f330552114f37a226c38ed3a9acd35b75f5852e6879eb3c8746ed2676f9e3957d32bbee1493ac055f03a56e964c4ea278d323abde55b36439b5460897771bf5ef3be1992448a5479f3515d5605d403a51391c6c6db94a90b3353aa956a5ba6f7b951fee2b7657ab431f90220c84820c2db8e4b57eaa5c3ea13e8d1f4ed31b9434c5df5c083dbb24fdf9dc5a64ba7416e6d154e6b17727a3acaa9f59820e926196efd232072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd00000000000000000000000000000000000000000000000000000000000000519562fa89830a0ba0063a636ca96e52ce2f032b855336c95dd788321f4e1934190eacb8ed649249b3ec6efd9fbd6ffebe1b325b53380a91f7d689bfc1aff3b6dcf0f26782f7afb93f926cacb145f55530714f20b1356725e3971dc99e0ef8b59101216d3e1700c3a0d5115686fa51caa982cb4e002a5bb9f9488c9c44e4d9a3042d2f290de42ce8ea03cf8ac09288166932f174cb069ff8147c95ed6374e4e6cb00000000000000000000000000000000000000000000000000000000645580d1000000000000000000000000000000000000000000000000000000006455a7e10000000000000000000000000000000000000000000000000000000000000000dc8e70637c1048e1e0406c4ed6fab51a7489ccb52f37ddd2c135cb1aa18ec6970000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000024000000000000000000000000000000000000000000000000000000000000000016de197db892fd7d3fe0a389452dae0c5d0520e23d18ad20327546e2189a7e3f1000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000fffff000000000000000000000000000000000000";
      const transaction = {
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: 31337,
        nonce: 4,
        value: 0,
        gasLimit: 5_000_000,
      };

      await expect(operator.sendTransaction(transaction)).to.not.be.reverted;
    });

    it("Should fail when sending with wrong merkle root location", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const lineaRollupAddress = await lineaRollup.getAddress();

      const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();

      // This contains the Merkle roots length for public input at 0x220 and a contrived pointer to an alternate location.
      const encodedCall =
        "0x5603c65f0000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003e00000000000000000000000000000000000000000000000000000000000000360008e483358dc0ac1d3f6a27b43f6332e88c4196151b2a1ce6c8575013cec2cbe2f666e5649165dfc5a88ede985203341f5910ec2f4e3d4e9fa2a0a2621e4b0a70a91e0f995a8d754a3ddec1698800be83da8815650488540d2ffb699eaf3518d14bff479974f1f5f8bef7687f89c6301247cd92a4ccfa76a868bd24090a670f20eb06513431e74411648ab8c02a270353146bc759451d00cf2a59477d6106b9d11df3e18bca0d3a36951e31699b21a73e22a77fb7fe43a33b717a23dd06a889a102e4143d1e024e053e4318f903c3dbd0ec0f6e49672dd0ccb10749ac00adaf311149a24bf2c2ee4cfe7ed1db713526c6f7c0893f2ec153e5f55a4b390ce135a0d3ebe372c1beea8142afebdc96fe2764b8b5a33efc0457b9ad65f678c8b9ede196eb1fe34a82e0f370fe1d53e5cfc34bc77f38a64eddbb74f6530a82feb0e0a148de4d309b179697ff8a3c97a5516a1bbffc36d8cf153e9f5ee8fdc98002fd1136a2a12603aa1c7a985b34c5dc6774a5efd6270eac180406b4f03c7ef08eee428bb45d04a3c6635a0225aae76c0522049df5080045629cd6a686c2d50d8b7fe10e6facded652c8d75d4a67d5f8f4c3c6b491f44cc23c307064ec94f7c3a44eb1d1dcd369464960e54a88ad3f9edbdd818f2ac711d6c8f49efe224fb2616f2ac0005ca0c753c801d9e2df1133cdd4de5e54554197ac3e2f42d0b140261a12796050d64ec14bb22355173d157a7a2a9a5c0a20d47430819b4c106fda54923f9bf273cbc4145962e07b853279e64e198d3da5919814935c4cf12ffc03128d0e4fc24e8f35180d257253c0ce33725cb665a81141f16418fb20cf2abb96fa65a03bc0fa8e1e842123e2a2590f423e34c1db2c51c9e872ea41942d44d94d735cf8b4b0ccc265c6bdc41dd6f1167da133450a5ea0c07ea4704c9ca377cd201d8a63f932f7d4c9edb88269008bfe08d91d576cbf1a46c2da88b13d2a38e68f66f330552114f37a226c38ed3a9acd35b75f5852e6879eb3c8746ed2676f9e3957d32bbee1493ac055f03a56e964c4ea278d323abde55b36439b5460897771bf5ef3be1992448a5479f3515d5605d403a51391c6c6db94a90b3353aa956a5ba6f7b951fee2b7657ab431f90220c84820c2db8e4b57eaa5c3ea13e8d1f4ed31b9434c5df5c083dbb24fdf9dc5a64ba7416e6d154e6b17727a3acaa9f59820e926196efd232072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd00000000000000000000000000000000000000000000000000000000000000519562fa89830a0ba0063a636ca96e52ce2f032b855336c95dd788321f4e1934190eacb8ed649249b3ec6efd9fbd6ffebe1b325b53380a91f7d689bfc1aff3b6dcf0f26782f7afb93f926cacb145f55530714f20b1356725e3971dc99e0ef8b59101216d3e1700c3a0d5115686fa51caa982cb4e002a5bb9f9488c9c44e4d9a3042d2f290de42ce8ea03cf8ac09288166932f174cb069ff8147c95ed6374e4e6cb00000000000000000000000000000000000000000000000000000000645580d1000000000000000000000000000000000000000000000000000000006455a7e10000000000000000000000000000000000000000000000000000000000000000dc8e70637c1048e1e0406c4ed6fab51a7489ccb52f37ddd2c135cb1aa18ec6970000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000280000000000000000000000000000000000000000000000000000000000000024000000000000000000000000000000000000000000000000000000000000000016de197db892fd7d3fe0a389452dae0c5d0520e23d18ad20327546e2189a7e3f1000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000fffff000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000001cafecafe";
      const transaction = {
        data: encodedCall,
        maxPriorityFeePerGas: maxPriorityFeePerGas!,
        maxFeePerGas: maxFeePerGas!,
        to: lineaRollupAddress,
        chainId: 31337,
        nonce: 4,
        value: 0,
        gasLimit: 5_000_000,
      };

      await expectRevertWithCustomError(lineaRollup, operator.sendTransaction(transaction), "InvalidProof");
    });

    it("Should fail to finalize with extra merkle roots", async () => {
      const submissionDataBeforeFinalization = generateCallDataSubmissionMultipleProofs(0, 4);
      let index = 0;
      for (const data of submissionDataBeforeFinalization) {
        const parentAndExpectedShnarf = generateParentAndExpectedShnarfForMulitpleIndex(index);
        await lineaRollup
          .connect(operator)
          .submitDataAsCalldata(data, parentAndExpectedShnarf.parentShnarf, parentAndExpectedShnarf.expectedShnarf, {
            gasLimit: 30_000_000,
          });
        index++;
      }

      const merkleRoots = aggregatedProof1To81.l2MerkleRoots;
      merkleRoots.push(generateRandomBytes(32));

      const finalizationData = await generateFinalizationData({
        l1RollingHash: aggregatedProof1To81.l1RollingHash,
        l1RollingHashMessageNumber: BigInt(aggregatedProof1To81.l1RollingHashMessageNumber),
        lastFinalizedTimestamp: BigInt(aggregatedProof1To81.parentAggregationLastBlockTimestamp),
        parentStateRootHash: aggregatedProof1To81.parentStateRootHash,
        finalTimestamp: BigInt(aggregatedProof1To81.finalTimestamp),
        l2MerkleRoots: merkleRoots,
        l2MerkleTreesDepth: BigInt(aggregatedProof1To81.l2MerkleTreesDepth),
        l2MessagingBlocksOffsets: aggregatedProof1To81.l2MessagingBlocksOffsets,
        aggregatedProof: aggregatedProof1To81.aggregatedProof,
        shnarfData: generateParentShnarfData(2, true),
      });

      finalizationData.lastFinalizedL1RollingHash = HASH_ZERO;
      finalizationData.lastFinalizedL1RollingHashMessageNumber = 0n;

      await lineaRollup.setRollingHash(
        aggregatedProof1To81.l1RollingHashMessageNumber,
        aggregatedProof1To81.l1RollingHash,
      );

      const finalizeCall = lineaRollup
        .connect(operator)
        .finalizeBlocks(aggregatedProof1To81.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

      await expectRevertWithCustomError(lineaRollup, finalizeCall, "InvalidProof");
    });
  });

  describe("Validate L2 computed rolling hash", () => {
    it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "MissingMessageNumberForRollingHash",
        [l1RollingHash],
      );
    });

    it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = HASH_ZERO;

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "MissingRollingHashForMessageNumber",
        [l1MessageNumber],
      );
    });

    it("Should revert if l1RollingHash does not exist on L1", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "L1RollingHashDoesNotExistOnL1",
        [l1MessageNumber, l1RollingHash],
      );
    });

    it("Should succeed if l1 message number == 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = HASH_ZERO;
      await expect(lineaRollup.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash)).to.not.be.reverted;
    });

    it("Should succeed if l1 message number != 0, l1 rolling hash is not empty and exists on L1", async () => {
      const l1MessageNumber = 1n;
      const messageHash = generateRandomBytes(32);

      await lineaRollup.addRollingHash(l1MessageNumber, messageHash);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await expect(lineaRollup.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash)).to.not.be.reverted;
    });
  });

  describe("Calculate Y value for Compressed Data", () => {
    it("Should successfully calculate y", async () => {
      const compressedDataBytes = ethers.decodeBase64(compressedData);

      expect(await lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: 30_000_000 })).to.equal(
        expectedY,
      );
    });

    it("Should revert if first byte is no zero", async () => {
      const compressedDataBytes = encodeData(
        ["bytes32", "bytes32", "bytes32"],
        [generateRandomBytes(32), HASH_WITHOUT_ZERO_FIRST_BYTE, generateRandomBytes(32)],
      );

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: 30_000_000 }),
        "FirstByteIsNotZero",
      );
    });

    it("Should revert if bytes length is not a multiple of 32", async () => {
      const compressedDataBytes = generateRandomBytes(56);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: 30_000_000 }),
        "BytesLengthNotMultipleOf32",
      );
    });
  });

  describe("fallback operator Role", () => {
    const expectedLastFinalizedState = calculateLastFinalizedState(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP);

    it("Should revert if trying to set fallback operator role before six months have passed", async () => {
      const initialBlock = await ethers.provider.getBlock("latest");

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setFallbackOperator(0n, HASH_ZERO, BigInt(initialBlock!.timestamp)),
        "LastFinalizationTimeNotLapsed",
      );
    });

    it("Should revert if the time has passed and the last finalized timestamp does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(0n, HASH_ZERO, 123456789n);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setFallbackOperator(0n, HASH_ZERO, 123456789n),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 message number does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(1n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setFallbackOperator(1n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 rolling hash does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const random32Bytes = generateRandomBytes(32);
      const actualSentState = calculateLastFinalizedState(0n, random32Bytes, DEFAULT_LAST_FINALIZED_TIMESTAMP);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setFallbackOperator(0n, random32Bytes, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should set the fallback operator role after six months have passed", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setFallbackOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FallbackOperatorRoleGranted",
        [admin.address, fallbackoperatorAddress],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, fallbackoperatorAddress)).to.be.true;
    });

    it("Should revert if trying to renounce role as fallback operator", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setFallbackOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FallbackOperatorRoleGranted",
        [admin.address, fallbackoperatorAddress],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, fallbackoperatorAddress)).to.be.true;

      const renounceCall = lineaRollup.renounceRole(OPERATOR_ROLE, fallbackoperatorAddress);

      expectRevertWithCustomError(lineaRollup, renounceCall, "OnlyNonFallbackOperator");
    });

    it("Should renounce role if not fallback operator", async () => {
      expect(await lineaRollup.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;

      const renounceCall = lineaRollup.connect(operator).renounceRole(OPERATOR_ROLE, operator.address);
      const args = [OPERATOR_ROLE, operator.address, operator.address];
      expectEvent(lineaRollup, renounceCall, "RoleRevoked", args);
    });
  });

  async function sendBlobTransaction(startIndex: number, finalIndex: number, isMultiple: boolean = false) {
    const operatorHDSigner = getWalletForIndex(2);
    const lineaRollupAddress = await lineaRollup.getAddress();

    const {
      blobDataSubmission: blobSubmission,
      compressedBlobs: compressedBlobs,
      parentShnarf: parentShnarf,
      finalShnarf: finalShnarf,
    } = generateBlobDataSubmission(startIndex, finalIndex, isMultiple);

    const encodedCall = lineaRollup.interface.encodeFunctionData("submitBlobs", [
      blobSubmission,
      parentShnarf,
      finalShnarf,
    ]);

    const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
    const nonce = await operatorHDSigner.getNonce();

    const transaction = Transaction.from({
      data: encodedCall,
      maxPriorityFeePerGas: maxPriorityFeePerGas!,
      maxFeePerGas: maxFeePerGas!,
      to: lineaRollupAddress,
      chainId: (await ethers.provider.getNetwork()).chainId,
      type: 3,
      nonce: nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);
    const txResponse = await ethers.provider.broadcastTransaction(signedTx);

    const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);

    const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

    expectEventDirectFromReceiptData(lineaRollup as BaseContract, receipt!, "DataSubmittedV3", expectedEventArgs);
  }

  async function sendBlobTransactionViaCallForwarder(
    lineaRollupUpgraded: Contract,
    startIndex: number,
    finalIndex: number,
    callforwarderAddress: string,
  ) {
    const operatorHDSigner = getWalletForIndex(2);

    const {
      blobDataSubmission: blobSubmission,
      compressedBlobs: compressedBlobs,
      parentShnarf: parentShnarf,
      finalShnarf: finalShnarf,
    } = generateBlobDataSubmission(startIndex, finalIndex, false);

    const encodedCall = lineaRollupUpgraded.interface.encodeFunctionData("submitBlobs", [
      blobSubmission,
      parentShnarf,
      finalShnarf,
    ]);

    const { maxFeePerGas, maxPriorityFeePerGas } = await ethers.provider.getFeeData();
    const nonce = await operatorHDSigner.getNonce();

    const transaction = Transaction.from({
      data: encodedCall,
      maxPriorityFeePerGas: maxPriorityFeePerGas!,
      maxFeePerGas: maxFeePerGas!,
      to: callforwarderAddress,
      chainId: (await ethers.provider.getNetwork()).chainId,
      type: 3,
      nonce: nonce,
      value: 0,
      gasLimit: 5_000_000,
      kzg,
      maxFeePerBlobGas: 1n,
      blobs: compressedBlobs,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);
    const txResponse = await ethers.provider.broadcastTransaction(signedTx);
    const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);

    const expectedEventArgs = [parentShnarf, finalShnarf, blobSubmission[blobSubmission.length - 1].finalStateRootHash];

    expectEventDirectFromReceiptData(
      lineaRollupUpgraded as BaseContract,
      receipt!,
      "DataSubmittedV3",
      expectedEventArgs,
    );
  }

  async function expectSuccessfulFinalize(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    proofData: any,
    blobParentShnarfIndex: number,
    finalStateRootHash: string,
    shnarfDataGenerator: ShnarfDataGenerator,
    isMultiple: boolean = false,
    lastFinalizedRollingHash: string = HASH_ZERO,
    lastFinalizedMessageNumber: bigint = 0n,
  ) {
    const finalizationData = await generateFinalizationData({
      l1RollingHash: proofData.l1RollingHash,
      l1RollingHashMessageNumber: BigInt(proofData.l1RollingHashMessageNumber),
      lastFinalizedTimestamp: BigInt(proofData.parentAggregationLastBlockTimestamp),
      endBlockNumber: BigInt(proofData.finalBlockNumber),
      parentStateRootHash: proofData.parentStateRootHash,
      finalTimestamp: BigInt(proofData.finalTimestamp),
      l2MerkleRoots: proofData.l2MerkleRoots,
      l2MerkleTreesDepth: BigInt(proofData.l2MerkleTreesDepth),
      l2MessagingBlocksOffsets: proofData.l2MessagingBlocksOffsets,
      aggregatedProof: proofData.aggregatedProof,
      shnarfData: shnarfDataGenerator(blobParentShnarfIndex, isMultiple),
    });
    finalizationData.lastFinalizedL1RollingHash = lastFinalizedRollingHash;
    finalizationData.lastFinalizedL1RollingHashMessageNumber = lastFinalizedMessageNumber;

    await lineaRollup.setRollingHash(proofData.l1RollingHashMessageNumber, proofData.l1RollingHash);

    const finalizeCompressedCall = lineaRollup
      .connect(operator)
      .finalizeBlocks(proofData.aggregatedProof, TEST_PUBLIC_VERIFIER_INDEX, finalizationData);

    const eventArgs = [
      BigInt(proofData.lastFinalizedBlockNumber) + 1n,
      finalizationData.endBlockNumber,
      proofData.finalShnarf,
      finalizationData.parentStateRootHash,
      finalStateRootHash,
    ];

    await expectEvent(lineaRollup, finalizeCompressedCall, "DataFinalizedV3", eventArgs);

    const [expectedFinalStateRootHash, lastFinalizedBlockNumber, lastFinalizedState] = await Promise.all([
      lineaRollup.stateRootHashes(finalizationData.endBlockNumber),
      lineaRollup.currentL2BlockNumber(),
      lineaRollup.currentFinalizedState(),
    ]);

    expect(expectedFinalStateRootHash).to.equal(finalizationData.shnarfData.finalStateRootHash);
    expect(lastFinalizedBlockNumber).to.equal(finalizationData.endBlockNumber);
    expect(lastFinalizedState).to.equal(
      generateKeccak256(
        ["uint256", "bytes32", "uint256"],
        [finalizationData.l1RollingHashMessageNumber, finalizationData.l1RollingHash, finalizationData.finalTimestamp],
      ),
    );
  }

  async function expectSuccessfulFinalizeViaCallForwarder(
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    proofData: any,
    blobParentShnarfIndex: number,
    finalStateRootHash: string,
    shnarfDataGenerator: ShnarfDataGenerator,
    isMultiple: boolean = false,
    lastFinalizedRollingHash: string = HASH_ZERO,
    lastFinalizedMessageNumber: bigint = 0n,
    callforwarderAddress: string,
    upgradedContract: Contract,
  ) {
    const finalizationData = await generateFinalizationData({
      l1RollingHash: proofData.l1RollingHash,
      l1RollingHashMessageNumber: BigInt(proofData.l1RollingHashMessageNumber),
      lastFinalizedTimestamp: BigInt(proofData.parentAggregationLastBlockTimestamp),
      endBlockNumber: BigInt(proofData.finalBlockNumber),
      parentStateRootHash: proofData.parentStateRootHash,
      finalTimestamp: BigInt(proofData.finalTimestamp),
      l2MerkleRoots: proofData.l2MerkleRoots,
      l2MerkleTreesDepth: BigInt(proofData.l2MerkleTreesDepth),
      l2MessagingBlocksOffsets: proofData.l2MessagingBlocksOffsets,
      aggregatedProof: proofData.aggregatedProof,
      shnarfData: shnarfDataGenerator(blobParentShnarfIndex, isMultiple),
    });
    finalizationData.lastFinalizedL1RollingHash = lastFinalizedRollingHash;
    finalizationData.lastFinalizedL1RollingHashMessageNumber = lastFinalizedMessageNumber;

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
        proofData.parentStateRootHash,
        BigInt(proofData.finalBlockNumber),
        [
          shnarfData.parentShnarf,
          shnarfData.snarkHash,
          shnarfData.finalStateRootHash,
          shnarfData.dataEvaluationPoint,
          shnarfData.dataEvaluationClaim,
        ],
        proofData.parentAggregationLastBlockTimestamp,
        proofData.finalTimestamp,
        lastFinalizedRollingHash,
        proofData.l1RollingHash,
        lastFinalizedMessageNumber,
        proofData.l1RollingHashMessageNumber,
        proofData.l2MerkleTreesDepth,
        proofData.l2MerkleRoots,
        proofData.l2MessagingBlocksOffsets,
      ],
    ];

    const encodedCall = ethers.concat([
      "0x5603c65f",
      ethers.AbiCoder.defaultAbiCoder().encode(
        [
          "bytes",
          "uint256",
          "tuple(bytes32,uint256,tuple(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,bytes32[],bytes)",
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
      nonce: nonce,
      value: 0,
      gasLimit: 5_000_000,
    });

    const signedTx = await operatorHDSigner.signTransaction(transaction);

    const txResponse = await ethers.provider.broadcastTransaction(signedTx);
    const receipt = await ethers.provider.getTransactionReceipt(txResponse.hash);
    expect(receipt).is.not.null;

    const eventArgs = [
      BigInt(proofData.lastFinalizedBlockNumber) + 1n,
      finalizationData.endBlockNumber,
      proofData.finalShnarf,
      finalizationData.parentStateRootHash,
      finalStateRootHash,
    ];

    const dataFinalizedLogIndex = 8;

    expectEventDirectFromReceiptData(
      upgradedContract as BaseContract,
      receipt!,
      "DataFinalizedV3",
      eventArgs,
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
      generateKeccak256(
        ["uint256", "bytes32", "uint256"],
        [finalizationData.l1RollingHashMessageNumber, finalizationData.l1RollingHash, finalizationData.finalTimestamp],
      ),
    );
  }

  describe("LineaRollup Upgradeable Tests", () => {
    let newRoleAddresses: { addressWithRole: string; role: string }[];

    async function deployLineaRollupFixture() {
      const plonkVerifierFactory = await ethers.getContractFactory("TestPlonkVerifierForDataAggregation");
      const plonkVerifier = await plonkVerifierFactory.deploy();
      await plonkVerifier.waitForDeployment();

      verifier = await plonkVerifier.getAddress();

      const lineaRollup = (await deployUpgradableFromFactory(
        "contracts/test-contracts/TestLineaRollupV5.sol:TestLineaRollupV5",
        [
          parentStateRootHash,
          0,
          verifier,
          securityCouncil.address,
          [operator.address],
          ONE_DAY_IN_SECONDS,
          INITIAL_WITHDRAW_LIMIT,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ],
        {
          initializer: "initialize(bytes32,uint256,address,address,address[],uint256,uint256,uint256)",
          unsafeAllow: ["constructor"],
        },
      )) as unknown as TestLineaRollupV5;

      return lineaRollup;
    }

    before(async () => {
      const securityCouncilAddress = securityCouncil.address;

      newRoleAddresses = [
        { addressWithRole: securityCouncilAddress, role: USED_RATE_LIMIT_RESETTER_ROLE },
        { addressWithRole: securityCouncilAddress, role: VERIFIER_UNSETTER_ROLE },
        { addressWithRole: securityCouncilAddress, role: PAUSE_ALL_ROLE },
        { addressWithRole: securityCouncilAddress, role: PAUSE_L1_L2_ROLE },
        { addressWithRole: securityCouncilAddress, role: PAUSE_L2_L1_ROLE },
        { addressWithRole: securityCouncilAddress, role: UNPAUSE_ALL_ROLE },
        { addressWithRole: securityCouncilAddress, role: UNPAUSE_L1_L2_ROLE },
        { addressWithRole: securityCouncilAddress, role: UNPAUSE_L2_L1_ROLE },
        { addressWithRole: securityCouncilAddress, role: PAUSE_BLOB_SUBMISSION_ROLE },
        { addressWithRole: securityCouncilAddress, role: UNPAUSE_BLOB_SUBMISSION_ROLE },
        { addressWithRole: securityCouncilAddress, role: PAUSE_FINALIZATION_ROLE },
        { addressWithRole: securityCouncilAddress, role: UNPAUSE_FINALIZATION_ROLE },
      ];
    });

    beforeEach(async () => {
      lineaRollupV5 = await loadFixture(deployLineaRollupFixture);

      // we need to set a non-zero value for the genesis shnarf else it will break on blob submission.
      const { parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim } =
        generateParentShnarfData(0);
      const genesisShnarf = generateKeccak256(
        ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
        [parentShnarf, snarkHash, finalStateRootHash, dataEvaluationPoint, dataEvaluationClaim],
      );

      await lineaRollupV5.setDefaultShnarfExistValue(genesisShnarf);
    });

    it("Should revert if fallback operator has address zero", async () => {
      expect(await lineaRollupV5.currentL2BlockNumber()).to.equal(0);

      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory(
        "contracts/test-contracts/TestLineaRollup.sol:TestLineaRollup",
      );
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();

      expectRevertWithCustomError(
        upgradedContract,
        upgradedContract.reinitializeLineaRollupV6(
          newRoleAddresses,
          LINEA_ROLLUP_PAUSE_TYPES_ROLES,
          LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
          ADDRESS_ZERO,
        ),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should fail to accept ETH on the CallForwardingProxy receive function", async () => {
      await deployCallForwardingProxy(await lineaRollupV5.getAddress());
      const forwardingProxyAddress = await callForwardingProxy.getAddress();

      const tx = {
        to: forwardingProxyAddress,
        value: ethers.parseEther("0.1"),
      };

      await expectRevertWithReason(admin.sendTransaction(tx), "ETH not accepted");
    });

    it("Should be able to submit blobs and finalize via callforwarding proxy", async () => {
      // Deploy callforwarding proxy
      await deployCallForwardingProxy(await lineaRollupV5.getAddress());
      const forwardingProxyAddress = await callForwardingProxy.getAddress();

      expect(await lineaRollupV5.currentL2BlockNumber()).to.equal(0);

      // Deploy new LineaRollup implementation
      const newLineaRollupFactory = await ethers.getContractFactory(
        "contracts/test-contracts/TestLineaRollup.sol:TestLineaRollup",
      );
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });

      const upgradedContract = await newLineaRollup.waitForDeployment();

      await upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        forwardingProxyAddress,
      );

      // Grants deployed callforwarding proxy as operator
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      await expectEvent(
        upgradedContract,
        upgradedContract.setFallbackOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FallbackOperatorRoleGranted",
        [admin.address, forwardingProxyAddress],
      );

      // Submit 2 blobs
      await sendBlobTransactionViaCallForwarder(upgradedContract, 0, 2, forwardingProxyAddress);
      // Submit another 2 blobs
      await sendBlobTransactionViaCallForwarder(upgradedContract, 2, 4, forwardingProxyAddress);

      // Finalize 4 blobs
      await expectSuccessfulFinalizeViaCallForwarder(
        blobAggregatedProof1To155,
        4,
        fourthCompressedDataContent.finalStateRootHash,
        generateBlobParentShnarfData,
        false,
        HASH_ZERO,
        0n,
        forwardingProxyAddress,
        upgradedContract,
      );
    });

    it("Should deploy and upgrade the LineaRollup contract expecting LineaRollupVersionChanged", async () => {
      expect(await lineaRollupV5.currentL2BlockNumber()).to.equal(0);

      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();

      const upgradeCall = upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      const expectedVersion5Bytes8 = convertStringToPaddedHexBytes("5.0", 8);
      const expectedVersion6Bytes8 = convertStringToPaddedHexBytes("6.0", 8);

      await expectEvent(upgradedContract, upgradeCall, "LineaRollupVersionChanged", [
        expectedVersion5Bytes8,
        expectedVersion6Bytes8,
      ]);
    });

    it("Should upgrade the LineaRollup contract expecting FallbackOperatorAddressSet", async () => {
      expect(await lineaRollupV5.currentL2BlockNumber()).to.equal(0);

      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();
      const upgradeCall = upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      await expectEvent(upgradedContract, upgradeCall, "FallbackOperatorAddressSet", [
        admin.address,
        fallbackoperatorAddress,
      ]);
    });

    it("Should not be able to call reinitializeLineaRollupV6 when upgraded.", async () => {
      expect(await lineaRollupV5.currentL2BlockNumber()).to.equal(0);

      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();
      await upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      const secondCall = upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      await expectRevertWithReason(secondCall, "Initializable: contract is already initialized");
    });

    it("Should revert with ZeroAddressNotAllowed when addressWithRole is zero address in reinitializeLineaRollupV6", async () => {
      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();
      const roleAddresses = [{ addressWithRole: ZeroAddress, role: DEFAULT_ADMIN_ROLE }, ...newRoleAddresses.slice(1)];

      await expectRevertWithCustomError(
        upgradedContract,
        upgradedContract.reinitializeLineaRollupV6(
          roleAddresses,
          LINEA_ROLLUP_PAUSE_TYPES_ROLES,
          LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
          fallbackoperatorAddress,
        ),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should set all permissions", async () => {
      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();

      await upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      for (const { role, addressWithRole } of newRoleAddresses) {
        expect(await upgradedContract.hasRole(role, addressWithRole)).to.be.true;
      }
    });

    it("Should set all pause types and unpause types in mappings and emit events", async () => {
      // Deploy new implementation
      const newLineaRollupFactory = await ethers.getContractFactory("contracts/LineaRollup.sol:LineaRollup");
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollupV5, newLineaRollupFactory, {
        unsafeAllowRenames: true,
      });
      const upgradedContract = await newLineaRollup.waitForDeployment();

      const reinitializePromise = upgradedContract.reinitializeLineaRollupV6(
        newRoleAddresses,
        LINEA_ROLLUP_PAUSE_TYPES_ROLES,
        LINEA_ROLLUP_UNPAUSE_TYPES_ROLES,
        fallbackoperatorAddress,
      );

      await Promise.all([
        expectEvents(
          upgradedContract,
          reinitializePromise,
          LINEA_ROLLUP_PAUSE_TYPES_ROLES.map(({ pauseType, role }) => ({
            name: "PauseTypeRoleSet",
            args: [pauseType, role],
          })),
        ),
        expectEvents(
          upgradedContract,
          reinitializePromise,
          LINEA_ROLLUP_UNPAUSE_TYPES_ROLES.map(({ pauseType, role }) => ({
            name: "UnPauseTypeRoleSet",
            args: [pauseType, role],
          })),
        ),
      ]);

      const pauseTypeRolesMappingSlot = 219;
      const unpauseTypeRolesMappingSlot = 220;

      for (const { pauseType, role } of LINEA_ROLLUP_PAUSE_TYPES_ROLES) {
        const slot = generateKeccak256(["uint8", "uint256"], [pauseType, pauseTypeRolesMappingSlot]);
        const roleInMapping = await ethers.provider.getStorage(upgradedContract.getAddress(), slot);
        expect(roleInMapping).to.equal(role);
      }

      for (const { pauseType, role } of LINEA_ROLLUP_UNPAUSE_TYPES_ROLES) {
        const slot = generateKeccak256(["uint8", "uint256"], [pauseType, unpauseTypeRolesMappingSlot]);
        const roleInMapping = await ethers.provider.getStorage(upgradedContract.getAddress(), slot);
        expect(roleInMapping).to.equal(role);
      }
    });
  });
});
