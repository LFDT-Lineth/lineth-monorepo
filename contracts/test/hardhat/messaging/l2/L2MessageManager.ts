import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { TestL2MessageManager } from "../../../../typechain-types";
import {
  DEFAULT_ADMIN_ROLE,
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  L1_L2_MESSAGE_SETTER_ROLE,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "../../common/constants";
import { deployUpgradableFromFactory } from "../../common/deployment";
import {
  buildAccessErrorMessage,
  calculateRollingHashFromCollection,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateKeccak256Hash,
  generateNKeccak256Hashes,
} from "../../common/helpers";

describe("L2MessageManager", () => {
  let l2MessageManager: TestL2MessageManager;
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let pauser: SignerWithAddress;
  let l1l2MessageSetter: SignerWithAddress;
  let notAuthorizedAccount: SignerWithAddress;

  async function deployL2MessageManagerFixture() {
    return deployUpgradableFromFactory("TestL2MessageManager", [
      pauser.address,
      l1l2MessageSetter.address,
      pauseTypeRoles,
      unpauseTypeRoles,
    ]) as unknown as Promise<TestL2MessageManager>;
  }

  beforeEach(async () => {
    [admin, pauser, l1l2MessageSetter, notAuthorizedAccount] = await ethers.getSigners();
    l2MessageManager = await loadFixture(deployL2MessageManagerFixture);
  });

  describe("Initialization checks", () => {
    it("Deployer has DEFAULT_ADMIN_ROLE", async () => {
      expect(await l2MessageManager.hasRole(DEFAULT_ADMIN_ROLE, admin.address)).to.be.true;
    });
    it("l1l2MessageSetter has L1_L2_MESSAGE_SETTER_ROLE", async () => {
      expect(await l2MessageManager.hasRole(L1_L2_MESSAGE_SETTER_ROLE, l1l2MessageSetter.address)).to.be.true;
    });
  });

  describe("Add L1->L2 message hashes in 'inboxL1L2MessageStatus'", () => {
    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 2);
      await l2MessageManager.connect(pauser).pauseByType(GENERAL_PAUSE_TYPE);

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.connect(l1l2MessageSetter).anchorL1L2MessageHashes(messageHashes, 1, 100, HASH_ZERO),
        "IsPaused",
        [GENERAL_PAUSE_TYPE],
      );
    });

    it("Should revert anchorL1L2MessageHashes if the caller does not have the role 'L1_L2_MESSAGE_SETTER_ROLE'", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 2);

      await expectRevertWithReason(
        l2MessageManager.connect(notAuthorizedAccount).anchorL1L2MessageHashes(messageHashes, 1, 100, HASH_ZERO),
        buildAccessErrorMessage(notAuthorizedAccount, L1_L2_MESSAGE_SETTER_ROLE),
      );
    });

    it("Should revert if message hashes array length is zero", async () => {
      const messageHashes: [] = [];

      const anchorCall = l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 100, HASH_ZERO);

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "MessageHashesListLengthIsZero");
    });

    it("Should update rolling hash and messages emitting events", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 100));

      const anchorCall = l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 100, expectedRollingHash);

      await expectEvent(l2MessageManager, anchorCall, "L1L2MessageHashesAddedToInbox", [messageHashes]);
      await expectEvent(l2MessageManager, anchorCall, "RollingHashUpdated", [100, expectedRollingHash]);

      let mappedRollingHash = await l2MessageManager.l1RollingHashes(100);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      mappedRollingHash = await l2MessageManager.l1RollingHashes(100);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      expect(await l2MessageManager.lastAnchoredL1MessageNumber()).to.equal(100);
    });

    it("Should not emit events when a second anchoring is duplicated", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 100));

      const anchorCall = l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 100, expectedRollingHash);

      await expectEvent(l2MessageManager, anchorCall, "L1L2MessageHashesAddedToInbox", [messageHashes]);
      await expectEvent(l2MessageManager, anchorCall, "RollingHashUpdated", [100, expectedRollingHash]);

      let mappedRollingHash = await l2MessageManager.l1RollingHashes(100);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      mappedRollingHash = await l2MessageManager.l1RollingHashes(100);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      const transaction = await l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 101, 100, expectedRollingHash);

      const transactionReceipt = await transaction.wait();

      expect(transactionReceipt?.logs).to.be.empty;

      expect(await l2MessageManager.lastAnchoredL1MessageNumber()).to.equal(100);
    });

    it("Should update rolling hashes mapping ignoring 1 duplicate", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];
      await l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 99, expectedRollingHash);

      let mappedRollingHash = await l2MessageManager.l1RollingHashes(99);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      mappedRollingHash = await l2MessageManager.l1RollingHashes(100);
      expect(mappedRollingHash).to.equal(ethers.ZeroHash);

      expect(await l2MessageManager.lastAnchoredL1MessageNumber()).to.equal(99);
    });

    it("Should revert when message hashes array length is higher than 100", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 101);

      const anchorCall = l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 99, HASH_ZERO);

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "MessageHashesListLengthHigherThanOneHundred", [
        101,
      ]);
    });

    it("Should revert when final rolling hash is zero hash", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const anchorCall = l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 1, 99, HASH_ZERO);

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "FinalRollingHashIsZero");
    });

    it("Should revert the with mistmatched hashes", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const badRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes);

      const foundRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.connect(l1l2MessageSetter).anchorL1L2MessageHashes(messageHashes, 1, 99, badRollingHash),
        "L1RollingHashSynchronizationWrong",
        [badRollingHash, foundRollingHash],
      );
    });

    it("Should revert the with mistmatched counts", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const foundRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.connect(l1l2MessageSetter).anchorL1L2MessageHashes(messageHashes, 1, 100, foundRollingHash),
        "L1MessageNumberSynchronizationWrong",
        [100, 99],
      );
    });

    it("Should revert if L1 message number is out of sequence when lastAnchoredL1MessageNumber is higher than zero", async () => {
      await l2MessageManager.setLastAnchoredL1MessageNumber(100);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager
          .connect(l1l2MessageSetter)
          .anchorL1L2MessageHashes(messageHashes, 100, 199, expectedRollingHash),
        "L1MessageNumberSynchronizationWrong",
        [99, 100],
      );
    });

    it("Should update rolling hashes mapping ignoring 1 duplicate when lastAnchoredL1MessageNumber is higher than zero", async () => {
      await l2MessageManager.setLastAnchoredL1MessageNumber(100);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await l2MessageManager
        .connect(l1l2MessageSetter)
        .anchorL1L2MessageHashes(messageHashes, 101, 199, expectedRollingHash);

      let mappedRollingHash = await l2MessageManager.l1RollingHashes(199);
      expect(mappedRollingHash).to.equal(expectedRollingHash);

      mappedRollingHash = await l2MessageManager.l1RollingHashes(200);
      expect(mappedRollingHash).to.equal(ethers.ZeroHash);
    });

    it("Should NOT emit the ServiceVersionMigrated event when anchoring", async () => {
      await l2MessageManager.setLastAnchoredL1MessageNumber(100);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(ethers.ZeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expect(
        l2MessageManager
          .connect(l1l2MessageSetter)
          .anchorL1L2MessageHashes(messageHashes, 101, 199, expectedRollingHash),
      ).to.not.emit(l2MessageManager, "ServiceVersionMigrated");
    });
  });

  describe("Update L1->L2 message status to 'claimed' in 'inboxL1L2MessageStatus'", () => {
    it("Should revert if the message hash has not the status 'received' in 'inboxL1L2MessageStatus' mapping", async () => {
      const messageHash = generateKeccak256Hash("message");

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.updateL1L2MessageStatusToClaimed(messageHash),
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });
  });
});
