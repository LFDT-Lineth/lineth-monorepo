import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Abi, type Address, type Hex, bytesToHex, getAddress, pad, parseEther, stringToHex } from "viem";

import { ensureKzgSetup } from "./helpers/index.js";
import {
  deployCallForwardingProxy,
  deployForcedTransactionGatewayFixture,
  deployLinethRollupFixture,
  expectSuccessfulFinalizeViaCallForwarder,
  getAccountsFixture,
  getRoleAddressesFixture,
  sendBlobTransactionViaCallForwarder,
} from "./helpers/index.js";
import {
  LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
  LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
} from "../../../common/constants/index.js";
import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json" with { type: "json" };
import blobAggregatedProof1To155 from "../_testData/compressedDataEip4844/aggregatedProof-1-155.json" with { type: "json" };
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
  LINETH_ROLLUP_INITIALIZE_SIGNATURE,
  FORCED_TRANSACTION_FEE,
  SET_ADDRESS_FILTER_ROLE,
  INITIALIZED_ALREADY_MESSAGE,
} from "../common/constants/index.js";
import { deployUpgradableFromFactory, getUpgrades, reinitializeUpgradeableProxy } from "../common/deployment.js";
import {
  type TestAccount,
  hex,
  calculateRollingHash,
  encodeData,
  generateRandomBytes,
  buildAccessErrorMessage,
  generateBlobParentShnarfData,
  calculateLastFinalizedState,
  expectNoEvent,
  expectEventDirectFromReceiptData,
  computeGenesisShnarf,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
} from "../common/helpers/index.js";

import type { LinethRollupInitializationData, RoleAddress } from "../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

ensureKzgSetup();

describe("Lineth Rollup contract", () => {
  let linethRollup: ContractReturnType<"TestLinethRollup">;
  let verifier: Address;
  let callForwardingProxy: ContractReturnType<"CallForwardingProxy">;
  let yieldManager: Address;
  let linethRollupAbi: Abi;

  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let operator: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let alternateShnarfProviderAddress: TestAccount;
  let roleAddresses: RoleAddress[];
  let addressFilterAddress: Address;
  let addressFilter: ContractReturnType<"AddressFilter">;

  const { compressedData } = firstCompressedDataContent;
  const expectedX = hex(firstCompressedDataContent.expectedX);
  const expectedY = hex(firstCompressedDataContent.expectedY);
  const parentStateRootHash = hex(firstCompressedDataContent.parentStateRootHash);

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateShnarfProviderAddress } =
      await networkHelpers.loadFixture(getAccountsFixture));
    roleAddresses = await networkHelpers.loadFixture(getRoleAddressesFixture);
    ({ abi: linethRollupAbi } = await hre.artifacts.readArtifact("src/rollup/LinethRollup.sol:LinethRollup"));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ addressFilter, verifier, linethRollup, yieldManager } = await networkHelpers.loadFixture(
      deployForcedTransactionGatewayFixture,
    ));
    addressFilterAddress = addressFilter.address;
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: Hex) => {
      return admin.sendTransaction({ to: linethRollup.address, value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the linethRollup contract through the fallback", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract(EMPTY_CALLDATA));
    });

    it("Should fail to send eth to the linethRollup contract through the receive function", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract("0x1234"));
    });
  });

  describe("Initialisation", () => {
    // Helper to create default initialization data (typed to match contract expectations)
    const createDefaultInitData = (): LinethRollupInitializationData => ({
      initialStateRootHash: parentStateRootHash,
      initialL2BlockNumber: BigInt(INITIAL_MIGRATION_BLOCK),
      genesisTimestamp: BigInt(GENESIS_L2_TIMESTAMP),
      defaultVerifier: verifier,
      rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
      rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
      roleAddresses: [...roleAddresses.slice(1)],
      pauseTypeRoles: LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
      unpauseTypeRoles: LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
      defaultAdmin: securityCouncil.address,
      shnarfProvider: ADDRESS_ZERO,
      addressFilter: addressFilterAddress,
    });

    type InitData = ReturnType<typeof createDefaultInitData>;

    // Helper to deploy with custom init data and args
    const deployLinethRollupWithConfig = (
      initData: InitData,
      fallbackOperator: Address = FALLBACK_OPERATOR_ADDRESS,
      contractName: string = "TestLinethRollup",
    ) =>
      deployUpgradableFromFactory(contractName, [initData, fallbackOperator, yieldManager], {
        initializer: LINETH_ROLLUP_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      description: string;
      getInitData: () => InitData;
      fallbackOperator?: Address;
      contractName?: string;
    }> = [
      {
        description: "verifier address is zero address",
        getInitData: () => ({ ...createDefaultInitData(), defaultVerifier: ADDRESS_ZERO, roleAddresses }),
        contractName: "src/rollup/LinethRollup.sol:LinethRollup",
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
        const deployCall = deployLinethRollupWithConfig(getInitData(), fallbackOperator, contractName);
        await expectRevertWithCustomError(linethRollup, deployCall, "ZeroAddressNotAllowed");
      });
    });

    it("Should store verifier address in storage", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      expect(await linethRollup.read.verifiers([0n])).to.be.equal(verifier);
    });

    it("Should assign the OPERATOR_ROLE to operator addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, operator.address])).to.be.true;
    });

    it("Should assign the VERIFIER_SETTER_ROLE to securityCouncil addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      expect(await linethRollup.read.hasRole([VERIFIER_SETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Should assign the VERIFIER_UNSETTER_ROLE to securityCouncil addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      expect(await linethRollup.read.hasRole([VERIFIER_UNSETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Should store the startingRootHash in storage for the first block number", async () => {
      // This test needs explicit BigInt types for event verification
      const { viem } = await network.getOrCreate();
      const initializationData: LinethRollupInitializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: BigInt(INITIAL_MIGRATION_BLOCK),
        genesisTimestamp: BigInt(GENESIS_L2_TIMESTAMP),
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: LINETH_ROLLUP_V8_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: LINETH_ROLLUP_V8_UNPAUSE_TYPES_ROLES,
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

      const linethRollup = await deployUpgradableFromFactory(
        "src/rollup/LinethRollup.sol:LinethRollup",
        [initializationData, FALLBACK_OPERATOR_ADDRESS, yieldManager],
        {
          initializer: LINETH_ROLLUP_INITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor", "incorrect-initializer-order"],
        },
      );

      // The OZ viem plugin does not expose the proxy deployment transaction: with automine it is the only
      // transaction of the latest block.
      const publicClient = await viem.getPublicClient();
      const { transactions } = await publicClient.getBlock();
      const receipt = await publicClient.getTransactionReceipt({ hash: transactions[transactions.length - 1] });
      expect(receipt.contractAddress?.toLowerCase()).to.equal(linethRollup.address.toLowerCase());

      await expectEventDirectFromReceiptData(
        linethRollup,
        receipt,
        "LineaRollupBaseInitialized",
        [
          pad(stringToHex("8.0"), { dir: "right", size: 8 }),
          expectedAsTuple,
          computeGenesisShnarf(parentStateRootHash),
        ],
        38,
      );

      expect(await linethRollup.read.stateRootHashes([BigInt(INITIAL_MIGRATION_BLOCK)])).to.be.equal(
        parentStateRootHash,
      );
    });

    it("Should assign the VERIFIER_SETTER_ROLE to both SecurityCouncil and Operator", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
      };

      const linethRollup = await deployLinethRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LinethRollup.sol:LinethRollup",
      );

      expect(await linethRollup.read.hasRole([VERIFIER_SETTER_ROLE, securityCouncil.address])).to.be.true;
      expect(await linethRollup.read.hasRole([VERIFIER_SETTER_ROLE, operator.address])).to.be.true;
    });

    it("Should assign the passed in shnarfProvider address", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        shnarfProvider: alternateShnarfProviderAddress.address,
      };

      const linethRollup = await deployLinethRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LinethRollup.sol:LinethRollup",
      );

      expect(await linethRollup.read.shnarfProvider()).to.equal(alternateShnarfProviderAddress.address);
    });

    it("Should assign the passed in addressFilter address", async () => {
      const initData = {
        ...createDefaultInitData(),
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
      };

      const linethRollup = await deployLinethRollupWithConfig(
        initData,
        FALLBACK_OPERATOR_ADDRESS,
        "src/rollup/LinethRollup.sol:LinethRollup",
      );

      expect(await linethRollup.read.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Should have the linethRollup address as the shnarfProvider", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      const linethRollupAddress = linethRollup.address;

      expect(await linethRollup.read.shnarfProvider()).to.equal(linethRollupAddress);
    });

    it("Should have the correct contract version", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      expect(await linethRollup.read.CONTRACT_VERSION()).to.equal("8.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      const initData = { ...createDefaultInitData(), roleAddresses };
      const initializeCall = linethRollup.write.initialize([initData, FALLBACK_OPERATOR_ADDRESS, yieldManager]);

      await expectRevertWithCustomError(linethRollup, initializeCall, "InitializedVersionWrong", [0n, 9n]);
    });
  });

  describe("Upgrading / reinitialisation", () => {
    beforeEach(async () => {
      // Simulate a pre-upgrade state by lowering the initialized version to allow reinitializer(9) to run
      await linethRollup.write.setSlotValue([0n, 8n]);
    });

    it("Should revert if the forced transaction fee is zero", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        0n,
        addressFilterAddress,
      ]);

      await expectRevertWithCustomError(linethRollup, upgradeCall, "ZeroValueNotAllowed");
    });

    it("Should revert if the address filter address is zero address", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        ADDRESS_ZERO,
      ]);

      await expectRevertWithCustomError(linethRollup, upgradeCall, "ZeroAddressNotAllowed");
    });

    it("Should set the next forced transaction number to 1", async () => {
      await reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await linethRollup.read.nextForcedTransactionNumber()).to.equal(1n);
    });

    it("Should emit the AddressFilterChanged event when the address filter is set", async () => {
      await expectEvent(
        linethRollup,
        reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
          FORCED_TRANSACTION_FEE,
          addressFilterAddress,
        ]),
        "AddressFilterChanged",
        [ADDRESS_ZERO, addressFilterAddress],
      );
    });

    it("Should emit the ForcedTransactionFeeSet event when the address filter is set", async () => {
      await expectEvent(
        linethRollup,
        reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
          FORCED_TRANSACTION_FEE,
          addressFilterAddress,
        ]),
        "ForcedTransactionFeeSet",
        [FORCED_TRANSACTION_FEE],
      );
    });

    it("Should set the address filter", async () => {
      await reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await linethRollup.read.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Next contract version number should be 8.0", async () => {
      await reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      expect(await linethRollup.read.CONTRACT_VERSION()).to.equal("8.0");
    });

    it("Next contract version number should be 8.0", async () => {
      const upgradeCall = reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      const previousVersion = pad(stringToHex("7.1"), { dir: "right", size: 8 });
      const newVersion = pad(stringToHex("8.0"), { dir: "right", size: 8 });

      await expectEvent(linethRollup, upgradeCall, "LineaRollupVersionChanged", [previousVersion, newVersion]);
    });

    it("Fails to reinitialize twice", async () => {
      await reinitializeUpgradeableProxy(linethRollup, linethRollupAbi, "reinitializeLineaRollupV9", [
        FORCED_TRANSACTION_FEE,
        addressFilterAddress,
      ]);

      const secondUpgradeCall = reinitializeUpgradeableProxy(
        linethRollup,
        linethRollupAbi,
        "reinitializeLineaRollupV9",
        [FORCED_TRANSACTION_FEE, addressFilterAddress],
      );

      await expectRevertWithReason(secondUpgradeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Change verifier address", () => {
    it("Should revert if the caller has not the VERIFIER_SETTER_ROLE", async () => {
      const setVerifierCall = linethRollup.write.setVerifierAddress([verifier, 2n], {
        account: nonAuthorizedAccount.account,
      });

      await expectRevertWithReason(
        setVerifierCall,
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_SETTER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      await linethRollup.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      const setVerifierCall = linethRollup.write.setVerifierAddress([ADDRESS_ZERO, 2n], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(linethRollup, setVerifierCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new verifier address", async () => {
      await linethRollup.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      await linethRollup.write.setVerifierAddress([verifier, 2n], { account: securityCouncil.account });
      expect(await linethRollup.read.verifiers([2n])).to.be.equal(verifier);
    });

    it("Should remove verifier address in storage ", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      await linethRollup.write.unsetVerifierAddress([0n], { account: securityCouncil.account });

      expect(await linethRollup.read.verifiers([0n])).to.be.equal(ADDRESS_ZERO);
    });

    it("Should revert when removing verifier address if the caller has not the VERIFIER_UNSETTER_ROLE ", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));

      await expectRevertWithReason(
        linethRollup.write.unsetVerifierAddress([0n], { account: nonAuthorizedAccount.account }),
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_UNSETTER_ROLE),
      );
    });

    it("Should emit the correct event", async () => {
      await linethRollup.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      const oldVerifierAddress = await linethRollup.read.verifiers([2n]);

      const setVerifierCall = linethRollup.write.setVerifierAddress([verifier, 2n], {
        account: securityCouncil.account,
      });
      let expectedArgs: [Address, bigint, Address, Address] = [
        verifier,
        2n,
        securityCouncil.address,
        oldVerifierAddress,
      ];

      await expectEvent(linethRollup, setVerifierCall, "VerifierAddressChanged", expectedArgs);

      await linethRollup.write.unsetVerifierAddress([2n], { account: securityCouncil.account });

      const unsetVerifierCall = linethRollup.write.unsetVerifierAddress([2n], { account: securityCouncil.account });
      expectedArgs = [ADDRESS_ZERO, 2n, securityCouncil.address, oldVerifierAddress];

      await expectEvent(linethRollup, unsetVerifierCall, "VerifierAddressChanged", expectedArgs);
    });
  });

  describe("Change address filter", () => {
    beforeEach(async () => {
      await linethRollup.write.grantRole([SET_ADDRESS_FILTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });
    });

    it("Should revert if the address filter is the zero address", async () => {
      const setAddressFilterCall = linethRollup.write.setAddressFilter([ADDRESS_ZERO], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(linethRollup, setAddressFilterCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if the caller has not the SET_ADDRESS_FILTER_ROLE", async () => {
      const setAddressFilterCall = linethRollup.write.setAddressFilter([addressFilterAddress], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithReason(
        setAddressFilterCall,
        buildAccessErrorMessage(nonAuthorizedAccount, SET_ADDRESS_FILTER_ROLE),
      );
    });

    it("Should set the new address filter", async () => {
      const newAddressFilter = getAddress(generateRandomBytes(20));
      await linethRollup.write.setAddressFilter([newAddressFilter], { account: securityCouncil.account });
      expect(await linethRollup.read.addressFilter()).to.be.equal(newAddressFilter);
    });

    it("Should emit the AddressFilterChanged event when the address filter is set", async () => {
      const newAddressFilter = getAddress(generateRandomBytes(20));
      await expectEvent(
        linethRollup,
        linethRollup.write.setAddressFilter([newAddressFilter], { account: securityCouncil.account }),
        "AddressFilterChanged",
        [addressFilterAddress, newAddressFilter],
      );
    });

    it("Should not emit the event if the address filter is the same", async () => {
      const newAddressFilter = getAddress(generateRandomBytes(20));

      await expectEvent(
        linethRollup,
        linethRollup.write.setAddressFilter([newAddressFilter], { account: securityCouncil.account }),
        "AddressFilterChanged",
        [addressFilterAddress, newAddressFilter],
      );

      await expectNoEvent(
        linethRollup,
        linethRollup.write.setAddressFilter([newAddressFilter], { account: securityCouncil.account }),
        "AddressFilterChanged",
      );
    });
  });

  describe("Validate L2 computed rolling hash", () => {
    it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
      const l1MessageNumber = 0n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "MissingMessageNumberForRollingHash",
        [l1RollingHash],
      );
    });

    it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = HASH_ZERO;

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "MissingRollingHashForMessageNumber",
        [l1MessageNumber],
      );
    });

    it("Should revert if l1RollingHash does not exist on L1", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "L1RollingHashDoesNotExistOnL1",
        [l1MessageNumber, l1RollingHash],
      );
    });

    it("Should succeed if l1 message number == 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 0n;
      const l1RollingHash = HASH_ZERO;
      await linethRollup.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]);
    });

    it("Should succeed if l1 message number != 0, l1 rolling hash is not empty and exists on L1", async () => {
      const l1MessageNumber = 1n;
      const messageHash = generateRandomBytes(32);

      await linethRollup.write.addRollingHash([l1MessageNumber, messageHash]);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await linethRollup.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]);
    });
  });

  describe("Calculate Y value for Compressed Data", () => {
    it("Should successfully calculate y", async () => {
      const compressedDataBytes = bytesToHex(Buffer.from(compressedData, "base64"));

      // viem reads take no gas limit (ethers passed `gasLimit: MAX_GAS_LIMIT` to the call).
      expect(await linethRollup.read.calculateY([compressedDataBytes, expectedX])).to.equal(expectedY);
    });

    it("Should revert if first byte is no zero", async () => {
      const compressedDataBytes = encodeData(
        ["bytes32", "bytes32", "bytes32"],
        [generateRandomBytes(32), HASH_WITHOUT_ZERO_FIRST_BYTE, generateRandomBytes(32)],
      );

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.read.calculateY([compressedDataBytes, expectedX]),
        "FirstByteIsNotZero",
      );
    });

    it("Should revert if bytes length is not a multiple of 32", async () => {
      const compressedDataBytes = generateRandomBytes(56);

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.read.calculateY([compressedDataBytes, expectedX]),
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
      const { viem } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const initialBlock = await publicClient.getBlock({ blockTag: "latest" });

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([0n, HASH_ZERO, 0n, HASH_ZERO, initialBlock.timestamp]),
        "LastFinalizationTimeNotLapsed",
      );
    });

    it("Should revert if the time has passed and the last finalized timestamp does not match", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(0n, HASH_ZERO, 0n, HASH_ZERO, 123456789n);

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([0n, HASH_ZERO, 0n, HASH_ZERO, 123456789n]),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 message number does not match", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);
      const actualSentState = calculateLastFinalizedState(
        1n,
        HASH_ZERO,
        0n,
        HASH_ZERO,
        DEFAULT_LAST_FINALIZED_TIMESTAMP,
      );

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          1n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should revert if the time has passed and the last finalized L1 rolling hash does not match", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);
      const random32Bytes = generateRandomBytes(32);
      const actualSentState = calculateLastFinalizedState(
        0n,
        random32Bytes,
        0n,
        HASH_ZERO,
        DEFAULT_LAST_FINALIZED_TIMESTAMP,
      );

      await expectRevertWithCustomError(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          0n,
          random32Bytes,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "FinalizationStateIncorrect",
        [expectedLastFinalizedState, actualSentState],
      );
    });

    it("Should set the liveness recovery operator role after six months have passed", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          0n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS])).to.be.true;
    });

    it("Should not expect a second event with liveness operator setting", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          0n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS])).to.be.true;

      await expectNoEvent(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          0n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "LivenessRecoveryOperatorRoleGranted",
      );
    });

    it("Should revert if trying to renounce role as liveness recovery operator", async () => {
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        linethRollup,
        linethRollup.write.setLivenessRecoveryOperator([
          0n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
        "LivenessRecoveryOperatorRoleGranted",
        [admin.address, FALLBACK_OPERATOR_ADDRESS],
      );

      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS])).to.be.true;

      const renounceCall = linethRollup.write.renounceRole([OPERATOR_ROLE, FALLBACK_OPERATOR_ADDRESS]);

      await expectRevertWithCustomError(linethRollup, renounceCall, "OnlyNonLivenessRecoveryOperator");
    });

    it("Should renounce role if not liveness recovery operator", async () => {
      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, operator.address])).to.be.true;

      const renounceCall = linethRollup.write.renounceRole([OPERATOR_ROLE, operator.address], {
        account: operator.account,
      });
      await expectEvent(linethRollup, renounceCall, "RoleRevoked", [OPERATOR_ROLE, operator.address, operator.address]);
    });

    it("Should fail to accept ETH on the CallForwardingProxy receive function", async () => {
      callForwardingProxy = await deployCallForwardingProxy(linethRollup.address);
      const forwardingProxyAddress = callForwardingProxy.address;

      const tx = {
        to: forwardingProxyAddress,
        value: parseEther("0.1"),
      };

      await expectRevertWithReason(admin.sendTransaction(tx), "ETH not accepted");
    });

    it("Should be able to submit blobs and finalize via callforwarding proxy", async () => {
      callForwardingProxy = await deployCallForwardingProxy(linethRollup.address);
      const forwardingProxyAddress = callForwardingProxy.address;

      expect(await linethRollup.read.currentL2BlockNumber()).to.equal(0n);

      // Deploy new LinethRollup implementation
      const oz = await getUpgrades();
      const upgradedContract = await oz.upgradeProxy(
        linethRollup.address,
        "src/_testing/unit/rollup/TestLinethRollup.sol:TestLinethRollup",
        {
          unsafeAllowRenames: true,
          unsafeAllow: ["incorrect-initializer-order"],
        },
      );

      await upgradedContract.write.setLivenessRecoveryOperatorAddress([forwardingProxyAddress]);

      // Grants deployed callforwarding proxy as operator
      const { networkHelpers } = await network.getOrCreate();
      await networkHelpers.time.increase(SIX_MONTHS_IN_SECONDS);

      await expectEvent(
        upgradedContract,
        upgradedContract.write.setLivenessRecoveryOperator([
          0n,
          HASH_ZERO,
          0n,
          HASH_ZERO,
          DEFAULT_LAST_FINALIZED_TIMESTAMP,
        ]),
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
