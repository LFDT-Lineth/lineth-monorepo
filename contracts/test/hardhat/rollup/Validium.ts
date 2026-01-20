import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";

import firstCompressedDataContent from "../_testData/compressedData/blocks-1-46.json";
import secondCompressedDataContent from "../_testData/compressedData/blocks-47-81.json";

import {
  VALIDIUM_PAUSE_TYPES_ROLES,
  VALIDIUM_UNPAUSE_TYPES_ROLES,
  STATE_DATA_SUBMISSION_PAUSE_TYPE,
} from "contracts/common/constants";
import { TestValidium } from "contracts/typechain-types";
import { deployValidiumFixture, getAccountsFixture, getValidiumRoleAddressesFixture } from "./helpers";
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
import { deployUpgradableFromFactory } from "../common/deployment";
import {
  calculateRollingHash,
  generateRandomBytes,
  generateCallDataSubmission,
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateKeccak256,
} from "../common/helpers";

describe("Validium contract", () => {
  let validium: TestValidium;
  let verifier: string;

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let alternateShnarfProviderAddress: SignerWithAddress;
  let roleAddresses: { addressWithRole: string; role: string }[];

  const { prevShnarf, expectedShnarf, parentStateRootHash } = firstCompressedDataContent;
  const { expectedShnarf: secondExpectedShnarf } = secondCompressedDataContent;

  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount, alternateShnarfProviderAddress } =
      await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getValidiumRoleAddressesFixture);
  });

  beforeEach(async () => {
    ({ verifier, validium } = await loadFixture(deployValidiumFixture));
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
    it("Should revert if verifier address is zero address", async () => {
      const initializationData = {
        initialStateRootHash: parentStateRootHash,
        initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
        genesisTimestamp: GENESIS_L2_TIMESTAMP,
        defaultVerifier: ADDRESS_ZERO,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses,
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: securityCouncil.address,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      await expectRevertWithCustomError(validium, deployCall, "ZeroAddressNotAllowed");
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
        pauseTypeRoles: VALIDIUM_PAUSE_TYPES_ROLES,
        unpauseTypeRoles: VALIDIUM_UNPAUSE_TYPES_ROLES,
        defaultAdmin: ADDRESS_ZERO,
        shnarfProvider: ADDRESS_ZERO,
      };

      const deployCall = deployUpgradableFromFactory("TestValidium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      await expectRevertWithCustomError(validium, deployCall, "ZeroAddressNotAllowed");
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
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.stateRootHashes(INITIAL_MIGRATION_BLOCK)).to.be.equal(parentStateRootHash);
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
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.hasRole(VERIFIER_SETTER_ROLE, securityCouncil.address)).to.be.true;
      expect(await validium.hasRole(VERIFIER_SETTER_ROLE, operator.address)).to.be.true;
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
      };

      const validium = await deployUpgradableFromFactory("src/rollup/Validium.sol:Validium", [initializationData], {
        initializer: VALIDIUM_INITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      });

      expect(await validium.shnarfProvider()).to.equal(alternateShnarfProviderAddress.address);
    });

    it("Should have the validium address as the shnarfProvider", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      const validiumAddress = await validium.getAddress();

      expect(await validium.shnarfProvider()).to.equal(validiumAddress);
    });

    it("Should have the correct contract version", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      expect(await validium.CONTRACT_VERSION()).to.equal("1.0");
    });

    it("Should revert if the initialize function is called a second time", async () => {
      ({ verifier, validium } = await loadFixture(deployValidiumFixture));
      const initializeCall = validium.initialize({
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
      });

      await expectRevertWithReason(initializeCall, INITIALIZED_ALREADY_MESSAGE);
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

      await expect(validium.connect(nonAuthorizedAccount).unsetVerifierAddress(0)).to.be.revertedWith(
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

    const [DATA_ONE] = generateCallDataSubmission(0, 1);

    it("Should fail when the parent shnarf does not exist", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);
      const nonExistingParentShnarf = generateRandomBytes(32);

      const wrongExpectedShnarf = generateKeccak256(
        ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
        [HASH_ZERO, HASH_ZERO, submissionData.finalStateRootHash, HASH_ZERO, HASH_ZERO],
      );

      const asyncCall = validium
        .connect(operator)
        .acceptShnarfData(nonExistingParentShnarf, wrongExpectedShnarf, submissionData.finalStateRootHash, {
          gasLimit: MAX_GAS_LIMIT,
        });

      await expectRevertWithCustomError(validium, asyncCall, "ParentShnarfNotSubmitted", [nonExistingParentShnarf]);
    });

    it("Should succesfully submit 1 compressed data chunk setting values", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      await expect(
        validium
          .connect(operator)
          .acceptShnarfData(prevShnarf, expectedShnarf, submissionData.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT }),
      ).to.not.be.reverted;

      const blobShnarfExists = await validium.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should successfully submit 2 compressed data chunks in two transactions", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await expect(
        validium
          .connect(operator)
          .acceptShnarfData(prevShnarf, expectedShnarf, firstSubmissionData.finalStateRootHash, {
            gasLimit: MAX_GAS_LIMIT,
          }),
      ).to.not.be.reverted;

      await expect(
        validium
          .connect(operator)
          .acceptShnarfData(expectedShnarf, secondExpectedShnarf, secondSubmissionData.finalStateRootHash, {
            gasLimit: MAX_GAS_LIMIT,
          }),
      ).to.not.be.reverted;

      let blobShnarfExists = await validium.blobShnarfExists(expectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
      blobShnarfExists = await validium.blobShnarfExists(secondExpectedShnarf);
      expect(blobShnarfExists).to.equal(1n);
    });

    it("Should emit an event while submitting 1 compressed data chunk", async () => {
      const [submissionData] = generateCallDataSubmission(0, 1);

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, secondExpectedShnarf, submissionData.finalStateRootHash, {
          gasLimit: MAX_GAS_LIMIT,
        });
      const eventArgs = [prevShnarf, secondExpectedShnarf, submissionData.finalStateRootHash];

      await expectEvent(validium, submitDataCall, "DataSubmittedV3", eventArgs);
    });

    it("Should fail if the final state root hash is HASH_ZERO", async () => {
      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, HASH_ZERO, { gasLimit: MAX_GAS_LIMIT });

      // TODO: Make the failure shnarf dynamic and computed
      await expectRevertWithCustomError(validium, submitDataCall, "FinalStateRootHashIsZeroHash", []);
    });

    it("Should fail to submit where submitted shnarf is HASH_ZERO", async () => {
      const [firstSubmissionData, secondSubmissionData] = generateCallDataSubmission(0, 2);

      await expect(
        validium
          .connect(operator)
          .acceptShnarfData(prevShnarf, expectedShnarf, firstSubmissionData.finalStateRootHash, {
            gasLimit: MAX_GAS_LIMIT,
          }),
      ).to.not.be.reverted;

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(expectedShnarf, HASH_ZERO, secondSubmissionData.finalStateRootHash, {
          gasLimit: MAX_GAS_LIMIT,
        });

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfSubmissionIsZeroHash", []);
    });

    it("Should revert if the caller does not have the OPERATOR_ROLE", async () => {
      const submitDataCall = validium
        .connect(nonAuthorizedAccount)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithReason(submitDataCall, buildAccessErrorMessage(nonAuthorizedAccount, OPERATOR_ROLE));
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      await validium.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if STATE_DATA_SUBMISSION_PAUSE_TYPE is enabled", async () => {
      await validium.connect(securityCouncil).pauseByType(STATE_DATA_SUBMISSION_PAUSE_TYPE);

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "IsPaused", [STATE_DATA_SUBMISSION_PAUSE_TYPE]);
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same compressed data twice in 2 separate transactions", async () => {
      await validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });

    it("Should revert with ShnarfAlreadySubmitted when submitting same data, differing block numbers", async () => {
      await validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, DATA_ONE.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      const [dataOneCopy] = generateCallDataSubmission(0, 1);

      const submitDataCall = validium
        .connect(operator)
        .acceptShnarfData(prevShnarf, expectedShnarf, dataOneCopy.finalStateRootHash, { gasLimit: MAX_GAS_LIMIT });

      await expectRevertWithCustomError(validium, submitDataCall, "ShnarfAlreadySubmitted", [expectedShnarf]);
    });
  });

  describe("Validate L2 computed rolling hash", () => {
    it("Should revert if l1 message number == 0 and l1 rolling hash is not empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "MissingMessageNumberForRollingHash",
        [l1RollingHash],
      );
    });

    it("Should revert if l1 message number != 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = HASH_ZERO;

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "MissingRollingHashForMessageNumber",
        [l1MessageNumber],
      );
    });

    it("Should revert if l1RollingHash does not exist on L1", async () => {
      const l1MessageNumber = 1n;
      const l1RollingHash = generateRandomBytes(32);

      await expectRevertWithCustomError(
        validium,
        validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash),
        "L1RollingHashDoesNotExistOnL1",
        [l1MessageNumber, l1RollingHash],
      );
    });

    it("Should succeed if l1 message number == 0 and l1 rolling hash is empty", async () => {
      const l1MessageNumber = 0;
      const l1RollingHash = HASH_ZERO;
      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash)).to.not.be.reverted;
    });

    it("Should succeed if l1 message number != 0, l1 rolling hash is not empty and exists on L1", async () => {
      const l1MessageNumber = 1n;
      const messageHash = generateRandomBytes(32);

      await validium.addRollingHash(l1MessageNumber, messageHash);

      const l1RollingHash = calculateRollingHash(HASH_ZERO, messageHash);

      await expect(validium.validateL2ComputedRollingHash(l1MessageNumber, l1RollingHash)).to.not.be.reverted;
    });
  });
});
