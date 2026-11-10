import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import { deployValidiumFixture, getAccountsFixture, getValidiumRoleAddressesFixture } from "./helpers/index.js";
import {
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "../../../common/constants/index.js";
import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json" with { type: "json" };
import secondCompressedDataContent from "../_testData/compressedData/blocks-47-81.json" with { type: "json" };
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
} from "../common/constants/index.js";
import { deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  hex,
  calculateRollingHash,
  generateRandomBytes,
  generateCallDataSubmission,
  buildAccessErrorMessage,
  expectRevertWhenPaused,
  generateKeccak256,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectEvent,
} from "../common/helpers/index.js";

import type { RoleAddress } from "../common/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";
import type { Address, Hex } from "viem";

describe("Validium contract", () => {
  let validium: ContractReturnType<"TestValidium">;
  let addressFilter: ContractReturnType<"AddressFilter">;
  let verifier: Address;

  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let operator: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let alternateShnarfProviderAddress: TestAccount;
  let roleAddresses: RoleAddress[];
  let addressFilterAddress: Address;

  const prevShnarf = hex(firstCompressedDataContent.prevShnarf);
  const expectedShnarf = hex(firstCompressedDataContent.expectedShnarf);
  const parentStateRootHash = hex(firstCompressedDataContent.parentStateRootHash);
  const secondExpectedShnarf = hex(secondCompressedDataContent.expectedShnarf);

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateShnarfProviderAddress } =
      await networkHelpers.loadFixture(getAccountsFixture));
    roleAddresses = await networkHelpers.loadFixture(getValidiumRoleAddressesFixture);
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ verifier, validium, addressFilter } = await networkHelpers.loadFixture(deployValidiumFixture));
    addressFilterAddress = addressFilter.address;
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: Hex) => {
      return admin.sendTransaction({ to: validium.address, value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the validium contract through the fallback", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract(EMPTY_CALLDATA));
    });

    it("Should fail to send eth to the validium contract through the receive function", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract("0x1234"));
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
      pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
      unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
      defaultAdmin: securityCouncil.address,
      shnarfProvider: ADDRESS_ZERO,
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
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      expect(await validium.read.verifiers([0n])).to.be.equal(verifier);
    });

    it("Should assign the OPERATOR_ROLE to operator addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      expect(await validium.read.hasRole([OPERATOR_ROLE, operator.address])).to.be.true;
    });

    it("Should assign the VERIFIER_SETTER_ROLE to securityCouncil addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      expect(await validium.read.hasRole([VERIFIER_SETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Should assign the VERIFIER_UNSETTER_ROLE to securityCouncil addresses", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      expect(await validium.read.hasRole([VERIFIER_UNSETTER_ROLE, securityCouncil.address])).to.be.true;
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
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.read.stateRootHashes([BigInt(INITIAL_MIGRATION_BLOCK)])).to.be.equal(parentStateRootHash);
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
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.read.hasRole([VERIFIER_SETTER_ROLE, securityCouncil.address])).to.be.true;
      expect(await validium.read.hasRole([VERIFIER_SETTER_ROLE, operator.address])).to.be.true;
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
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: alternateShnarfProviderAddress.address,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.read.shnarfProvider()).to.equal(alternateShnarfProviderAddress.address);
    });

    it("Should assign the passed in addressFilter address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [...roleAddresses, { addressWithRole: operator.address, role: VERIFIER_SETTER_ROLE }],
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: alternateShnarfProviderAddress.address,
        addressFilter: addressFilterAddress,
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.read.addressFilter()).to.equal(addressFilterAddress);
    });

    it("Should have the validium address as the shnarfProvider", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      const validiumAddress = validium.address;

      expect(await validium.read.shnarfProvider()).to.equal(validiumAddress);
    });

    it("Should have the correct contract version", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      expect(await validium.read.CONTRACT_VERSION()).to.equal("2.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      const initializeCall = validium.write.initialize([
        {
          initialStateRootHash: parentStateRootHash,
          initialL2BlockNumber: BigInt(INITIAL_MIGRATION_BLOCK),
          genesisTimestamp: BigInt(GENESIS_L2_TIMESTAMP),
          defaultVerifier: verifier,
          rateLimitPeriodInSeconds: BigInt(ONE_DAY_IN_SECONDS),
          rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
          roleAddresses,
          pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
          unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
          defaultAdmin: securityCouncil.address,
          shnarfProvider: ADDRESS_ZERO,
          addressFilter: addressFilterAddress,
        },
      ]);

      await expectRevertWithReason(initializeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Change verifier address", () => {
    it("Should revert if the caller has not the VERIFIER_SETTER_ROLE", async () => {
      const setVerifierCall = validium.write.setVerifierAddress([verifier, 2n], {
        account: nonAuthorizedAccount.account,
      });

      await expectRevertWithReason(
        setVerifierCall,
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_SETTER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      await validium.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      const setVerifierCall = validium.write.setVerifierAddress([ADDRESS_ZERO, 2n], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(validium, setVerifierCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new verifier address", async () => {
      await validium.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      await validium.write.setVerifierAddress([verifier, 2n], { account: securityCouncil.account });
      expect(await validium.read.verifiers([2n])).to.be.equal(verifier);
    });

    it("Should remove verifier address in storage ", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));
      await validium.write.unsetVerifierAddress([0n], { account: securityCouncil.account });

      expect(await validium.read.verifiers([0n])).to.be.equal(ADDRESS_ZERO);
    });

    it("Should revert when removing verifier address if the caller has not the VERIFIER_UNSETTER_ROLE ", async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ verifier, validium } = await networkHelpers.loadFixture(deployValidiumFixture));

      await expectRevertWithReason(
        validium.write.unsetVerifierAddress([0n], { account: nonAuthorizedAccount.account }),
        buildAccessErrorMessage(nonAuthorizedAccount, VERIFIER_UNSETTER_ROLE),
      );
    });

    it("Should emit the correct event", async () => {
      await validium.write.grantRole([VERIFIER_SETTER_ROLE, securityCouncil.address], {
        account: securityCouncil.account,
      });

      const oldVerifierAddress = await validium.read.verifiers([2n]);

      const setVerifierCall = validium.write.setVerifierAddress([verifier, 2n], { account: securityCouncil.account });
      let expectedArgs: [Address, bigint, Address, Address] = [
        verifier,
        2n,
        securityCouncil.address,
        oldVerifierAddress,
      ];

      await expectEvent(validium, setVerifierCall, "VerifierAddressChanged", expectedArgs);

      await validium.write.unsetVerifierAddress([2n], { account: securityCouncil.account });

      const unsetVerifierCall = validium.write.unsetVerifierAddress([2n], { account: securityCouncil.account });
      expectedArgs = [ADDRESS_ZERO, 2n, securityCouncil.address, oldVerifierAddress];

      await expectEvent(validium, unsetVerifierCall, "VerifierAddressChanged", expectedArgs);
    });
  });

  describe("Data submission tests", () => {
    beforeEach(async () => {
      await validium.write.setLastFinalizedBlock([0n]);
    });

    const [DATA_ONE] = generateCallDataSubmission(0, 1);

    it("Should fail when the parent shnarf does not exist", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);
      const nonExistingParentShnarf = generateRandomBytes(32);

      const wrongExpectedShnarf = generateKeccak256(
        ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
        [HASH_ZERO, HASH_ZERO, submissionData.finalStateRootHash, HASH_ZERO, HASH_ZERO],
      );

      const asyncCall = validium.write.acceptShnarfData(
        [nonExistingParentShnarf, wrongExpectedShnarf, submissionData.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectRevertWithCustomError(validium, asyncCall, "ParentShnarfNotSubmitted", [nonExistingParentShnarf]);
    });

    it("Should succesfully submit 1 compressed data chunk setting values", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      await validium.write.acceptShnarfData([prevShnarf, expectedShnarf, submissionData.finalStateRootHash], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      const blobShnarfExists = await validium.read.blobShnarfExists([expectedShnarf]);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await validium.write.acceptShnarfData([prevShnarf, expectedShnarf, firstSubmissionData.finalStateRootHash], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      await validium.write.acceptShnarfData(
        [expectedShnarf, secondExpectedShnarf, secondSubmissionData.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      let blobShnarfExists = await validium.read.blobShnarfExists([expectedShnarf]);
      expect(blobShnarfExists).to.equal(1n);
      blobShnarfExists = await validium.read.blobShnarfExists([secondExpectedShnarf]);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should emit an event while submitting 1 compressed data chunk", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      const submitDataCall = validium.write.acceptShnarfData(
        [prevShnarf, secondExpectedShnarf, submissionData.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectEvent(validium, submitDataCall, "DataSubmittedV3", [
        prevShnarf,
        secondExpectedShnarf,
        submissionData.finalStateRootHash,
      ]);
    });

    it("Should fail if the final state root hash is HASH_ZERO", async () => {
      const submitDataCall = validium.write.acceptShnarfData([prevShnarf, expectedShnarf, HASH_ZERO], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(validium, submitDataCall, "FinalStateRootHashIsZeroHash");
    });

    it("Should fail to submit where submitted shnarf is HASH_ZERO", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await validium.write.acceptShnarfData([prevShnarf, expectedShnarf, firstSubmissionData.finalStateRootHash], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      const submitDataCall = validium.write.acceptShnarfData(
        [expectedShnarf, HASH_ZERO, secondSubmissionData.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfSubmissionIsZeroHash");
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const submitDataCall = validium.write.acceptShnarfData(
        [prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash],
        { account: nonAuthorizedAccount.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    // Parameterized pause type tests for Validium data submission
    const validiumSubmissionPauseTypes = [
      { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
      { pauseType: STATE_DATA_SUBMISSION_PAUSE_TYPE, name: "STATE_DATA_SUBMISSION_PAUSE_TYPE" },
    ];

    validiumSubmissionPauseTypes.forEach(({ pauseType, name }) => {
      it(`Should revert if ${name} is enabled`, async () => {
        await validium.write.pauseByType([pauseType], { account: securityCouncil.account });

        const submitDataCall = validium.write.acceptShnarfData(
          [prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash],
          { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
        );

        await expectRevertWhenPaused(validium, submitDataCall, pauseType);
      });
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
      await validium.write.acceptShnarfData([prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      const submitDataCall = validium.write.acceptShnarfData(
        [prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same data, differing block numbers", async () => {
      await validium.write.acceptShnarfData([prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash], {
        account: operator.account,
        gas: BigInt(MAX_GAS_LIMIT),
      });

      const [dataOneCopy] = generateCallDataSubmission(0, 1);

      const submitDataCall = validium.write.acceptShnarfData(
        [prevShnarf, expectedShnarf, dataOneCopy.finalStateRootHash],
        { account: operator.account, gas: BigInt(MAX_GAS_LIMIT) },
      );

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });
  });

  describe("Validate L2 computed rolling hash", () => {
    it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
      const l1MessageNumber = 0n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "MissingMessageNumberForRollingHash",
        [l1RollingHash],
      );
    });

    it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = HASH_ZERO;

      await expectRevertWithCustomError(
        validium,
        validium.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "MissingRollingHashForMessageNumber",
        [l1MessageNumber],
      );
    });

    it("Should revert if l1RollingHash does not exist on L1", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]),
        "L1RollingHashDoesNotExistOnL1",
        [l1MessageNumber, l1RollingHash],
      );
    });

    it("Should succeed if l1 message number == 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 0n;
      const l1RollingHash = HASH_ZERO;
      await validium.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]);
    });

    it("Should succeed if l1 message number != 0, l1 rolling hash is not empty and exists on L1", async () => {
      const l1MessageNumber = 1n;
      const messageHash = generateRandomBytes(32);

      await validium.write.addRollingHash([l1MessageNumber, messageHash]);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await validium.read.validateL2ComputedRollingHash([l1MessageNumber, l1RollingHash]);
    });
  });
});
