import { ethers } from "hardhat";
import { ZeroAddress } from "ethers";
import { expect } from "chai";
import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { deployFromFactory } from "../common/deployment";
import { expectEvent, expectEvents, expectRevertWithCustomError, expectRevertWithReason } from "../common/helpers";
import { LineaSequencerUptimeFeed, TestLineaSequencerUptimeFeedAccess } from "../../../typechain-types";

describe("LineaSequencerUptimeFeed", () => {
  let contract: LineaSequencerUptimeFeed;

  let admin: SignerWithAddress;
  let l2Sender: SignerWithAddress;
  let notAuthorized: SignerWithAddress;

  async function deployLineaSequencerUptimeFeedFixture() {
    const initialStatus = false; // Sequencer is up
    const adminAddress = admin.address;
    const l2SenderAddress = l2Sender.address;
    return deployFromFactory("LineaSequencerUptimeFeed", initialStatus, adminAddress, l2SenderAddress);
  }

  before(async () => {
    [admin, l2Sender, notAuthorized] = await ethers.getSigners();
  });

  beforeEach(async () => {
    contract = (await loadFixture(deployLineaSequencerUptimeFeedFixture)) as LineaSequencerUptimeFeed;
  });

  describe("constructor", () => {
    it("should revert when admin address is zero", async () => {
      const initialStatus = false;
      const adminAddress = ZeroAddress;
      await expectRevertWithCustomError(
        contract,
        deployFromFactory("LineaSequencerUptimeFeed", initialStatus, adminAddress, l2Sender.address),
        "ZeroAddressNotAllowed",
      );
    });

    it("should revert when l2Sender address is zero", async () => {
      const initialStatus = false;

      const l2SenderAddress = ZeroAddress;
      await expectRevertWithCustomError(
        contract,
        deployFromFactory("LineaSequencerUptimeFeed", initialStatus, admin.address, l2SenderAddress),
        "ZeroAddressNotAllowed",
      );
    });

    it("should set initial status correctly", async () => {
      const initialStatus = false;
      const contract = (await deployFromFactory(
        "LineaSequencerUptimeFeed",
        initialStatus,
        admin.address,
        l2Sender.address,
      )) as LineaSequencerUptimeFeed;
      const deployTx = contract.deploymentTransaction();
      const block = await ethers.provider.getBlock(deployTx!.blockNumber!);

      const latestAnswer = await contract.latestAnswer();
      expect(latestAnswer).to.equal(0n);
      expect(await contract.latestTimestamp()).to.be.equal(block!.timestamp);
      expect(await contract.latestRound()).to.be.equal(1n);
    });

    it("should set admin and l2Sender roles correctly", async () => {
      const initialStatus = false;
      const contract = (await deployFromFactory(
        "LineaSequencerUptimeFeed",
        initialStatus,
        admin.address,
        l2Sender.address,
      )) as LineaSequencerUptimeFeed;

      expect(await contract.hasRole(await contract.DEFAULT_ADMIN_ROLE(), admin.address)).to.be.true;
      expect(await contract.hasRole(await contract.FEED_UPDATER_ROLE(), l2Sender.address)).to.be.true;
    });
  });

  describe("typeAndVersion", () => {
    it("should return the correct type and version", async () => {
      const typeAndVersion = await contract.typeAndVersion();
      expect(typeAndVersion).to.equal("LineaSequencerUptimeFeed 1.0.0");
    });
  });

  describe("updateStatus", () => {
    it("should revert if caller does not have the FEED_UPDATER_ROLE", async () => {
      const newStatus = false;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;

      await expectRevertWithCustomError(
        contract,
        contract.connect(notAuthorized).updateStatus(newStatus, timestamp),
        "InvalidSender",
      );
    });

    it("should ignore update if latest recorded timestamp is newer", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;

      await time.setNextBlockTimestamp(timestamp);

      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);
      const updatedAt = (await contract.latestRoundData()).updatedAt;

      expect(updatedAt).to.equal(timestamp);

      const oldTimestamp = timestamp - 1;
      await time.setNextBlockTimestamp(timestamp + 1);

      await expectEvent(contract, contract.connect(l2Sender).updateStatus(newStatus, oldTimestamp), "UpdateIgnored", [
        newStatus,
        updatedAt,
        true,
        oldTimestamp,
      ]);

      const latestRoundData = await contract.latestRoundData();

      expect(latestRoundData.roundId).to.equal(2n);
      expect(latestRoundData.answer).to.equal(1n);
      expect(latestRoundData.startedAt).to.be.equal(timestamp);
    });

    it("should update round and emit RoundUpdated event when latestStatus === _status", async () => {
      const newStatus = false;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);

      await expectEvent(contract, contract.connect(l2Sender).updateStatus(newStatus, timestamp), "RoundUpdated", [
        0n,
        timestamp,
      ]);

      const latestRoundData = await contract.latestRoundData();

      expect(latestRoundData.roundId).to.equal(1n);
      expect(latestRoundData.answer).to.equal(0n);
      expect(latestRoundData.startedAt).to.be.equal(timestamp - 10);
      expect(latestRoundData.updatedAt).to.be.equal(timestamp);
      expect(latestRoundData.answeredInRound).to.equal(1n);
    });

    it("should record round and emit AnswerUpdated event when latestStatus !== _status", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);

      await expectEvents(contract, contract.connect(l2Sender).updateStatus(newStatus, timestamp), [
        {
          name: "NewRound",
          args: [2n, l2Sender.address, timestamp],
        },
        {
          name: "AnswerUpdated",
          args: [1n, 2n, timestamp],
        },
      ]);

      const latestRoundData = await contract.latestRoundData();

      expect(latestRoundData.roundId).to.equal(2n);
      expect(latestRoundData.answer).to.equal(1n);
      expect(latestRoundData.startedAt).to.be.equal(timestamp);
      expect(latestRoundData.updatedAt).to.be.equal(timestamp);
      expect(latestRoundData.answeredInRound).to.equal(2n);
    });
  });

  describe("latestAnswer", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callLatestAnswer(), "No access");
    });

    it("should return the latest answer", async () => {
      const latestAnswer = await contract.latestAnswer();
      expect(latestAnswer).to.equal(0n);
    });
  });

  describe("latestRoundData", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callLatestRoundData(), "No access");
    });

    it("should return the latest round data", async () => {
      const latestRoundData = await contract.latestRoundData();
      expect(latestRoundData.roundId).to.equal(1n);
      expect(latestRoundData.answer).to.equal(0n);
      expect(latestRoundData.startedAt).to.be.greaterThan(0n);
      expect(latestRoundData.updatedAt).to.be.greaterThan(0n);
      expect(latestRoundData.answeredInRound).to.equal(1n);
    });
  });

  describe("getAnswer", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callGetAnswer(0n), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.getAnswer(roundId), "NoDataPresent");
    });

    it("should return the correct answer for a given roundId", async () => {
      const newStatus = false;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);
      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);

      const roundId = 1n;
      const answer = await contract.getAnswer(roundId);
      expect(answer).to.equal(0n);
    });
  });

  describe("getRoundData", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callGetRoundData(0n), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.getRoundData(roundId), "NoDataPresent");
    });

    it("should return the correct round data for a given roundId", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);
      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);

      const block = await ethers.provider.getBlock("latest");
      expect(block!.timestamp).to.equal(timestamp);

      const [roundId, answer, startedAt, updatedAt, answeredInRound] = await contract.getRoundData(2n);
      expect(roundId).to.equal(2n);
      expect(answer).to.equal(1n);
      expect(startedAt).to.equal(timestamp);
      expect(updatedAt).to.equal(timestamp);
      expect(answeredInRound).to.equal(2n);
    });
  });

  describe("getTimestamp", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callGetTimestamp(0n), "No access");
    });

    it("should revert if roundId does not exist", async () => {
      const roundId = 2n;
      await expectRevertWithCustomError(contract, contract.getTimestamp(roundId), "NoDataPresent");
    });

    it("should return the correct timestamp for a given roundId", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);
      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);

      const roundId = 2n;
      const roundTimestamp = await contract.getTimestamp(roundId);
      expect(roundTimestamp).to.equal(timestamp);
    });
  });

  describe("latestTimestamp", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callLatestTimestamp(), "No access");
    });

    it("should return the latest timestamp", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);
      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);

      const latestTimestamp = await contract.latestTimestamp();
      expect(latestTimestamp).to.equal(timestamp);
    });
  });

  describe("latestRound", () => {
    it("should revert if caller is not part of the access list or not the tx.origin", async () => {
      const callerContract = (await deployFromFactory(
        "TestLineaSequencerUptimeFeedAccess",
        await contract.getAddress(),
      )) as TestLineaSequencerUptimeFeedAccess;

      await expectRevertWithReason(callerContract.callLatestRound(), "No access");
    });

    it("should return the latest round ID", async () => {
      const newStatus = true;
      const currentBlockTs = await time.latest();
      const timestamp = currentBlockTs + 10;
      await time.setNextBlockTimestamp(timestamp);
      await contract.connect(l2Sender).updateStatus(newStatus, timestamp);

      const latestRound = await contract.latestRound();
      expect(latestRound).to.equal(2n);
    });
  });
});
