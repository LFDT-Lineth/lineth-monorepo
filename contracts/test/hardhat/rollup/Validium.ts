import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import {
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "contracts/common/constants";
import { AddressFilter, TestValidium, Validium__factory } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { deployValidiumFixture, getAccountsFixture, getValidiumRoleAddressesFixture } from "./helpers";
import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json";
import {
  ADDRESS_ZERO,
  GENERAL_PAUSE_TYPE,
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
  VALIDIUM_INITIALIZE_SIGNATURE,
  MAX_GAS_LIMIT,
} from "../common/constants";
import { deployUpgradableFromFactory, reinitializeUpgradeableProxy } from "../common/deployment";
import {
  calculateRollingHash,
  generateRandomBytes,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectRevertWhenPaused,
  computeDataRollingHash,
} from "../common/helpers";

describe("Validium contract", () => {
  let validium: TestValidium;
  let addressFilter: AddressFilter;
  let verifier: string;

  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let alternateDataRollingHashProviderAddress: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];
  let addressFilterAddress: string;

  const { parentStateRootHash } = firstCompressedDataContent;
  // Genesis accumulator is the deterministic anchor keccak256(EMPTY_HASH || initialBlockHash).
  const prevShnarf = () => computeDataRollingHash(HASH_ZERO, parentStateRootHash);
  const expectedShnarf = generateRandomBytes(32);
  const secondExpectedShnarf = generateRandomBytes(32);

  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateDataRollingHashProviderAddress } =
      await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getValidiumRoleAddressesFixture);
  });

  beforeEach(async () => {
    ({ verifier, validium, addressFilter } = await loadFixture(deployValidiumFixture));
    addressFilterAddress = await addressFilter.getAddress();
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: string) => {
      return admin.sendTransaction({ to: await validium.getAddress(), value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the validium contract through the fallback", async () => {
      await expect(sendEthToContract(EMPTY_CALLDATA)).to.be.reverted;
    });

    it("Should fail to send eth to the validium contract through the receive function", async () => {
      await expect(sendEthToContract("0x1234")).to.be.reverted;
    });
  });

  describe("Initialisation", () => {
    // Helper to create default initialization data (type inferred to match contract expectations)
    const createDefaultInitData = () => ({
      initialBlockHash: parentStateRootHash,
      initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
      genesisTimestamp: GENESIS_L2_TIMESTAMP,
      defaultVerifier: verifier,
      rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
      rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
      roleAddresses: [...roleAddresses.slice(1)],
      pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
      unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
      verifierKeys: [] as string[],
      defaultAdmin: securityCouncil.address,
      dataRollingHashProvider: ADDRESS_ZERO,
      addressFilter: addressFilterAddress,
    });

    type InitData = ReturnType<typeof createDefaultInitData>;

    // Helper to deploy with custom init data
    const deployValidiumWithConfig = (initData: InitData, contractName: string = "TestValidium") =>
      deployUpgradableFromFactory(contractName, [initData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      description: string;
      getInitData: () => InitData;
      contractName?: string;
    }> = [
      {
        description: "verifier address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), defaultVerifier: ADDRESS_ZERO, roleAddresses }),
        contractName: "src/rollup/Validium.sol:Validium",
      },
      {
        description: "the default admin address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), defaultAdmin: ADDRESS_ZERO }),
      },
      {
        description: "the address filter address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), addressFilter: ADDRESS_ZERO }),
      },
    ];

    zeroAddressValidationCases.forEach(({ description, getInitData, contractName }) => {
      it(`Should revert if ${description}`, async () => {
        const deployCall = deployValidiumWithConfig(getInitData(), contractName);
        await expectRevertWithCustomError(validium, deployCall, "ZeroAddressNotAllowed");
      });
    });

    it("Should store verifier address in storage", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.verifiers(0)).to.be.equal(verifier);
    });

    it("Should assign the OPERATOR_ROLE to operator addresses", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the VERIFIER_SETTER_ROLE to securityCouncil addresses", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
    });

    it("Should assign the VERIFIER_UNSETTER_ROLE to securityCouncil addresses", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.hasRole(VERIFIER_UNSETTER_ROLE, securityCouncil.address)).to.be.true;
    });

    it("Should store the starting block hash in storage for the first block number", async () => {
      const initializationData = {
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.blockHashes(INITIAL_MIGRATION_BLOCK)).to.be.equal(parentStateRootHash);
    });

    it("Should assign the VERIFIER_SETTER_ROLE to both SecurityCouncil and Operator", async () => {
      const initializationData = {
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
      expect(await validium.hasRole(VERIFIER_SETTER_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the passed in dataRollingHashProvider address", async () => {
      const initializationData = {
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: alternateDataRollingHashProviderAddress.address,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.dataRollingHashProvider()).to.equal(alternateDataRollingHashProviderAddress.address);
    });

    it("Should assign the passed in addressFilter address", async () => {
      const initializationData = {
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: alternateDataRollingHashProviderAddress.address,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Should have the validium address as the dataRollingHashProvider", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      const validiumAddress = await validium.getAddress();

      expect(await validium.dataRollingHashProvider()).to.equal(validiumAddress);
    });

    it("Should have the correct contract version", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.CONTRACT_VERSION()).to.equal("3.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      const initializeCall = validium.initialize({
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      });

      await expectRevertWithCustomError(validium, initializeCall, "InitializedVersionWrong", [0, 3]);
    });

    it("Should revert if initialize is called on an already deployed (version 1) proxy", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      // Simulate a Validium 2.0 proxy, which was initialized with `initializer` (version 1).
      await validium.setSlotValue(0, 1);

      const initializeCall = validium.initialize({
        initialBlockHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      });

      await expectRevertWithCustomError(validium, initializeCall, "InitializedVersionWrong", [0, 1]);
    });
  });

  describe("Upgrading / reinitialisation V3", () => {
    const legacyFinalizedShnarf = generateRandomBytes(32);

    beforeEach(async () => {
      // Simulate a deployed Validium 2.0 proxy: initialized with `initializer` (version 1), holding a live
      // legacy finalized shnarf.
      await validium.setSlotValue(0, 1);
      await validium.setLegacyFinalizedShnarf(legacyFinalizedShnarf);
    });

    it("Should revert reinitializeValidiumV3 on a freshly initialized proxy", async () => {
      ({ validium } = await loadFixture(deployValidiumFixture));
      await expectRevertWithReason(validium.reinitializeValidiumV3(), INITIALIZED_ALREADY_MESSAGE);
    });

    it("Should emit LineaRollupVersionChanged 2.0 to 3.0", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(validium, Validium__factory.abi, "reinitializeValidiumV3", []);

      const previousVersion = ethers.zeroPadBytes(ethers.toUtf8Bytes("2.0"), 8);
      const newVersion = ethers.zeroPadBytes(ethers.toUtf8Bytes("3.0"), 8);

      await expectEvent(validium, upgradeCall, "LineaRollupVersionChanged", [previousVersion, newVersion]);
      expect(await validium.CONTRACT_VERSION()).to.equal("3.0");
    });

    it("Migrates currentFinalizedShnarf_DEPRECATED into currentDataRollingHash and anchors it", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(validium, Validium__factory.abi, "reinitializeValidiumV3", []);

      await expectEvent(validium, upgradeCall, "LegacyShnarfMigrated", [legacyFinalizedShnarf]);

      expect(await validium.currentFinalizedShnarf_DEPRECATED()).to.equal(HASH_ZERO);
      expect(await validium.currentDataRollingHash()).to.equal(legacyFinalizedShnarf);
      expect(await validium.currentDataTailBytes()).to.equal(0n);
      expect(await validium.dataRollingHashExists(legacyFinalizedShnarf)).to.equal(1n);
    });

    it("Allows submissions to chain from the migrated legacy shnarf", async () => {
      await reinitializeUpgradeableProxy(validium, Validium__factory.abi, "reinitializeValidiumV3", []);

      const nextDataRollingHash = generateRandomBytes(32);
      await expect(validium.connect(operator).acceptDataRollingHash(legacyFinalizedShnarf, nextDataRollingHash)).to.not
        .be.reverted;
      expect(await validium.dataRollingHashExists(nextDataRollingHash)).to.equal(1n);
    });

    it("Should revert if there is no legacy shnarf to migrate", async () => {
      await validium.setLegacyFinalizedShnarf(HASH_ZERO);

      const upgradeCall = reinitializeUpgradeableProxy(validium, Validium__factory.abi, "reinitializeValidiumV3", []);

      await expectRevertWithCustomError(validium, upgradeCall, "ZeroHashNotAllowed");
    });

    it("Fails to reinitialize twice", async () => {
      await reinitializeUpgradeableProxy(validium, Validium__factory.abi, "reinitializeValidiumV3", []);

      const secondUpgradeCall = reinitializeUpgradeableProxy(
        validium,
        Validium__factory.abi,
        "reinitializeValidiumV3",
        [],
      );

      await expectRevertWithReason(secondUpgradeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Change verifier address", () => {
    it("Should revert if the caller has not the VERIFIER_SETTER_ROLE", async () => {
      const setVerifierCall = validium.connect(nonAuthorizedAccount).setVerifierAddress(verifier, 2);

      await expectRevertWithReason(
        setVerifierCall,
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_SETTER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      await validium.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      const setVerifierCall = validium.connect(securityCouncil).setVerifierAddress(ADDRESS_ZERO, 2);
      await expectRevertWithCustomError(validium, setVerifierCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new verifier address", async () => {
      await validium.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      await validium.connect(securityCouncil).setVerifierAddress(verifier, 2);
      expect(await validium.verifiers(2)).to.be.equal(verifier);
    });

    it("Should remove verifier address in storage ", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      await validium.connect(securityCouncil).unsetVerifierAddress(0);

      expect(await validium.verifiers(0)).to.be.equal(ADDRESS_ZERO);
    });

    it("Should revert when removing verifier address if the caller has not the VERIFIER_UNSETTER_ROLE ", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));

      await expectRevertWithReason(
        validium.connect(nonAuthorizedAccount).unsetVerifierAddress(0),
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_UNSETTER_ROLE),
      );
    });

    it("Should emit the correct event", async () => {
      await validium.connect(securityCouncil).grantRole(VERIFIER_SETTER_ROLE, securityCouncil.address);

      const oldVerifierAddress = await validium.verifiers(2);

      const setVerifierCall = validium.connect(securityCouncil).setVerifierAddress(verifier, 2);
      let expectedArgs = [verifier, 2, securityCouncil.address, oldVerifierAddress];

      await expectEvent(validium, setVerifierCall, "VerifierAddressChanged", expectedArgs);

      await validium.connect(securityCouncil).unsetVerifierAddress(2);

      const unsetVerifierCall = validium.connect(securityCouncil).unsetVerifierAddress(2);
      expectedArgs = [ADDRESS_ZERO, 2, securityCouncil.address, oldVerifierAddress];

      await expectEvent(validium, unsetVerifierCall, "VerifierAddressChanged", expectedArgs);
    });
  });

  describe("Data submission tests", () => {
    beforeEach(async () => {
      await validium.setLastFinalizedBlock(0);
    });

    it("Should fail when the parent dataRollingHash is not anchored", async () => {
      const nonExistingParent = generateRandomBytes(32);
      const dataRollingHash = generateRandomBytes(32);

      const asyncCall = validium
        .connect(operator)
        .acceptDataRollingHash(nonExistingParent, dataRollingHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, asyncCall, "ParentDataRollingHashNotAnchored", [nonExistingParent]);
    });

    it("Should succesfully submit 1 compressed data chunk setting values", async () => {
      await expect(
        validium.connect(operator).acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      const dataRollingHashExists = await validium.dataRollingHashExists(expectedShnarf);
      expect(dataRollingHashExists).to.equal(1n);
    });

    it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
      await expect(
        validium.connect(operator).acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      await expect(
        validium
          .connect(operator)
          .acceptDataRollingHash(expectedShnarf, secondExpectedShnarf, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      let dataRollingHashExists = await validium.dataRollingHashExists(expectedShnarf);
      expect(dataRollingHashExists).to.equal(1n);
      dataRollingHashExists = await validium.dataRollingHashExists(secondExpectedShnarf);
      expect(dataRollingHashExists).to.equal(1n);
    });

    it("Should emit an event while submitting 1 compressed data chunk", async () => {
      const submitDataCall = validium
        .connect(operator)
        .acceptDataRollingHash(prevShnarf(), secondExpectedShnarf, { gasLimit: MAX_GAS_LIMIT });
      const eventArgs = [prevShnarf(), secondExpectedShnarf];

      await expectEvent(validium, submitDataCall, "DataSubmittedV4", eventArgs);
    });

    it("Should fail to submit where the submitted dataRollingHash is HASH_ZERO", async () => {
      const submitDataCall = validium
        .connect(operator)
        .acceptDataRollingHash(prevShnarf(), HASH_ZERO, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "DataRollingHashSubmissionIsZeroHash", []);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const submitDataCall = validium
        .connect(nonAuthorizedAccount)
        .acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    // Parameterized pause type tests for Validium data submission
    const validiumSubmissionPauseTypes = [
      { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
      { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
    ];

    validiumSubmissionPauseTypes.forEach(({ pauseType, name }) => {
      it(`Should revert if ${name} is enabled`, async () => {
        await validium.connect(securityCouncil).pauseByType(pauseType);

        const submitDataCall = validium
          .connect(operator)
          .acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

        await expectRevertWhenPaused(validium, submitDataCall, pauseType);
      });
    });

    it("Should revert with DataRollingHashAlreadyAnchored when submitting the same dataRollingHash twice", async () => {
      await validium.connect(operator).acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      const submitDataCall = validium
        .connect(operator)
        .acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "DataRollingHashAlreadyAnchored", [expectedShnarf]);
    });

    it("Should revert with DataRollingHashAlreadyAnchored when re-anchoring an existing dataRollingHash from a different parent", async () => {
      // Anchor both the target dataRollingHash and a second parent, so the parent-anchored check passes
      // and the already-anchored check is what fires.
      await validium.connect(operator).acceptDataRollingHash(prevShnarf(), expectedShnarf, { gasLimit: MAX_GAS_LIMIT });
      await validium
        .connect(operator)
        .acceptDataRollingHash(expectedShnarf, secondExpectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      const submitDataCall = validium
        .connect(operator)
        .acceptDataRollingHash(secondExpectedShnarf, expectedShnarf, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "DataRollingHashAlreadyAnchored", [expectedShnarf]);
    });
  });

  describe("Validate L2 computed rolling hash", () => {
    it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, 0n),
        "MissingMessageNumberForRollingHash",
        [l1RollingHash],
      );
    });

    it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = HASH_ZERO;

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, 0n),
        "MissingRollingHashForMessageNumber",
        [l1MessageNumber],
      );
    });

    it("Should revert if l1RollingHash does not exist on L1", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, 0n),
        "L1RollingHashDoesNotExistOnL1",
        [l1MessageNumber, l1RollingHash],
      );
    });

    it("Should succeed if l1 message number == 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = HASH_ZERO;
      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, 0n)).to.not.be.reverted;
    });

    it("Should succeed if l1 message number != 0, l1 rolling hash is not empty and exists on L1", async () => {
      const l1MessageNumber = 1n;
      const messageHash = generateRandomBytes(32);

      await validium.addRollingHash(l1MessageNumber, messageHash);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, 0n)).to.not.be.reverted;
    });

    it("Should revert if l1 message number is lower than the last finalized message number", async () => {
      const l1MessageNumber = 5n;
      const messageHash = generateRandomBytes(32);

      await validium.addRollingHash(l1MessageNumber, messageHash);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      const lastFinalizedMessageNumber = l1MessageNumber + 1n;
      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, lastFinalizedMessageNumber),
        "L1RollingHashMessageNumberDecreasing",
        [l1MessageNumber, lastFinalizedMessageNumber],
      );
    });

    it("Should succeed if l1 message number is equal to or greater than the last finalized message number", async () => {
      const l1MessageNumber = 5n;
      const messageHash = generateRandomBytes(32);

      await validium.addRollingHash(l1MessageNumber, messageHash);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, l1MessageNumber)).to.not.be
        .reverted;
      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash, l1MessageNumber - 1n)).to.not
        .be.reverted;
    });
  });
});
