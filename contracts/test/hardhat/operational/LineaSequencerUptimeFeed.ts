import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { zeroAddress } from "viem";

import { deployFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  getLastBlockTimestamp,
  getTestAccounts,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
  expectEvents,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("LineaSequencerUptimeFeed", () => {
  let contract: ContractReturnType<"LineaSequencerUptimeFeed">;

  let admin: TestAccount;
  let l2Sender: TestAccount;
  let notAuthorized: TestAccount;

  async function deployLineaSequencerUptimeFeedFixture() {
    const initialStatus = false; // Sequencer is up
    const adminAddress = admin.address;
    const l2SenderAddress = l2Sender.address;
    return deployFromFactory("LineaSequencerUptimeFeed", [initialStatus, adminAddress, l2SenderAddress]);
  }

  before(async () => {
    [admin, l2Sender, notAuthorized] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    contract = await networkHelpers.loadFixture(deployLineaSequencerUptimeFeedFixture);
  });

  describe("constructor", () => {
    it("should revert when admin address is zero", async () => {
      const initialStatus = false;
      const adminAddress = zeroAddress;
      await expectRevertWithCustomError(
        contract,
        deployFromFactory("LineaSequencerUptimeFeed", [initialStatus, adminAddress, l2Sender.address]),
        "ZeroAddressNotAllowed",
      );
    });

    it("should revert when l2Sender address is zero", async () => {
      const initialStatus = false;

      const l2SenderAddress = zeroAddress;
      await expectRevertWithCustomError(
        contract,
        deployFromFactory("LineaSequencerUptimeFeed", [initialStatus, admin.address, l2SenderAddress]),
        "ZeroAddressNotAllowed",
      );
    });

    it("should set initial status correctly", async () => {
      const { viem } = await network.getOrCreate();
      const initialStatus = false;
      const publicClient = await viem.getPublicClient();
      const { contract, deploymentTransaction } = await viem.sendDeploymentTransaction("LineaSequencerUptimeFeed", [
        initialStatus,
        admin.address,
        l2Sender.address,
      ]);
      const deployReceipt = await publicClient.waitForTransactionReceipt({ hash: deploymentTransaction.hash });
      const block = await publicClient.getBlock({ blockNumber: deployReceipt.blockNumber });

      const latestAnswer = await contract.read.latestAnswer();
      expect(latestAnswer).to.equal(0n);
      expect(await contract.read.latestTimestamp()).to.be.equal(block.timestamp);
      expect(await contract.read.latestRound()).to.be.equal(1n);
    });

    it("should set admin and l2Sender roles correctly", async () => {
      const initialStatus = false;
      const contract = await deployFromFactory("LineaSequencerUptimeFeed", [
        initialStatus,
        admin.address,
        l2Sender.address,
      ]);

      expect(await contract.read.hasRole([await contract.read.DEFAULT_ADMIN_ROLE(), admin.address])).to.be.true;
      expect(await contract.read.hasRole([await contract.read.FEED_UPDATER_ROLE(), l2Sender.address])).to.be.true;
    });
  });

  describe("typeAndVersion", () => {
    it("should return the correct type and version", async () => {
      const typeAndVersion = await contract.read.typeAndVersion();
      expect(typeAndVersion).to.equal("LineaSequencerUptimeFeed 1.0.0");
    });
  });

  describe("updateStatus", () => {
    it("should revert if caller does not have the FEED_UPDATER_ROLE", async () => {
      const newStatus = false;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;

      await expectRevertWithCustomError(
        contract,
        contract.write.updateStatus([newStatus, timestamp], { account: notAuthorized.account }),
        "InvalidSender",
      );
    });

    it("should ignore update if latest recorded timestamp is newer", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;

      await networkHelpers.time.setNextBlockTimestamp(timestamp);

      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });
      const [, , , updatedAt] = await contract.read.latestRoundData();

      expect(updatedAt).to.equal(timestamp);

      const oldTimestamp = timestamp - 1n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp + 1n);

      await expectEvent(
        contract,
        contract.write.updateStatus([newStatus, oldTimestamp], { account: l2Sender.account }),
        "UpdateIgnored",
        [newStatus, updatedAt, true, oldTimestamp],
      );

      const [roundId, answer, startedAt] = await contract.read.latestRoundData();

      expect(roundId).to.equal(2n);
      expect(answer).to.equal(1n);
      expect(startedAt).to.be.equal(timestamp);
    });

    it("should update round and emit RoundUpdated event when latestStatus === _status", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = false;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);

      await expectEvent(
        contract,
        contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account }),
        "RoundUpdated",
        [0n, timestamp],
      );

      const [roundId, answer, startedAt, updatedAt, answeredInRound] = await contract.read.latestRoundData();

      expect(roundId).to.equal(1n);
      expect(answer).to.equal(0n);
      expect(startedAt).to.be.equal(timestamp - 10n);
      expect(updatedAt).to.be.equal(timestamp);
      expect(answeredInRound).to.equal(1n);
    });

    it("should record round and emit AnswerUpdated event when latestStatus !== _status", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);

      await expectEvents(contract, contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account }), [
        {
          name: "NewRound",
          args: [2n, l2Sender.address, timestamp],
        },
        {
          name: "AnswerUpdated",
          args: [1n, 2n, timestamp],
        },
      ]);

      const [roundId, answer, startedAt, updatedAt, answeredInRound] = await contract.read.latestRoundData();

      expect(roundId).to.equal(2n);
      expect(answer).to.equal(1n);
      expect(startedAt).to.be.equal(timestamp);
      expect(updatedAt).to.be.equal(timestamp);
      expect(answeredInRound).to.equal(2n);
    });
  });

  describe("latestAnswer", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callLatestAnswer(), "No access");
    });

    it("should return the latest answer", async () => {
      const latestAnswer = await contract.read.latestAnswer();
      expect(latestAnswer).to.equal(0n);
    });
  });

  describe("latestRoundData", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callLatestRoundData(), "No access");
    });

    it("should return the latest round data", async () => {
      const [roundId, answer, startedAt, updatedAt, answeredInRound] = await contract.read.latestRoundData();
      expect(roundId).to.equal(1n);
      expect(answer).to.equal(0n);
      expect(startedAt).to.be.greaterThan(0n);
      expect(updatedAt).to.be.greaterThan(0n);
      expect(answeredInRound).to.equal(1n);
    });
  });

  describe("getAnswer", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callGetAnswer([0n]), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.read.getAnswer([roundId]), "NoDataPresent");
    });

    it("should return the correct answer for a given roundId", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = false;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);
      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });

      const roundId = 1n;
      const answer = await contract.read.getAnswer([roundId]);
      expect(answer).to.equal(0n);
    });
  });

  describe("getRoundData", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callGetRoundData([0n]), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.read.getRoundData([roundId]), "NoDataPresent");
    });

    it("should return the correct round data for a given roundId", async () => {
      const { viem, networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);
      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });

      const block = await (await viem.getPublicClient()).getBlock();
      expect(block.timestamp).to.equal(timestamp);

      const [roundId, answer, startedAt, updatedAt, answeredInRound] = await contract.read.getRoundData([2n]);
      expect(roundId).to.equal(2n);
      expect(answer).to.equal(1n);
      expect(startedAt).to.equal(timestamp);
      expect(updatedAt).to.equal(timestamp);
      expect(answeredInRound).to.equal(2n);
    });
  });

  describe("getTimestamp", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callGetTimestamp([0n]), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.read.getTimestamp([roundId]), "NoDataPresent");
    });

    it("should return the correct timestamp for a given roundId", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);
      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });

      const roundId = 2n;
      const roundTimestamp = await contract.read.getTimestamp([roundId]);
      expect(roundTimestamp).to.equal(timestamp);
    });
  });

  describe("latestTimestamp", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callLatestTimestamp(), "No access");
    });

    it("should return the latest timestamp", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);
      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });

      const latestTimestamp = await contract.read.latestTimestamp();
      expect(latestTimestamp).to.equal(timestamp);
    });
  });

  describe("latestRound", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = await deployFromFactory("TestLineaSequencerUptimeFeedAccess", [contract.address]);

      await expectRevertWithReason(callerContract.read.callLatestRound(), "No access");
    });

    it("should return the latest round ID", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const newStatus = true;
      const currentBlockTs = await getLastBlockTimestamp();
      const timestamp = currentBlockTs + 10n;
      await networkHelpers.time.setNextBlockTimestamp(timestamp);
      await contract.write.updateStatus([newStatus, timestamp], { account: l2Sender.account });

      const latestRound = await contract.read.latestRound();
      expect(latestRound).to.equal(2n);
    });
  });
});
