import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, setNonce } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import {
  TestClaimingCaller,
  TestL1MessageService,
  TestL1MessageServiceMerkleProof,
  TestL1RevertContract,
  TestReceivingContract,
} from "../../typechain-types";
import {
  ADDRESS_ZERO,
  DEFAULT_ADMIN_ROLE,
  EMPTY_CALLDATA,
  GENERAL_PAUSE_TYPE,
  INBOX_STATUS_RECEIVED,
  INBOX_STATUS_UNKNOWN,
  INITIALIZED_ERROR_MESSAGE,
  INITIAL_WITHDRAW_LIMIT,
  INVALID_MERKLE_PROOF,
  INVALID_MERKLE_PROOF_REVERT,
  L1_L2_PAUSE_TYPE,
  L2_L1_PAUSE_TYPE,
  LOW_NO_REFUND_MESSAGE_FEE,
  MERKLE_PROOF_FALLBACK,
  MERKLE_PROOF_REENTRY,
  MESSAGE_FEE,
  MESSAGE_VALUE_1ETH,
  ONE_DAY_IN_SECONDS,
  PAUSE_ALL_ROLE,
  UNPAUSE_ALL_ROLE,
  RATE_LIMIT_SETTER_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
  VALID_MERKLE_PROOF,
  PAUSE_L2_L1_ROLE,
  PAUSE_L1_L2_ROLE,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "./common/constants";
import { deployFromFactory, deployUpgradableFromFactory } from "./common/deployment";
import {
  buildAccessErrorMessage,
  calculateRollingHash,
  encodeSendMessage,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateKeccak256Hash,
} from "./common/helpers";

describe("L1MessageService", () => {
  let l1MessageService: TestL1MessageService;
  let l1MessageServiceMerkleProof: TestL1MessageServiceMerkleProof;
  let l1TestRevert: TestL1RevertContract;
  let admin: SignerWithAddress;
  let pauser: SignerWithAddress;
  let limitSetter: SignerWithAddress;
  let notAuthorizedAccount: SignerWithAddress;
  let postmanAddress: SignerWithAddress;
  let l2Sender: SignerWithAddress;

  async function deployTestL1MessageServiceFixture(): Promise<TestL1MessageService> {
    return deployUpgradableFromFactory("TestL1MessageService", [
      ONE_DAY_IN_SECONDS,
      INITIAL_WITHDRAW_LIMIT,
      pauseTypeRoles,
      unpauseTypeRoles,
    ]) as unknown as Promise<TestL1MessageService>;
  }

  async function deployL1MessageServiceMerkleFixture(): Promise<TestL1MessageServiceMerkleProof> {
    return deployUpgradableFromFactory("TestL1MessageServiceMerkleProof", [
      ONE_DAY_IN_SECONDS,
      INITIAL_WITHDRAW_LIMIT,
      pauseTypeRoles,
      unpauseTypeRoles,
    ]) as unknown as Promise<TestL1MessageServiceMerkleProof>;
  }

  async function deployL1TestRevertFixture(): Promise<TestL1RevertContract> {
    return deployUpgradableFromFactory("TestL1RevertContract", []) as unknown as Promise<TestL1RevertContract>;
  }

  before(async () => {
    [admin, pauser, limitSetter, notAuthorizedAccount, postmanAddress, l2Sender] = await ethers.getSigners();
    // TODO adjust the tests to dynamically use whatever nonce is set for the merkle proof
    await setNonce(admin.address, 1);
  });

  beforeEach(async () => {
    l1MessageService = await loadFixture(deployTestL1MessageServiceFixture);
    l1MessageServiceMerkleProof = await loadFixture(deployL1MessageServiceMerkleFixture);
    l1TestRevert = await loadFixture(deployL1TestRevertFixture);

    await l1MessageService.grantRole(PAUSE_L1_L2_ROLE, pauser.address);
    await l1MessageService.grantRole(PAUSE_L2_L1_ROLE, pauser.address);
    await l1MessageService.grantRole(PAUSE_ALL_ROLE, pauser.address);
    await l1MessageService.grantRole(UNPAUSE_ALL_ROLE, pauser.address);
    await l1MessageService.grantRole(RATE_LIMIT_SETTER_ROLE, limitSetter.address);
    await l1MessageService.grantRole(USED_RATE_LIMIT_RESETTER_ROLE, limitSetter.address);

    await l1MessageServiceMerkleProof.grantRole(PAUSE_ALL_ROLE, pauser.address);

    await l1MessageService.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });
    await l1MessageServiceMerkleProof.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });
  });

  describe("Initialisation tests", () => {
    it("Deployer has default admin role", async () => {
      expect(await l1MessageService.hasRole(DEFAULT_ADMIN_ROLE, admin.address)).to.be.true;
    });

    it("limitSetter has RATE_LIMIT_SETTER_ROLE", async () => {
      expect(await l1MessageService.hasRole(RATE_LIMIT_SETTER_ROLE, limitSetter.address)).to.be.true;
    });

    it("limitSetter has USED_RATE_LIMIT_RESETTER_ROLE", async () => {
      expect(await l1MessageService.hasRole(USED_RATE_LIMIT_RESETTER_ROLE, limitSetter.address)).to.be.true;
    });

    it("pauser has PAUSE_ALL_ROLE", async () => {
      expect(await l1MessageService.hasRole(PAUSE_ALL_ROLE, pauser.address)).to.be.true;
    });

    it("pauser has UNPAUSE_ALL_ROLE", async () => {
      expect(await l1MessageService.hasRole(UNPAUSE_ALL_ROLE, pauser.address)).to.be.true;
    });

    it("Should set rate limit and period", async () => {
      expect(await l1MessageService.periodInSeconds()).to.be.equal(ONE_DAY_IN_SECONDS);
      expect(await l1MessageService.limitInWei()).to.be.equal(INITIAL_WITHDRAW_LIMIT);
    });

    it("It should fail when not initializing", async () => {
      await expectRevertWithReason(
        l1MessageService.tryInitialize(ONE_DAY_IN_SECONDS, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles),
        INITIALIZED_ERROR_MESSAGE,
      );
    });

    it("Should initialize nextMessageNumber", async () => {
      expect(await l1MessageService.nextMessageNumber()).to.be.equal(1);
    });

    it("Should fail to deploy missing amount", async () => {
      await expectRevertWithCustomError(
        l1MessageService,
        deployUpgradableFromFactory("TestL1MessageService", [ONE_DAY_IN_SECONDS, 0, pauseTypeRoles, unpauseTypeRoles]),
        "LimitIsZero",
      );
    });

    it("Should fail to deploy missing limit period", async () => {
      await expectRevertWithCustomError(
        l1MessageService,
        deployUpgradableFromFactory("TestL1MessageService", [
          0,
          INITIAL_WITHDRAW_LIMIT,
          pauseTypeRoles,
          unpauseTypeRoles,
        ]),
        "PeriodIsZero",
      );
    });
  });

  describe("Send messages", () => {
    it("Should fail when the fee is higher than the amount sent", async () => {
      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, { value: MESSAGE_FEE - 1n });

      await expectRevertWithCustomError(l1MessageService, sendMessageCall, "ValueSentTooLow");
    });

    it("Should fail when the to address is address 0", async () => {
      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(ADDRESS_ZERO, MESSAGE_FEE, EMPTY_CALLDATA, { value: MESSAGE_FEE });

      await expectRevertWithCustomError(l1MessageService, sendMessageCall, "ZeroAddressNotAllowed");
    });

    it("Should send an ether only message with fees emitting the MessageSent event", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, {
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        });
      const eventArgs = [
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1,
        EMPTY_CALLDATA,
        messageHash,
      ];
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", eventArgs);
    });

    it("Should send max limit ether only message with no fee emitting the MessageSent event", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        0n,
        INITIAL_WITHDRAW_LIMIT,
        1n,
        EMPTY_CALLDATA,
      );
      const messageHash = ethers.keccak256(expectedBytes);

      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, 0, EMPTY_CALLDATA, { value: INITIAL_WITHDRAW_LIMIT });
      const eventArgs = [
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        0,
        INITIAL_WITHDRAW_LIMIT,
        1,
        EMPTY_CALLDATA,
        messageHash,
      ];
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", eventArgs);
    });

    // this is testing to allow even if claim is blocked
    it("Should send a message even when L2 to L1 communication is paused", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      await l1MessageService.connect(pauser).pauseByType(L2_L1_PAUSE_TYPE);

      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, {
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        });
      const eventArgs = [
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1,
        EMPTY_CALLDATA,
        messageHash,
      ];
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", eventArgs);
    });

    it("Should update the rolling hash when sending a message post migration", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);
      const rollingHash = calculateRollingHash(ethers.ZeroHash, messageHash);

      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, {
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        });
      const messageSentEventArgs = [
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1,
        EMPTY_CALLDATA,
        messageHash,
      ];
      const rollingHashUpdatedEventArgs = [1, rollingHash, messageHash];

      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", messageSentEventArgs);
      await expectEvent(l1MessageService, sendMessageCall, "RollingHashUpdated", rollingHashUpdatedEventArgs);

      const rollingHashAtIndex = await l1MessageService.rollingHashes(1);

      expect(rollingHashAtIndex).to.equal(rollingHash);
      expect(rollingHashAtIndex).to.not.equal(ethers.ZeroHash);
    });

    it("Should use the previous existing rolling hash when sending a message post migration", async () => {
      let expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      let messageHash = ethers.keccak256(expectedBytes);
      let rollingHash = calculateRollingHash(ethers.ZeroHash, messageHash);

      await l1MessageService.connect(admin).canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, {
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
      });

      expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2n,
        EMPTY_CALLDATA,
      );

      messageHash = ethers.keccak256(expectedBytes);

      rollingHash = calculateRollingHash(rollingHash, messageHash);

      const sendMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA, {
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        });
      const messageSenteventArgs = [
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2,
        EMPTY_CALLDATA,
        messageHash,
      ];
      const rollingHashUpdatedEventArgs = [2, rollingHash, messageHash];

      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", messageSenteventArgs);
      await expectEvent(l1MessageService, sendMessageCall, "RollingHashUpdated", rollingHashUpdatedEventArgs);

      const rollingHashAtIndex = await l1MessageService.rollingHashes(2);
      expect(rollingHashAtIndex).to.equal(rollingHash);
      expect(rollingHashAtIndex).to.not.equal(ethers.ZeroHash);
    });
  });

  describe("Claiming messages", () => {
    it("Should fail when the message hash does not exist", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      const claimMessageCall = l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1,
      );

      await expectRevertWithCustomError(
        l1MessageService,
        claimMessageCall,
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });

    it("Should execute the claim message and send fees to recipient, left over fee to destination", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      await expect(
        l1MessageService.claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1,
        ),
      ).to.not.be.reverted;
    });

    it("Should claim message and send the fees when L1 to L2 communication is paused", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      // this is for sending only and should not affect claim
      await l1MessageService.connect(pauser).pauseByType(L1_L2_PAUSE_TYPE);

      await expect(
        l1MessageService.claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1,
        ),
      ).to.not.be.reverted;
    });

    it("Should execute the claim message and emit the MessageClaimed event", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      await l1MessageService.addL2L1MessageHash(messageHash);

      const claimMessageCall = l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1,
      );

      await expectEvent(l1MessageService, claimMessageCall, "MessageClaimed", [messageHash]);
    });

    it("Should fail when the message hash has been claimed", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      await l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1,
      );

      const claimMessageCall = l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1,
      );

      await expectRevertWithCustomError(
        l1MessageService,
        claimMessageCall,
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });

    it("Should execute the claim message and send the fees to msg.sender, left over fee to destination", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const expectedSecondBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2n,
        EMPTY_CALLDATA,
      );

      const destinationBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));
      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedSecondBytes));

      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1,
        );

      const adminBalance = await ethers.provider.getBalance(admin.address);

      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          2,
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.greaterThan(
        destinationBalance + MESSAGE_VALUE_1ETH + MESSAGE_VALUE_1ETH,
      );
      expect(await ethers.provider.getBalance(admin.address)).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message and send the fees to msg.sender and NOT refund the destination", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        LOW_NO_REFUND_MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );
      const destinationBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          LOW_NO_REFUND_MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1,
          { gasPrice: 1000000000 },
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.equal(
        destinationBalance + MESSAGE_VALUE_1ETH,
      );

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message and send fees to recipient contract and no refund sent", async () => {
      const factory = await ethers.getContractFactory("TestReceivingContract");
      const testContract = (await factory.deploy()) as TestReceivingContract;

      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        await testContract.getAddress(),
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const adminBalance = await ethers.provider.getBalance(admin.address);
      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          await testContract.getAddress(),
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1,
        );

      expect(await ethers.provider.getBalance(await testContract.getAddress())).to.be.equal(MESSAGE_VALUE_1ETH);
      expect(await ethers.provider.getBalance(admin.address)).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message and send fees to EOA with calldata and no refund sent", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        "0x123456789a",
      );

      const startingBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const adminBalance = await ethers.provider.getBalance(admin.address);
      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          "0x123456789a",
          1,
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await ethers.provider.getBalance(admin.address)).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message and no fees to EOA with calldata and no refund sent", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        "0x12",
      );

      const startingBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const adminBalance = await ethers.provider.getBalance(admin.address);
      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          "0x12",
          1,
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await ethers.provider.getBalance(admin.address)).to.be.lessThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message and no fees to EOA with empty calldata and no refund sent", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const startingBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const adminBalance = await ethers.provider.getBalance(admin.address);
      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1,
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await ethers.provider.getBalance(admin.address)).to.be.lessThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should execute the claim message when there are no fees", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );
      const destinationBalance = await ethers.provider.getBalance(notAuthorizedAccount.address);

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const adminBalance = await ethers.provider.getBalance(admin.address);
      await l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          0,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1,
        );

      expect(await ethers.provider.getBalance(notAuthorizedAccount.address)).to.be.equal(
        destinationBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await ethers.provider.getBalance(admin.address)).to.be.lessThan(adminBalance);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_UNKNOWN,
      );
    });

    it("Should provide the correct origin sender", async () => {
      const claimingCaller = (await deployFromFactory(
        "TestClaimingCaller",
        l2Sender.address,
      )) as unknown as TestClaimingCaller;

      const expectedBytes = await encodeSendMessage(
        l2Sender.address,
        await claimingCaller.getAddress(),
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const storedSenderBeforeSending = await l1MessageService.originalSender();
      expect(storedSenderBeforeSending).to.be.equal(ADDRESS_ZERO);

      await expect(
        l1MessageService
          .connect(admin)
          .claimMessage(
            l2Sender.address,
            await claimingCaller.getAddress(),
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            EMPTY_CALLDATA,
            1,
          ),
      ).to.not.be.reverted;
    });

    it("Should allow sending post claiming a message", async () => {
      const sendCalldata = generateKeccak256Hash("sendNewMessage()").substring(0, 10);

      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        await l1MessageService.getAddress(),
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        sendCalldata,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      await expect(
        l1MessageService
          .connect(admin)
          .claimMessage(
            await l1MessageService.getAddress(),
            await l1MessageService.getAddress(),
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            sendCalldata,
            1,
          ),
      ).to.not.be.reverted;
    });

    it("Should fail on reentry when sending to recipient", async () => {
      const callSignature = generateKeccak256Hash("doReentry()").substring(0, 10);

      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        await l1MessageService.getAddress(),
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        callSignature,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const claimMessageCall = l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          await l1MessageService.getAddress(),
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          callSignature,
          1n,
        );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "ReentrantCall");
    });

    it("Should fail when the destination errors", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        await l1MessageService.getAddress(),
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));
      await expect(
        l1MessageService
          .connect(admin)
          .claimMessage(
            await l1MessageService.getAddress(),
            await l1MessageService.getAddress(),
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            EMPTY_CALLDATA,
            1,
          ),
      )
        .to.be.revertedWithCustomError(l1MessageService, "MessageSendingFailed")
        .withArgs(await l1MessageService.getAddress());

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_RECEIVED,
      );
    });

    it("Should fail when the fee recipient fails errors", async () => {
      const expectedBytes = await encodeSendMessage(
        await l1MessageService.getAddress(),
        admin.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

      const claimMessageCall = l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          admin.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          await l1MessageService.getAddress(),
          EMPTY_CALLDATA,
          1,
        );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "FeePaymentFailed", [
        await l1MessageService.getAddress(),
      ]);

      expect(await l1MessageService.inboxL2L1MessageStatus(ethers.keccak256(expectedBytes))).to.be.equal(
        INBOX_STATUS_RECEIVED,
      );
    });

    it("Should revert with send over max limit amount only", async () => {
      await setHash(0n, INITIAL_WITHDRAW_LIMIT + 1n);

      const claimMessageCall = l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          0,
          INITIAL_WITHDRAW_LIMIT + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1,
        );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });

    it("Should revert with send over max limit amount and fees", async () => {
      await setHash(1n, INITIAL_WITHDRAW_LIMIT + 1n);

      const claimMessageCall = l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          1,
          INITIAL_WITHDRAW_LIMIT + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1,
        );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });

    it("Should revert with send over max limit amount and fees - multi tx", async () => {
      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      await setHash(1n, INITIAL_WITHDRAW_LIMIT - MESSAGE_VALUE_1ETH - MESSAGE_FEE + 1n);

      const claimMessageCall = l1MessageService
        .connect(admin)
        .claimMessage(
          await l1MessageService.getAddress(),
          notAuthorizedAccount.address,
          1,
          INITIAL_WITHDRAW_LIMIT - MESSAGE_VALUE_1ETH - MESSAGE_FEE + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1,
        );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });
  });

  describe("Claim Message with Proof", () => {
    it("Should be able to claim a message that was sent", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      await expect(
        l1MessageServiceMerkleProof.claimMessageWithProof({
          proof: VALID_MERKLE_PROOF.proof,
          messageNumber: 1,
          leafIndex: VALID_MERKLE_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        }),
      ).to.not.be.reverted;
    });

    it("Should be able to claim a message and emit a MessageClaimed event", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      const messageLeafHash = ethers.keccak256(
        ethers.AbiCoder.defaultAbiCoder().encode(
          ["address", "address", "uint256", "uint256", "uint256", "bytes"],
          [admin.address, admin.address, MESSAGE_FEE, MESSAGE_FEE + MESSAGE_VALUE_1ETH, "1", EMPTY_CALLDATA],
        ),
      );

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectEvent(l1MessageServiceMerkleProof, claimMessageCall, "MessageClaimed", [messageLeafHash]);
    });

    it("Should fail to claim when the contract is generally paused", async () => {
      await l1MessageServiceMerkleProof.connect(pauser).pauseByType(GENERAL_PAUSE_TYPE);

      await expect(
        l1MessageServiceMerkleProof.claimMessageWithProof({
          proof: VALID_MERKLE_PROOF.proof,
          messageNumber: 1,
          leafIndex: VALID_MERKLE_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        }),
      )
        .to.be.revertedWithCustomError(l1MessageService, "IsPaused")
        .withArgs(GENERAL_PAUSE_TYPE);
    });

    it("Should fail when the message has already been claimed", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      await l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageCall, "MessageAlreadyClaimed", [1]);
    });

    it("Should fail when l2 Merkle root does not exist on L1", async () => {
      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "L2MerkleRootDoesNotExist");
    });

    it("Should fail when l2 merkle proof is invalid", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: INVALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "InvalidMerkleProof");
    });

    it("Should fail claiming when the call transaction fails with receive()", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [INVALID_MERKLE_PROOF_REVERT.merkleRoot],
        INVALID_MERKLE_PROOF_REVERT.proof.length,
      );

      await expect(
        l1MessageServiceMerkleProof.claimMessageWithProof({
          proof: INVALID_MERKLE_PROOF_REVERT.proof,
          messageNumber: 1,
          leafIndex: INVALID_MERKLE_PROOF_REVERT.index,
          from: admin.address,
          to: await l1TestRevert.getAddress(),
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: INVALID_MERKLE_PROOF_REVERT.merkleRoot,
          data: "0xcd4aed30",
        }),
      ).to.be.reverted;
    });

    // TODO MAKE THIS DYNAMIC BECAUSE THE setNonce makes the testing brittle
    it("Should fail claiming when the call transaction fails with empty fallback", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [MERKLE_PROOF_FALLBACK.merkleRoot],
        MERKLE_PROOF_FALLBACK.proof.length,
      );

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: MERKLE_PROOF_FALLBACK.proof,
        messageNumber: 1,
        leafIndex: MERKLE_PROOF_FALLBACK.index,
        from: admin.address,
        to: await l1TestRevert.getAddress(),
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: MERKLE_PROOF_FALLBACK.merkleRoot,
        data: "0xce398a64",
      });

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "MessageSendingFailed", [
        await l1TestRevert.getAddress(),
      ]);
    });

    // TODO MAKE THIS DYNAMIC BECAUSE THE setNonce makes the testing brittle
    it("Should fail on reentry", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [MERKLE_PROOF_REENTRY.merkleRoot],
        MERKLE_PROOF_REENTRY.proof.length,
      );

      const claimMessageWithProof = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: MERKLE_PROOF_REENTRY.proof,
        messageNumber: 1,
        leafIndex: MERKLE_PROOF_REENTRY.index,
        from: admin.address,
        to: await l1MessageServiceMerkleProof.getAddress(),
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: MERKLE_PROOF_REENTRY.merkleRoot,
        data: "0xaf5696840000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000014000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266000000000000000000000000dc64a140aa3e981100a9beca4e685f962f0cf6c900000000000000000000000000000000000000000000000000b1a2bc2ec500000000000000000000000000000000000000000000000000000e92596fd62900000000000000000000000000000000000000000000000000000000000000000000c817003bf40005bdd4b6e06fd0ed2d01c27a89a4cf6ee67ea585489af54e4a8c000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
      });

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageWithProof, "ReentrantCall");
    });

    it("Should fail when the fee recipient fails errors", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: await l1MessageService.getAddress(),
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageCall, "FeePaymentFailed", [
        await l1MessageService.getAddress(),
      ]);
    });

    it("Should fail when the merkle depth is different than the proof length", async () => {
      await l1MessageServiceMerkleProof.addL2MerkleRoots(
        [VALID_MERKLE_PROOF.merkleRoot],
        VALID_MERKLE_PROOF.proof.length,
      );

      const merkleDepth = await l1MessageServiceMerkleProof.l2MerkleRootsDepths(VALID_MERKLE_PROOF.merkleRoot);

      const claimMessageCall = l1MessageServiceMerkleProof.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF.proof.slice(0, -1),
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: await l1MessageService.getAddress(),
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectRevertWithCustomError(
        l1MessageServiceMerkleProof,
        claimMessageCall,
        "ProofLengthDifferentThanMerkleDepth",
        [merkleDepth, VALID_MERKLE_PROOF.proof.slice(0, -1).length],
      );
    });
  });

  describe("Resetting limits", () => {
    it("Should reset limits as limitSetter", async () => {
      let usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0);

      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);

      await l1MessageService.connect(limitSetter).resetAmountUsedInPeriod();
      usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0);
    });

    it("Should fail reset limits as non-RATE_LIMIT_SETTER_ROLE", async () => {
      let usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0);

      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);

      await expectRevertWithReason(
        l1MessageService.connect(admin).resetAmountUsedInPeriod(),
        buildAccessErrorMessage(admin, USED_RATE_LIMIT_RESETTER_ROLE),
      );

      usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);
    });
  });

  describe("Pausing contracts", () => {
    it("Should fail general pausing as non-pauser", async () => {
      expect(await l1MessageService.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;

      await expect(l1MessageService.connect(admin).pauseByType(GENERAL_PAUSE_TYPE)).to.be.revertedWith(
        "AccessControl: account " + admin.address.toLowerCase() + " is missing role " + PAUSE_ALL_ROLE,
      );

      expect(await l1MessageService.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;
    });

    it("Should pause generally as pause manager", async () => {
      expect(await l1MessageService.isPaused(GENERAL_PAUSE_TYPE)).to.be.false;

      await l1MessageService.connect(pauser).pauseByType(GENERAL_PAUSE_TYPE);

      expect(await l1MessageService.isPaused(GENERAL_PAUSE_TYPE)).to.be.true;
    });

    it("Should fail when to claim the contract is generally paused", async () => {
      await l1MessageService.connect(pauser).pauseByType(GENERAL_PAUSE_TYPE);

      const claimMessageCall = l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1,
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should fail to claim when the L2 to L1 communication is paused", async () => {
      await l1MessageService.connect(pauser).pauseByType(L2_L1_PAUSE_TYPE);

      const claimMessageCall = l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1,
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "IsPaused", [L2_L1_PAUSE_TYPE]);
    });

    it("Should fail to send if the contract is generally paused", async () => {
      await l1MessageService.connect(pauser).pauseByType(GENERAL_PAUSE_TYPE);

      const claimMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, 0, EMPTY_CALLDATA, { value: INITIAL_WITHDRAW_LIMIT });

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "IsPaused", [GENERAL_PAUSE_TYPE]);

      const usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0);
    });

    it("Should fail to send if L1 to L2 communication is paused", async () => {
      await l1MessageService.connect(pauser).pauseByType(L1_L2_PAUSE_TYPE);

      const claimMessageCall = l1MessageService
        .connect(admin)
        .canSendMessage(notAuthorizedAccount.address, 0, EMPTY_CALLDATA, { value: INITIAL_WITHDRAW_LIMIT });

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "IsPaused", [L1_L2_PAUSE_TYPE]);

      const usedAmount = await l1MessageService.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0);
    });
  });

  async function setHash(fee: bigint, value: bigint) {
    const expectedBytes = await encodeSendMessage(
      await l1MessageService.getAddress(),
      notAuthorizedAccount.address,
      fee,
      value,
      1n,
      EMPTY_CALLDATA,
    );

    await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));
  }

  async function setHashAndClaimMessage(fee: bigint, value: bigint) {
    const expectedBytes = await encodeSendMessage(
      await l1MessageService.getAddress(),
      notAuthorizedAccount.address,
      fee,
      value,
      1n,
      EMPTY_CALLDATA,
    );

    await l1MessageService.addL2L1MessageHash(ethers.keccak256(expectedBytes));

    await expect(
      l1MessageService.claimMessage(
        await l1MessageService.getAddress(),
        notAuthorizedAccount.address,
        fee,
        value,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1,
      ),
    ).to.not.be.reverted;
  }
});
