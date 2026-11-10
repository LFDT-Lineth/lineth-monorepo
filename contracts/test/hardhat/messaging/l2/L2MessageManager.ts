import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { type Hex, zeroHash } from "viem";

import {
  DEFAULT_ADMIN_ROLE,
  GENERAL_PAUSE_TYPE,
  HASH_ZERO,
  L1_L2_MESSAGE_SETTER_ROLE,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "../../common/constants/index.js";
import { deployUpgradableFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  calculateRollingHashFromCollection,
  expectHasRole,
  expectNoEvent,
  expectRevertWhenPaused,
  generateKeccak256Hash,
  generateNKeccak256Hashes,
  getTestAccounts,
  validateRollingHashStorage,
  validateRollingHashIsZero,
  expectRevertWithReason,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("L2MessageManager", () => {
  let l2MessageManager: ContractReturnType<"TestL2MessageManager">;

  let admin: TestAccount;
  let pauser: TestAccount;
  let l1l2MessageSetter: TestAccount;
  let notAuthorizedAccount: TestAccount;

  async function deployL2MessageManagerFixture() {
    return deployUpgradableFromFactory("TestL2MessageManager", [
      pauser.address,
      l1l2MessageSetter.address,
      pauseTypeRoles,
      unpauseTypeRoles,
    ]);
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    [admin, pauser, l1l2MessageSetter, notAuthorizedAccount] = await getTestAccounts();
    l2MessageManager = await networkHelpers.loadFixture(deployL2MessageManagerFixture);
  });

  describe("Initialization checks", () => {
    it("Deployer has DEFAULT_ADMIN_ROLE", async () => {
      await expectHasRole(l2MessageManager, DEFAULT_ADMIN_ROLE, admin);
    });
    it("l1l2MessageSetter has L1_L2_MESSAGE_SETTER_ROLE", async () => {
      await expectHasRole(l2MessageManager, L1_L2_MESSAGE_SETTER_ROLE, l1l2MessageSetter);
    });
  });

  describe("Add L1->L2 message hashes in 'inboxL1L2MessageStatus'", () => {
    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 2);
      await l2MessageManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: pauser.account });

      await expectRevertWhenPaused(
        l2MessageManager,
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 100n, HASH_ZERO], {
          account: l1l2MessageSetter.account,
        }),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should revert anchorL1L2MessageHashes if the caller does not have the role 'L1_L2_MESSAGE_SETTER_ROLE'", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 2);

      await expectRevertWithReason(
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 100n, HASH_ZERO], {
          account: notAuthorizedAccount.account,
        }),
        buildAccessErrorMessage(notAuthorizedAccount, L1_L2_MESSAGE_SETTER_ROLE),
      );
    });

    it("Should revert if message hashes array length is zero", async () => {
      const messageHashes: Hex[] = [];

      const anchorCall = l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 100n, HASH_ZERO], {
        account: l1l2MessageSetter.account,
      });

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "MessageHashesListLengthIsZero");
    });

    it("Should update rolling hash and messages emitting events", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 100));

      const anchorCall = l2MessageManager.write.anchorL1L2MessageHashes(
        [messageHashes, 1n, 100n, expectedRollingHash],
        { account: l1l2MessageSetter.account },
      );

      await expectEvent(l2MessageManager, anchorCall, "L1L2MessageHashesAddedToInbox", [messageHashes]);
      await expectEvent(l2MessageManager, anchorCall, "RollingHashUpdated", [100n, expectedRollingHash]);

      await validateRollingHashStorage({
        contract: l2MessageManager,
        messageNumber: 100n,
        expectedHash: expectedRollingHash,
        isL1RollingHash: true,
      });

      expect(await l2MessageManager.read.lastAnchoredL1MessageNumber()).to.equal(100n);
    });

    it("Should not emit events when a second anchoring is duplicated", async () => {
      const { viem } = await network.getOrCreate();
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 100));

      const anchorCall = l2MessageManager.write.anchorL1L2MessageHashes(
        [messageHashes, 1n, 100n, expectedRollingHash],
        { account: l1l2MessageSetter.account },
      );

      await expectEvent(l2MessageManager, anchorCall, "L1L2MessageHashesAddedToInbox", [messageHashes]);
      await expectEvent(l2MessageManager, anchorCall, "RollingHashUpdated", [100n, expectedRollingHash]);

      await validateRollingHashStorage({
        contract: l2MessageManager,
        messageNumber: 100n,
        expectedHash: expectedRollingHash,
        isL1RollingHash: true,
      });

      const publicClient = await viem.getPublicClient();
      const hash = await l2MessageManager.write.anchorL1L2MessageHashes(
        [messageHashes, 101n, 100n, expectedRollingHash],
        { account: l1l2MessageSetter.account },
      );

      const transactionReceipt = await publicClient.waitForTransactionReceipt({ hash });

      expect(transactionReceipt?.logs).to.be.empty;

      expect(await l2MessageManager.read.lastAnchoredL1MessageNumber()).to.equal(100n);
    });

    it("Should update rolling hashes mapping ignoring 1 duplicate", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];
      await l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 99n, expectedRollingHash], {
        account: l1l2MessageSetter.account,
      });

      await validateRollingHashStorage({
        contract: l2MessageManager,
        messageNumber: 99n,
        expectedHash: expectedRollingHash,
        isL1RollingHash: true,
      });
      await validateRollingHashIsZero({
        contract: l2MessageManager,
        messageNumber: 100n,
        isL1RollingHash: true,
      });

      expect(await l2MessageManager.read.lastAnchoredL1MessageNumber()).to.equal(99n);
    });

    it("Should revert when message hashes array length is higher than 100", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 101);

      const anchorCall = l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 99n, HASH_ZERO], {
        account: l1l2MessageSetter.account,
      });

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "MessageHashesListLengthHigherThanOneHundred", [
        101n,
      ]);
    });

    it("Should revert when final rolling hash is zero hash", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const anchorCall = l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 99n, HASH_ZERO], {
        account: l1l2MessageSetter.account,
      });

      await expectRevertWithCustomError(l2MessageManager, anchorCall, "FinalRollingHashIsZero");
    });

    it("Should revert the with mistmatched hashes", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);
      const badRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes);

      const foundRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 99n, badRollingHash], {
          account: l1l2MessageSetter.account,
        }),
        "L1RollingHashSynchronizationWrong",
        [badRollingHash, foundRollingHash],
      );
    });

    it("Should revert the with mistmatched counts", async () => {
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const foundRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 1n, 100n, foundRollingHash], {
          account: l1l2MessageSetter.account,
        }),
        "L1MessageNumberSynchronizationWrong",
        [100n, 99n],
      );
    });

    it("Should revert if L1 message number is out of sequence when lastAnchoredL1MessageNumber is higher than zero", async () => {
      await l2MessageManager.write.setLastAnchoredL1MessageNumber([100n]);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 100n, 199n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        }),
        "L1MessageNumberSynchronizationWrong",
        [99n, 100n],
      );
    });

    it("Should update rolling hashes mapping ignoring 1 duplicate when lastAnchoredL1MessageNumber is higher than zero", async () => {
      await l2MessageManager.write.setLastAnchoredL1MessageNumber([100n]);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 101n, 199n, expectedRollingHash], {
        account: l1l2MessageSetter.account,
      });

      await validateRollingHashStorage({
        contract: l2MessageManager,
        messageNumber: 199n,
        expectedHash: expectedRollingHash,
        isL1RollingHash: true,
      });
      await validateRollingHashIsZero({
        contract: l2MessageManager,
        messageNumber: 200n,
        isL1RollingHash: true,
      });
    });

    it("Should NOT emit the ServiceVersionMigrated event when anchoring", async () => {
      await l2MessageManager.write.setLastAnchoredL1MessageNumber([100n]);
      const messageHashes = generateNKeccak256Hashes("message", 100);

      const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, messageHashes.slice(0, 99));

      // forced duplicate
      messageHashes[99] = messageHashes[98];

      await expectNoEvent(
        l2MessageManager,
        l2MessageManager.write.anchorL1L2MessageHashes([messageHashes, 101n, 199n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        }),
        "ServiceVersionMigrated",
      );
    });
  });

  describe("Update L1->L2 message status to 'claimed' in 'inboxL1L2MessageStatus'", () => {
    it("Should revert if the message hash has not the status 'received' in 'inboxL1L2MessageStatus' mapping", async () => {
      const messageHash = generateKeccak256Hash("message");

      await expectRevertWithCustomError(
        l2MessageManager,
        l2MessageManager.write.updateL1L2MessageStatusToClaimed([messageHash]),
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });
  });
});
