import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import * as kzg from "c-kzg";
import { expect } from "chai";
import { ethers, upgrades } from "hardhat";

import blobAggregatedProof1To155 from "../_testData/compressedDataEip4844/aggregatedProof-1-155.json";
import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json";
import secondCompressedDataContent from "../_testData/compressedData/blocks-47-81.json";
import fourthCompressedDataContent from "../_testData/compressedData/blocks-115-155.json";

import {
  LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
  PAUSE_STATE_DATA_SUBMISSION_ROLE,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "contracts/common/constants";
import { CallForwardingProxy, TestLineaRollup } from "contracts/typechain-types";
import {
  deployCallForwardingProxy,
  deployLineaRollupFixture,
  expectSuccessfulFinalizeViaCallForwarder,
  getAccountsFixture,
  getRoleAddressesFixture,
  sendBlobTransactionViaCallForwarder,
} from "./helpers";
import {
  ADDRESS_ZERO,
  FALLBACK_OPERATOR_ADDRESS,
  GENERAL_PAUSE_TYPE,
  HASH_WITHOUT_ZERO_FIRST_BYTE,
  HASH_ZERO,
  INITIAL_MIGRATION_BLOCK,
  INITIAL_WITHDRAW_LIMIT,
  ONE_DAY_IN_SECONDS,
  OPERATOR_ROLE,
  VERIFIER_SETTER_ROLE,
  VERIFIER_UNSETTER_ROLE,
  GENESIS_L2_TIMESTAMP,
  EMPTY_CALLDATA,
  INITIALIZED_ALREADY_MESSAGE,
  DEFAULT_ADMIN_ROLE,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  SIX_MONTHS_IN_SECONDS,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  UNPAUSE_STATE_DATA_SUBMISSION_ROLE,
  MAX_GAS_LIMIT,
} from "../common/constants";
import { deployUpgradableFromFactory } from "../common/deployment";
import {
  calculateRollingHash,
  encodeData,
  generateRandomBytes,
  generateCallDataSubmission,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobParentShnarfData,
  calculateLastFinalizedState,
  generateKeccak256,
} from "../common/helpers";
import { CalldataSubmissionData } from "../common/types";
import { IPauseManager } from "contracts/typechain-types/src/_testing/unit/rollup/TestLineaRollup";
import { Typed } from "ethers";
import { IPermissionsManager } from "contracts/typechain-types/src/rollup/LineaRollup";

kzg.loadTrustedSetup(0, `${__dirname}/../_testData/trusted_setup.txt`);

describe("Linea Rollup contract", () => {
  let lineaRollup: TestLineaRollup;
  let verifier: string;
  let callForwardingProxy: CallForwardingProxy;
  let yieldManager: string;

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let alternateShnarfProviderAddress: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];
  let upgradePauseTypeRoles: Typed | IPauseManager.PauseTypeRoleStruct[] = [];
  let upgradeUnpauseTypeRoles: Typed | IPauseManager.PauseTypeRoleStruct[] = [];
  let upgradeArgs: (IPauseManager.PauseTypeRoleStruct[] | IPermissionsManager.RoleAddressStruct[])[] = [];
  let upgradeRoleAddresses: IPermissionsManager.RoleAddressStruct[];

  const { compressedData, prevShnarf, expectedShnarf, expectedX, expectedY, parentStateRootHash } =
    firstCompressedDataContent;
  const { expectedShnarf: secondExpectedShnarf } = secondCompressedDataContent;

  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateShnarfProviderAddress } =
      await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getRoleAddressesFixture);

    upgradeRoleAddresses = [
      {
        addressWithRole: securityCouncil.address,
        role: PAUSE_STATE_DATA_SUBMISSION_ROLE,
      },
      {
        addressWithRole: securityCouncil.address,
        role: UNPAUSE_STATE_DATA_SUBMISSION_ROLE,
      },
    ];

    upgradePauseTypeRoles = [{ pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, role: PAUSE_STATE_DATA_SUBMISSION_ROLE }];
    upgradeUnpauseTypeRoles = [
      { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, role: UNPAUSE_STATE_DATA_SUBMISSION_ROLE },
    ];
    upgradeArgs = [upgradeRoleAddresses, upgradePauseTypeRoles, upgradeUnpauseTypeRoles];
  });

  beforeEach(async () => {
    ({ verifier, lineaRollup, yieldManager } = await loadFixture(deployLineaRollupFixture));
  });

  describe("Upgrading", () => {
    it("Should be able to upgrade", async () => {
      // Deploy new LineaRollup implementation
      const newLineaRollupFactory = await ethers.getContractFactory(
        "src/_testing/unit/rollup/TestLineaRollup.sol:TestLineaRollup",
      );

      const newLineaRollup = await upgrades.upgradeProxy(lineaRollup, newLineaRollupFactory, {
        call: { fn: "reinitializeV8", args: upgradeArgs },
        kind: "transparent",
        unsafeAllowRenames: true,
        unsafeAllow: ["incorrect-initializer-order"],
      });

      await newLineaRollup.waitForDeployment();

      expect(await newLineaRollup.shnarfProvider()).to.equal(await lineaRollup.getAddress());
    });

    it("Should fail to upgrade twice", async () => {
      // Deploy new LineaRollup implementation
      const newLineaRollupFactory = await ethers.getContractFactory(
        "src/_testing/unit/rollup/TestLineaRollup.sol:TestLineaRollup",
      );

      const newLineaRollup = await upgrades.upgradeProxy(lineaRollup, newLineaRollupFactory, {
        call: { fn: "reinitializeV8", args: upgradeArgs },
        kind: "transparent",
        unsafeAllowRenames: true,
        unsafeAllow: ["incorrect-initializer-order"],
      });

      await newLineaRollup.waitForDeployment();

      expect(await newLineaRollup.shnarfProvider()).to.equal(await lineaRollup.getAddress());

      await expectRevertWithReason(
        upgrades.upgradeProxy(lineaRollup, newLineaRollupFactory, {
          call: { fn: "reinitializeV8", args: upgradeArgs },
          kind: "transparent",
          unsafeAllowRenames: true,
          unsafeAllow: ["incorrect-initializer-order"],
        }),
        "Initializable: contract is already initialized",
      );
    });

    it("Should fail to upgrade if not proxy admin", async () => {
      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup
          .connect(nonAuthorizedAccount)
          .reinitializeV8(upgradeRoleAddresses, upgradePauseTypeRoles, upgradeUnpauseTypeRoles),
        "CallerNotProxyAdmin",
      );
    });
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
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory(
        "src/rollup/LineaRollup.sol:LineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if the liveness recovery operator address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses.slice(1)],
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory(
        "TestLineaRollup",
        [initializationData, ADDRESS_ZERO, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

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
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: ADDRESS_ZERO,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory(
        "TestLineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

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
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory(
        "TestLineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should store verifier address in storage", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      expect(await lineaRollup.verifiers(0)).to.be.equal(verifier);
    });

    it("Should assign the OPERATOR_ROLE to operator addresses", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      expect(await lineaRollup.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the VERIFIER_SETTER_ROLE to securityCouncil addresses", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
    });

    it("Should assign the VERIFIER_UNSETTER_ROLE to securityCouncil addresses", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
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
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const lineaRollup = await deployUpgradableFromFactory(
        "src/rollup/LineaRollup.sol:LineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
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
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const lineaRollup = await deployUpgradableFromFactory(
        "src/rollup/LineaRollup.sol:LineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the passed in shnarfProvider address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: alternateShnarfProviderAddress.address,
      };

      const lineaRollup = await deployUpgradableFromFactory(
        "src/rollup/LineaRollup.sol:LineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      expect(await lineaRollup.shnarfProvider()).to.equal(alternateShnarfProviderAddress.address);
    });

    it("Should have the lineaRollup address as the shnarfProvider", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      const lineaRollupAddress = await lineaRollup.getAddress();

      expect(await lineaRollup.shnarfProvider()).to.equal(lineaRollupAddress);
    });

    it("Should have the correct contract version", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      expect(await lineaRollup.CONTRACT_VERSION()).to.equal("7.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      const initializeCall = lineaRollup.initialize(
        {
          initialStateRootHash: parentStateRootHash,
          initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
          genesisTimestamp: GENESIS_L2_TIMESTAMP,
          defaultVerifier: verifier,
          rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
          rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
          roleAddresses,
          pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES,
          unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
          defaultAdmin: securityCouncil.address,
          shnarfProvider: ADDRESS_ZERO,
        },
        FALLBACK_OPERATOR_ADDRESS,
        yieldManager,
      );

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
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      await lineaRollup.connect(securityCouncil).unsetVerifierAddress(0);

      expect(await lineaRollup.verifiers(0)).to.be.equal(ADDRESS_ZERO);
    });

    it("Should revert when removing verifier address if the caller has not the VERIFIER_UNSETTER_ROLE ", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));

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
        .submitDataAsCalldata(submissionData, prevShnarf, secondExpectedShnarf, { gasLimit: MAX_GAS_LIMIT });
      await expectRevertWithCustomError(lineaRollup, submitDataCall, "EmptySubmissionData");
    });

    it("Should fail when the parent shnarf does not exist", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);
      const nonExistingParentShnarf = generateRandomBytes(32);

      const wrongExpectedShnarf = generateKeccak256(
        ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
        [HASH_ZERO, HASH_ZERO, submissionData.finalStateRootHash, HASH_ZERO, HASH_ZERO],
      );

      const asyncCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, nonExistingParentShnarf, wrongExpectedShnarf, {
          gasLimit: MAX_GAS_LIMIT,
        });

      await expectRevertWithCustomError(lineaRollup, asyncCall, "ParentShnarfNotSubmitted", [nonExistingParentShnarf]);
    });

    it("Should succesfully submit 1 compressed data chunk setting values", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      await expect(
        lineaRollup
          .connect(operator)
          .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      const blobShnarfExists = await lineaRollup.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await expect(
        lineaRollup
          .connect(operator)
          .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      await expect(
        lineaRollup.connect(operator).submitDataAsCalldata(secondSubmissionData, expectedShnarf, secondExpectedShnarf, {
          gasLimit: MAX_GAS_LIMIT,
        }),
      ).to.not.be.reverted;

      const blobShnarfExists = await lineaRollup.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should emit an event while submitting 1 compressed data chunk", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });
      const eventArgs = [prevShnarf, expectedShnarf, submissionData.finalStateRootHash];

      await expectEvent(lineaRollup, submitDataCall, "DataSubmittedV3", eventArgs);
    });

    it("Should fail if the final state root hash is empty", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      submissionData.finalStateRootHash = HASH_ZERO;

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

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
          .submitDataAsCalldata(firstSubmissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      const wrongComputedShnarf = generateRandomBytes(32);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(secondSubmissionData, expectedShnarf, wrongComputedShnarf, { gasLimit: MAX_GAS_LIMIT });

      const eventArgs = [wrongComputedShnarf, secondExpectedShnarf];

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "FinalShnarfWrong", eventArgs);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const submitDataCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if STATE_DATA_SUBMISSION_PAUSE_TYPE is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "IsPaused", [STATE_DATA_SUBMISSION_PAUSE_TYPE]);
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
      await lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same data, differing block numbers", async () => {
      await lineaRollup
        .connect(operator)
        .submitDataAsCalldata(DATA_ONE, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      const [dataOneCopy] = generateCallDataSubmission(0, 1);

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(dataOneCopy, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(lineaRollup, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert when snarkHash is zero hash", async () => {
      const submissionData: CalldataSubmissionData = {
        ...DATA_ONE,
        snarkHash: HASH_ZERO,
      };

      const submitDataCall = lineaRollup
        .connect(operator)
        .submitDataAsCalldata(submissionData, prevShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(lineaRollup, submitDataCall, "FinalShnarfWrong", [
        expectedShnarf,
        "0xa6b52564082728b51bb81a4fa92cfb4ec3af8de3f18b5d68ec27b89eead93293",
      ]);
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

      expect(await lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: MAX_GAS_LIMIT })).to.equal(
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
        lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: MAX_GAS_LIMIT }),
        "FirstByteIsNotZero",
      );
    });

    it("Should revert if bytes length is not a multiple of 32", async () => {
      const compressedDataBytes = generateRandomBytes(56);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.calculateY(compressedDataBytes, expectedX, { gasLimit: MAX_GAS_LIMIT }),
        "BytesLengthNotMultipleOf32",
      );
    });
  });

  describe("liveness recovery operator Role", () => {
    const expectedLastFinalizedState = calculateLastFinalizedState(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP);

    it("Should revert if trying to set liveness recovery operator role before six months have passed", async () => {
      const initialBlock = await ethers.provider.getBlock("latest");

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, BigInt(initialBlock!.timestamp)),
        "LastFinalizationTimeNotLapsed",
      );
    });

    it("Should revert if the time has passed and the last finalized timestamp does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(0n, HASH_ZERO, 123456789n);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 123456789n),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 message number does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(1n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(1n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
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
        lineaRollup.setLivenessRecoveryOperator(0n, random32Bytes, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should set the liveness recovery operator role after six months have passed", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS)).to.be.true;
    });

    it("Should revert if trying to renounce role as liveness recovery operator", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS)).to.be.true;

      const renounceCall = lineaRollup.renounceRole(OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS);

      await expectRevertWithCustomError(lineaRollup, renounceCall, "OnlyNonLivenessRecoveryOperator");
    });

    it("Should renounce role if not liveness recovery operator", async () => {
      expect(await lineaRollup.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;

      const renounceCall = lineaRollup.connect(operator).renounceRole(OPERATOR_ROLE, operator.address);
      const args = [OPERATOR_ROLE, operator.address, operator.address];
      expectEvent(lineaRollup, renounceCall, "RoleRevoked", args);
    });

    it("Should fail to accept ETH on the CallForwardingProxy receive function", async () => {
      callForwardingProxy = await deployCallForwardingProxy(await lineaRollup.getAddress());
      const forwardingProxyAddress = await callForwardingProxy.getAddress();

      const tx = {
        to: forwardingProxyAddress,
        value: ethers.parseEther("0.1"),
      };

      await expectRevertWithReason(admin.sendTransaction(tx), "ETH not accepted");
    });

    it("Should be able to submit blobs and finalize via callforwarding proxy", async () => {
      callForwardingProxy = await deployCallForwardingProxy(await lineaRollup.getAddress());
      const forwardingProxyAddress = await callForwardingProxy.getAddress();

      expect(await lineaRollup.currentL2BlockNumber()).to.equal(0);

      // Deploy new LineaRollup implementation
      const newLineaRollupFactory = await ethers.getContractFactory(
        "src/_testing/unit/rollup/TestLineaRollup.sol:TestLineaRollup",
      );
      const newLineaRollup = await upgrades.upgradeProxy(lineaRollup, newLineaRollupFactory, {
        unsafeAllowRenames: true,
        unsafeAllow: ["incorrect-initializer-order"],
      });

      const upgradedContract = await newLineaRollup.waitForDeployment();

      await upgradedContract.setLivenessRecoveryOperatorAddress(forwardingProxyAddress);

      // Grants deployed callforwarding proxy as operator
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      await expectEvent(
        upgradedContract,
        upgradedContract.setLivenessRecoveryOperator(0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
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
  });
});
