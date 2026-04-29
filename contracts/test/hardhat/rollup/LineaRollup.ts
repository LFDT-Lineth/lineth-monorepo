import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time as networkTime } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES, LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES } from "contracts/common/constants";
import { AddressFilter, CallForwardingProxy, LineaRollup__factory, TestLineaRollup } from "contracts/typechain-types";
import { ethers, upgrades } from "hardhat";

import { ensureKzgSetup } from "./helpers";
import {
  deployCallForwardingProxy,
  deployForcedTransactionGatewayFixture,
  deployLineaRollupFixture,
  expectSuccessfulFinalizeViaCallForwarder,
  getAccountsFixture,
  getRoleAddressesFixture,
  sendBlobTransactionViaCallForwarder,
} from "./helpers";
import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json";
import blobAggregatedProof1To155 from "../_testData/compressedDataEip4844/aggregatedProof-1-155.json";
import {
  ADDRESS_ZERO,
  FALLBACK_OPERATOR_ADDRESS,
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
  DEFAULT_ADMIN_ROLE,
  DEFAULT_LAST_FINALIZED_TIMESTAMP,
  SIX_MONTHS_IN_SECONDS,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  FORCED_TRANSACTION_FEE,
  SET_ADDRESS_FILTER_ROLE,
  MAX_GAS_LIMIT,
  INITIALIZED_ALREADY_MESSAGE,
} from "../common/constants";
import { deployUpgradableFromFactory, reinitializeUpgradeableProxy } from "../common/deployment";
import {
  calculateRollingHash,
  encodeData,
  generateRandomBytes,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateBlobParentShnarfData,
  calculateLastFinalizedState,
  expectNoEvent,
  expectEventDirectFromReceiptData,
  computeGenesisShnarf,
} from "../common/helpers";
import { LineaRollupInitializationData, PauseTypeRole } from "../common/types";

ensureKzgSetup();

describe("Linea Rollup contract", () => {
  let lineaRollup: TestLineaRollup;
  let verifier: string;
  let callForwardingProxy: CallForwardingProxy;
  let yieldManager: string;

  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let alternateShnarfProviderAddress: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];
  let addressFilterAddress: string;
  let addressFilter: AddressFilter;

  const { compressedData, expectedX, expectedY, parentStateRootHash } = firstCompressedDataContent;

  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateShnarfProviderAddress } =
      await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getRoleAddressesFixture);
  });

  beforeEach(async () => {
    ({ addressFilter, verifier, lineaRollup, yieldManager } = await loadFixture(deployForcedTransactionGatewayFixture));
    addressFilterAddress = await addressFilter.getAddress();
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
    // Helper to create default initialization data (type inferred to match contract expectations)
    const createDefaultInitData = () => ({
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
      addressFilter: addressFilterAddress,
    });

    type InitData = ReturnType<typeof createDefaultInitData>;

    // Helper to deploy with custom init data and args
    const deployLineaRollupWithConfig = (
      initData: InitData,
      fallbackOperator: string = FALLBACK_OPERATOR_ADDRESS,
      contractName: string = "TestLineaRollup",
    ) =>
      deployUpgradableFromFactory(contractName, [initData, fallbackOperator, yieldManager], {
        initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      description: string;
      getInitData: () => InitData;
      fallbackOperator?: string;
      contractName?: string;
    }> = [
      {
        description: "verifier address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), defaultVerifier: ADDRESS_ZERO, roleAddresses }),
        contractName: "src/rollup/LineaRollup.sol:LineaRollup",
      },
      {
        description: "the liveness recovery operator address is zero address",
        getInitData: createDefaultInitData,
        fallbackOperator: ADDRESS_ZERO,
      },
      {
        description: "the default admin address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), defaultAdmin: ADDRESS_ZERO }),
      },
      {
        description: "an operator address is zero address",
        getInitData: () => ({
          ...createDefaultInitData(),
          roleAddresses: [{ addressWithRole: ADDRESS_ZERO, role: DEFAULT_ADMIN_ROLE }, ...roleAddresses.slice(1)],
        }),
      },
      {
        description: "the address filter address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), addressFilter: ADDRESS_ZERO }),
      },
    ];

    zeroAddressValidationCases.forEach(({ description, getInitData, fallbackOperator, contractName }) => {
      it(`Should revert if ${description}`, async () => {
        const deployCall = deployLineaRollupWithConfig(getInitData(), fallbackOperator, contractName);
        await expectRevertWithCustomError(lineaRollup, deployCall, "ZeroAddressNotAllowed");
      });
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
      // This test needs explicit BigInt types for event verification
      const initializationData: LineaRollupInitializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: BigInt(INITIAL_MIGRATION_BLOCK),
        genesisTimestamp: BigInt(GENESIS_L2_TIMESTAMP),
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINEA_ROLLUP_V8_PAUSE_TYPES_ROLES as unknown as PauseTypeRole[],
        unpauseTypeRoles: LINEA_ROLLUP_V8_UNPAUSE_TYPES_ROLES as unknown as PauseTypeRole[],
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      };

      const expectedAsTuple = [
        initializationData.initialStateRootHash,
        initializationData.initialL2BlockNumber,
        initializationData.genesisTimestamp,
        initializationData.defaultVerifier,
        initializationData.rateLimitPeriodInSeconds,
        initializationData.rateLimitAmountInWei,
        initializationData.roleAddresses.map((r) => [r.addressWithRole, r.role]),
        initializationData.pauseTypeRoles.map((p) => [BigInt(p.pauseType), p.role]),
        initializationData.unpauseTypeRoles.map((p) => [BigInt(p.pauseType), p.role]),
        initializationData.defaultAdmin,
        initializationData.shnarfProvider,
        initializationData.addressFilter,
      ];

      const lineaRollup = await deployUpgradableFromFactory(
        "src/rollup/LineaRollup.sol:LineaRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      const receipt = await lineaRollup.deploymentTransaction()?.wait();

      await expectEventDirectFromReceiptData(
        lineaRollup,
        receipt!,
        "LineaRollupBaseInitialized",
        [ethers.zeroPadBytes(ethers.toUtf8Bytes("8.0"), 8), expectedAsTuple, computeGenesisShnarf(parentStateRootHash)],
        38,
      );

      expect(await lineaRollup.stateRootHashes(INITIAL_MIGRATION_BLOCK)).to.be.equal(parentStateRootHash);
    });

    it("Should assign the VERIFIER_SETTER_ROLE to both SecurityCouncil and Operator", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
      };

      const lineaRollup = await deployLineaRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LineaRollup.sol:LineaRollup",
      );

      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
      expect(await lineaRollup.hasRole(VERIFIER_SETTER_ROLE, operator.address)).to.be.true;
    });

    it("Should assign the passed in shnarfProvider address", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        shnarfProvider: alternateShnarfProviderAddress.address,
      };

      const lineaRollup = await deployLineaRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LineaRollup.sol:LineaRollup",
      );

      expect(await lineaRollup.shnarfProvider()).to.equal(alternateShnarfProviderAddress.address);
    });

    it("Should assign the passed in addressFilter address", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
      };

      const lineaRollup = await deployLineaRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LineaRollup.sol:LineaRollup",
      );

      expect(await lineaRollup.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Should have the lineaRollup address as the shnarfProvider", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      const lineaRollupAddress = await lineaRollup.getAddress();

      expect(await lineaRollup.shnarfProvider()).to.equal(lineaRollupAddress);
    });

    it("Should have the correct contract version", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      expect(await lineaRollup.CONTRACT_VERSION()).to.equal("8.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      ({ verifier, lineaRollup } = await loadFixture(deployLineaRollupFixture));
      const initData = { ...createDefaultInitData(), roleAddresses };
      const initializeCall = lineaRollup.initialize(initData, FALLBACK_OPERATOR_ADDRESS, yieldManager);

      await expectRevertWithCustomError(lineaRollup, initializeCall, "InitializedVersionWrong", [0, 9]);
    });
  });

  describe("Upgrading / reinitialisation", () => {
    beforeEach(async () => {
      // Simulate a pre-upgrade state by lowering the initialized version to allow reinitializer(9) to run
      await lineaRollup.setSlotValue(0, 8);
    });

    it("Should revert if the forced transaction fee is zero", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(
        lineaRollup,
        LineaRollup__factory.abi,
        "reinitializeLineaRollupV9",
        [0n, addressFilterAddress],
      );

      await expectRevertWithCustomError(lineaRollup, upgradeCall, "ZeroValueNotAllowed");
    });

    it("Should revert if the address filter address is zero address", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(
        lineaRollup,
        LineaRollup__factory.abi,
        "reinitializeLineaRollupV9",
        [FORCED_TRANSACTION_FEE, ADDRESS_ZERO],
      );

      await expectRevertWithCustomError(lineaRollup, upgradeCall, "ZeroAddressNotAllowed");
    });

    it("Should set the next forced transaction number to 1", async () => {
      await reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await lineaRollup.nextForcedTransactionNumber()).to.equal(1n);
    });

    it("Should emit the AddressFilterChanged event when the address filter is set", async () => {
      await expectEvent(
        lineaRollup,
        reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
          FORCED_TRANSACTION_FEE,
          addressFilterAddress,
        ]),
        "AddressFilterChanged",
        [ADDRESS_ZERO, addressFilterAddress],
      );
    });

    it("Should emit the ForcedTransactionFeeSet event when the address filter is set", async () => {
      await expectEvent(
        lineaRollup,
        reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
          FORCED_TRANSACTION_FEE,
          addressFilterAddress,
        ]),
        "ForcedTransactionFeeSet",
        [FORCED_TRANSACTION_FEE],
      );
    });

    it("Should set the address filter", async () => {
      await reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await lineaRollup.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Next contract version number should be 8.0", async () => {
      await reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await lineaRollup.CONTRACT_VERSION()).to.equal("8.0");
    });

    it("Next contract version number should be 8.0", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(
        lineaRollup,
        LineaRollup__factory.abi,
        "reinitializeLineaRollupV9",
        [FORCED_TRANSACTION_FEE, addressFilterAddress],
      );

      const previousVersion = ethers.zeroPadBytes(ethers.toUtf8Bytes("7.1"), 8);
      const newVersion = ethers.zeroPadBytes(ethers.toUtf8Bytes("8.0"), 8);

      await expectEvent(lineaRollup, upgradeCall, "LineaRollupVersionChanged", [previousVersion, newVersion]);
    });

    it("Fails to reinitialize twice", async () => {
      await reinitializeUpgradeableProxy(lineaRollup, LineaRollup__factory.abi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      const secondUpgradeCall = reinitializeUpgradeableProxy(
        lineaRollup,
        LineaRollup__factory.abi,
        "reinitializeLineaRollupV9",
        [FORCED_TRANSACTION_FEE, addressFilterAddress],
      );

      await expectRevertWithReason(secondUpgradeCall, INITIALIZED_ALREADY_MESSAGE);
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

      await expectRevertWithReason(
        lineaRollup.connect(nonAuthorizedAccount).unsetVerifierAddress(0),
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

  describe("Change address filter", () => {
    beforeEach(async () => {
      await lineaRollup.connect(securityCouncil).grantRole(SET_ADDRESS_FILTER_ROLE, securityCouncil.address);
    });

    it("Should revert if the address filter is the zero address", async () => {
      const setAddressFilterCall = lineaRollup.connect(securityCouncil).setAddressFilter(ADDRESS_ZERO);
      await expectRevertWithCustomError(lineaRollup, setAddressFilterCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if the caller has not the SET_ADDRESS_FILTER_ROLE", async () => {
      const setAddressFilterCall = lineaRollup.connect(nonAuthorizedAccount).setAddressFilter(addressFilterAddress);
      await expectRevertWithReason(
        setAddressFilterCall,
        buildAccessErrorMessage(nonAuthorizedAccount, SET_ADDRESS_FILTER_ROLE),
      );
    });

    it("Should set the new address filter", async () => {
      const newAddressFilter = ethers.getAddress(generateRandomBytes(20));
      await lineaRollup.connect(securityCouncil).setAddressFilter(newAddressFilter);
      expect(await lineaRollup.addressFilter()).to.be.equal(newAddressFilter);
    });

    it("Should emit the AddressFilterChanged event when the address filter is set", async () => {
      const newAddressFilter = ethers.getAddress(generateRandomBytes(20));
      await expectEvent(
        lineaRollup,
        lineaRollup.connect(securityCouncil).setAddressFilter(newAddressFilter),
        "AddressFilterChanged",
        [addressFilterAddress, newAddressFilter],
      );
    });

    it("Should not emit the event if the address filter is the same", async () => {
      const newAddressFilter = ethers.getAddress(generateRandomBytes(20));

      await expectEvent(
        lineaRollup,
        lineaRollup.connect(securityCouncil).setAddressFilter(newAddressFilter),
        "AddressFilterChanged",
        [addressFilterAddress, newAddressFilter],
      );

      await expectNoEvent(
        lineaRollup,
        lineaRollup.connect(securityCouncil).setAddressFilter(newAddressFilter),
        "AddressFilterChanged",
      );
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
    const expectedLastFinalizedState = calculateLastFinalizedState(
      0n,
      HASH_ZERO,
      0n,
      HASH_ZERO,
      DEFAULT_LAST_FINALIZED_TIMESTAMP,
    );

    it("Should revert if trying to set liveness recovery operator role before six months have passed", async () => {
      const initialBlock = await ethers.provider.getBlock("latest");

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, BigInt(initialBlock!.timestamp)),
        "LastFinalizationTimeNotLapsed",
      );
    });

    it("Should revert if the time has passed and the last finalized timestamp does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(0n, HASH_ZERO, 0n, HASH_ZERO, 123456789n);

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, 123456789n),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 message number does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(
        1n,
        HASH_ZERO,
        0n,
        HASH_ZERO,
        DEFAULT_LAST_FINALIZED_TIMESTAMP,
      );

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(1n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 rolling hash does not match", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);
      const random32Bytes = generateRandomBytes(32);
      const actualSentState = calculateLastFinalizedState(
        0n,
        random32Bytes,
        0n,
        HASH_ZERO,
        DEFAULT_LAST_FINALIZED_TIMESTAMP,
      );

      await expectRevertWithCustomError(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, random32Bytes, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should set the liveness recovery operator role after six months have passed", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS)).to.be.true;
    });

    it("Should not expect a second event with liveness operator setting", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await lineaRollup.hasRole(OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS)).to.be.true;

      await expectNoEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
      );
    });

    it("Should revert if trying to renounce role as liveness recovery operator", async () => {
      await networkTime.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        lineaRollup,
        lineaRollup.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
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
      await expectEvent(lineaRollup, renounceCall, "RoleRevoked", args);
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
        upgradedContract.setLivenessRecoveryOperator(0n, HASH_ZERO, 0n, HASH_ZERO, DEFAULT_LAST_FINALIZED_TIMESTAMP),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, forwardingProxyAddress],
      );

      // Submit 2 blobs
      await sendBlobTransactionViaCallForwarder(upgradedContract, 0, 2, forwardingProxyAddress);
      // Submit another 2 blobs
      await sendBlobTransactionViaCallForwarder(upgradedContract, 2, 4, forwardingProxyAddress);

      // Finalize 4 blobs
      await expectSuccessfulFinalizeViaCallForwarder({
        context: {
          callforwarderAddress: forwardingProxyAddress,
          upgradedContract,
        },
        proofConfig: {
          proofData: blobAggregatedProof1To155,
          blobParentShnarfIndex: 4,
          shnarfDataGenerator: generateBlobParentShnarfData,
          isMultiple: false,
        },
      });
    });
  });
});
