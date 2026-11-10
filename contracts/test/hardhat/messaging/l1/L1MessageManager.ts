import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";

import { INBOX_STATUS_UNKNOWN } from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import {
  expectNoEvent,
  generateKeccak256Hash,
  generateL2MessagingBlocksOffsets,
  generateRandomBytes,
  range,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("L1MessageManager", () => {
  let l1MessageManager: ContractReturnType<"TestL1MessageManager">;

  async function deployTestL1MessageManagerFixture(): Promise<ContractReturnType<"TestL1MessageManager">> {
    return deployFromFactory("TestL1MessageManager");
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    l1MessageManager = await networkHelpers.loadFixture(deployTestL1MessageManagerFixture);
  });

  describe("Update L2->L1 message hash status in 'inboxL2L1MessageStatus' mapping to 'claimed'", () => {
    it("Should revert if the message hash has not the status 'received' in 'inboxL2L1MessageStatus' mapping", async () => {
      const messageHash = generateKeccak256Hash("message1");

      await l1MessageManager.write.addL2L1MessageHash([messageHash]);
      await l1MessageManager.write.updateL2L1MessageStatusToClaimed([messageHash]);

      await expectRevertWithCustomError(
        l1MessageManager,
        l1MessageManager.write.updateL2L1MessageStatusToClaimed([messageHash]),
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
      );
    });

    it("Should succeed if message hash has the status 'received' in 'inboxL2L1MessageStatus' mapping", async () => {
      const messageHash = generateKeccak256Hash("message1");

      await l1MessageManager.write.addL2L1MessageHash([messageHash]);
      await l1MessageManager.write.updateL2L1MessageStatusToClaimed([messageHash]);

      expect(await l1MessageManager.read.inboxL2L1MessageStatus([messageHash])).to.equal(BigInt(INBOX_STATUS_UNKNOWN));
    });
  });

  describe("Set L2->L1 message status in '_messageClaimedBitMap' mapping to 'claimed'", () => {
    it("Should failed if message has already been claimed", async () => {
      const messageNumber = 1n;
      await l1MessageManager.write.setL2L1MessageToClaimed([messageNumber]);

      await expectRevertWithCustomError(
        l1MessageManager,
        l1MessageManager.write.setL2L1MessageToClaimed([messageNumber]),
        "MessageAlreadyClaimed",
        [messageNumber],
      );
    });

    it("Should set the message as claimed", async () => {
      const messagesNumber = [1n, 2n, 3n];
      const [firstMessage, secondMessage, thirdMessage] = messagesNumber;

      await Promise.all([
        l1MessageManager.write.setL2L1MessageToClaimed([firstMessage]),
        l1MessageManager.write.setL2L1MessageToClaimed([secondMessage]),
        l1MessageManager.write.setL2L1MessageToClaimed([thirdMessage]),
      ]);

      const messagesStatus = await Promise.all([
        l1MessageManager.read.isMessageClaimed([firstMessage]),
        l1MessageManager.read.isMessageClaimed([secondMessage]),
        l1MessageManager.read.isMessageClaimed([thirdMessage]),
      ]);

      expect(messagesStatus).to.deep.equal([true, true, true]);
    });
  });

  describe("Add L2 merkle root in 'l2MerkleRootsDepths' mapping", () => {
    it("Should revert if the merkle root already exists", async () => {
      const merkleRoot = generateRandomBytes(32);
      const treeDepth = 32n;

      await l1MessageManager.write.addL2MerkleRoots([[merkleRoot], treeDepth]);
      await expectRevertWithCustomError(
        l1MessageManager,
        l1MessageManager.write.addL2MerkleRoots([[merkleRoot], treeDepth]),
        "L2MerkleRootAlreadyAnchored",
        [merkleRoot],
      );
    });

    it("Should add the new root to contract storage and emit a 'L2MerkleRootAdded' event", async () => {
      const merkleRoot = generateRandomBytes(32);
      const treeDepth = 32n;

      await expectEvent(
        l1MessageManager,
        l1MessageManager.write.addL2MerkleRoots([[merkleRoot], treeDepth]),
        "L2MerkleRootAdded",
        [merkleRoot, treeDepth],
      );

      expect(await l1MessageManager.read.l2MerkleRootsDepths([merkleRoot])).to.not.equal(0n);
    });
  });

  describe("Anchor L2 messaging blocks on L1", () => {
    it("Should fail when '_l2MessagingBlocksOffsets' length is not a multiple of 2", async () => {
      const currentL2BlockNumber = 10n;
      await expectRevertWithCustomError(
        l1MessageManager,
        l1MessageManager.write.anchorL2MessagingBlocks(["0x01", currentL2BlockNumber]),
        "BytesLengthNotMultipleOfTwo",
        [1n],
      );
    });

    it("Should not emit events when '_l2MessagingBlocksOffsets' is empty", async () => {
      const currentL2BlockNumber = 10n;
      await expectNoEvent(
        l1MessageManager,
        l1MessageManager.write.anchorL2MessagingBlocks(["0x", currentL2BlockNumber]),
        "L2MessagingBlockAnchored",
      );
    });

    it("Should anchor L2 messaging blocks on L1 when the input is not an empty array", async () => {
      const { viem } = await network.getOrCreate();
      const currentL2BlockNumber = 10_000_000n;

      const arr = range(1, 50);
      const l2MessagingBlocks = generateL2MessagingBlocksOffsets(1, 50);

      const publicClient = await viem.getPublicClient();
      const hash = await l1MessageManager.write.anchorL2MessagingBlocks([l2MessagingBlocks, currentL2BlockNumber]);
      const receipt = await publicClient.waitForTransactionReceipt({ hash });

      expect(receipt).to.not.be.undefined;
      const events = await l1MessageManager.getEvents.L2MessagingBlockAnchored({}, { fromBlock: 0n });

      expect(events.length).to.equal(50);

      for (let i = 0; i < events.length; i++) {
        expect(events[i].args.l2Block).to.deep.equal(currentL2BlockNumber + BigInt(arr[i]));
      }
    });
  });

  describe("Check if L2->L1 message has been claimed on L1 or not", () => {
    it("Should return false if the message has not been claimed", async () => {
      const messageNumber = 1n;
      expect(await l1MessageManager.read.isMessageClaimed([messageNumber])).to.equal(false);
    });

    it("Should return true if the message has been claimed", async () => {
      const messageNumber = 1n;
      await l1MessageManager.write.setL2L1MessageToClaimed([messageNumber]);
      expect(await l1MessageManager.read.isMessageClaimed([messageNumber])).to.equal(true);
    });
  });
});
