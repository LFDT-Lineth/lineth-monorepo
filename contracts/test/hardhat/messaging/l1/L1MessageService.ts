import { expect } from "chai";
import { describe, it, before, beforeEach } from "node:test";
import { type Hash, BaseError, encodeAbiParameters, keccak256, parseAbiParameters, slice } from "viem";

import { encodeSendMessage } from "../../../../common/helpers/encoding.js";
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
} from "../../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  getConnection,
  useFreshChain,
  calculateRollingHash,
  expectHasRole,
  expectRevertWhenPaused,
  expectPaused,
  expectNotPaused,
  generateKeccak256Hash,
  getTestAccounts,
  hex,
  validateRollingHashStorage,
  validateRollingHashNotZero,
  expectRollingHashUpdatedEvent,
  INITIAL_ROLLING_HASH,
  expectRevertWithReason,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";

import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";

/** Merkle proof fixture with its untyped JSON-like strings validated as `Hex`. */
function typedProof(proofData: { proof: string[]; merkleRoot: string; index: number }) {
  return { proof: proofData.proof.map(hex), merkleRoot: hex(proofData.merkleRoot), index: proofData.index };
}

const VALID_PROOF = typedProof(VALID_MERKLE_PROOF);
const INVALID_PROOF = typedProof(INVALID_MERKLE_PROOF);
const INVALID_PROOF_REVERT = typedProof(INVALID_MERKLE_PROOF_REVERT);
const PROOF_FALLBACK = typedProof(MERKLE_PROOF_FALLBACK);
const PROOF_REENTRY = typedProof(MERKLE_PROOF_REENTRY);

describe("L1MessageService", () => {
  // The merkle proof fixtures pin contract addresses to fixed deployer nonces, so this suite needs a pristine chain
  // (Hardhat 2 used `hardhat_reset`, which Hardhat 3 does not support).
  useFreshChain();

  let l1MessageService: ContractReturnType<"TestL1MessageService">;
  let l1MessageServiceMerkleProof: ContractReturnType<"TestL1MessageServiceMerkleProof">;
  let l1TestRevert: ContractReturnType<"TestL1RevertContract">;
  let admin: TestAccount;
  let pauser: TestAccount;
  let limitSetter: TestAccount;
  let notAuthorizedAccount: TestAccount;
  let postmanAddress: TestAccount;
  let l2Sender: TestAccount;
  let publicClient: PublicClient;

  async function deployTestL1MessageServiceFixture(): Promise<ContractReturnType<"TestL1MessageService">> {
    return deployUpgradableFromFactory(
      "TestL1MessageService",
      [ONE_DAY_IN_SECONDS, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles],
      { unsafeAllow: ["incorrect-initializer-order"] },
    );
  }

  async function deployL1MessageServiceMerkleFixture(): Promise<ContractReturnType<"TestL1MessageServiceMerkleProof">> {
    return deployUpgradableFromFactory(
      "TestL1MessageServiceMerkleProof",
      [ONE_DAY_IN_SECONDS, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles],
      { unsafeAllow: ["incorrect-initializer-order"] },
    );
  }

  async function deployL1TestRevertFixture(): Promise<ContractReturnType<"TestL1RevertContract">> {
    return deployUpgradableFromFactory("TestL1RevertContract", []);
  }

  /** Awaits a write and asserts its transaction succeeded (ethers' `.to.not.be.reverted`). */
  async function expectNotReverted(call: Promise<Hash>): Promise<void> {
    const receipt = await publicClient.waitForTransactionReceipt({ hash: await call });
    expect(receipt.status).to.equal("success");
  }

  /** Awaits a write and asserts it reverted, with or without a custom error (ethers' `.to.be.reverted`). */
  async function expectReverted(call: Promise<Hash>): Promise<void> {
    let error: unknown;
    try {
      await call;
    } catch (e) {
      error = e;
    }
    expect(error, "Expected transaction to be reverted, but it did not revert").to.be.instanceOf(BaseError);
  }

  before(async () => {
    const { networkHelpers, viem } = await getConnection();
    publicClient = await viem.getPublicClient();
    [admin, pauser, limitSetter, notAuthorizedAccount, postmanAddress, l2Sender] = await getTestAccounts();
    // TODO adjust the tests to dynamically use whatever nonce is set for the merkle proof
    // Nonce 2 (was 1 under Hardhat 2): the Hardhat 2 OZ plugin also deployed a shared ProxyAdmin from the deployer
    // during the first proxy deployment, while OZ 5 proxies create their admin internally. Starting one nonce later
    // keeps the merkle proof fixtures' contract addresses (TestL1MessageServiceMerkleProof proxy at deployer nonce 5,
    // TestL1RevertContract proxy at nonce 7) unchanged.
    await networkHelpers.setNonce(admin.address, 2);
  });

  beforeEach(async () => {
    const { networkHelpers } = await getConnection();
    l1MessageService = await networkHelpers.loadFixture(deployTestL1MessageServiceFixture);
    l1MessageServiceMerkleProof = await networkHelpers.loadFixture(deployL1MessageServiceMerkleFixture);
    l1TestRevert = await networkHelpers.loadFixture(deployL1TestRevertFixture);

    await l1MessageService.write.grantRole([PAUSE_L1_L2_ROLE, pauser.address]);
    await l1MessageService.write.grantRole([PAUSE_L2_L1_ROLE, pauser.address]);
    await l1MessageService.write.grantRole([PAUSE_ALL_ROLE, pauser.address]);
    await l1MessageService.write.grantRole([UNPAUSE_ALL_ROLE, pauser.address]);
    await l1MessageService.write.grantRole([RATE_LIMIT_SETTER_ROLE, limitSetter.address]);
    await l1MessageService.write.grantRole([USED_RATE_LIMIT_RESETTER_ROLE, limitSetter.address]);

    await l1MessageServiceMerkleProof.write.grantRole([PAUSE_ALL_ROLE, pauser.address]);

    await l1MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });
    await l1MessageServiceMerkleProof.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });
  });

  describe("Initialisation tests", () => {
    it("Deployer has default admin role", async () => {
      await expectHasRole(l1MessageService, DEFAULT_ADMIN_ROLE, admin);
    });

    it("limitSetter has RATE_LIMIT_SETTER_ROLE", async () => {
      await expectHasRole(l1MessageService, RATE_LIMIT_SETTER_ROLE, limitSetter);
    });

    it("limitSetter has USED_RATE_LIMIT_RESETTER_ROLE", async () => {
      await expectHasRole(l1MessageService, USED_RATE_LIMIT_RESETTER_ROLE, limitSetter);
    });

    it("pauser has PAUSE_ALL_ROLE", async () => {
      await expectHasRole(l1MessageService, PAUSE_ALL_ROLE, pauser);
    });

    it("pauser has UNPAUSE_ALL_ROLE", async () => {
      await expectHasRole(l1MessageService, UNPAUSE_ALL_ROLE, pauser);
    });

    it("Should set rate limit and period", async () => {
      expect(await l1MessageService.read.periodInSeconds()).to.be.equal(BigInt(ONE_DAY_IN_SECONDS));
      expect(await l1MessageService.read.limitInWei()).to.be.equal(INITIAL_WITHDRAW_LIMIT);
    });

    it("It should fail when not initializing", async () => {
      await expectRevertWithReason(
        l1MessageService.write.tryInitialize([
          BigInt(ONE_DAY_IN_SECONDS),
          INITIAL_WITHDRAW_LIMIT,
          pauseTypeRoles,
          unpauseTypeRoles,
        ]),
        INITIALIZED_ERROR_MESSAGE,
      );
    });

    it("Should initialize nextMessageNumber", async () => {
      expect(await l1MessageService.read.nextMessageNumber()).to.be.equal(1n);
    });

    it("Should fail to deploy missing amount", async () => {
      await expectRevertWithCustomError(
        l1MessageService,
        deployUpgradableFromFactory("TestL1MessageService", [ONE_DAY_IN_SECONDS, 0, pauseTypeRoles, unpauseTypeRoles], {
          unsafeAllow: ["incorrect-initializer-order"],
        }),
        "LimitIsZero",
      );
    });

    it("Should fail to deploy missing limit period", async () => {
      await expectRevertWithCustomError(
        l1MessageService,
        deployUpgradableFromFactory(
          "TestL1MessageService",
          [0, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles],
          { unsafeAllow: ["incorrect-initializer-order"] },
        ),
        "PeriodIsZero",
      );
    });
  });

  describe("Send messages", () => {
    it("Should fail when the fee is higher than the amount sent", async () => {
      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
        { account: admin.account, value: MESSAGE_FEE - 1n },
      );

      await expectRevertWithCustomError(l1MessageService, sendMessageCall, "ValueSentTooLow");
    });

    it("Should fail when the to address is address 0", async () => {
      const sendMessageCall = l1MessageService.write.canSendMessage([ADDRESS_ZERO, MESSAGE_FEE, EMPTY_CALLDATA], {
        account: admin.account,
        value: MESSAGE_FEE,
      });

      await expectRevertWithCustomError(l1MessageService, sendMessageCall, "ZeroAddressNotAllowed");
    });

    it("Should send an ether only message with fees emitting the MessageSent event", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
        { account: admin.account, value: MESSAGE_FEE + MESSAGE_VALUE_1ETH },
      );
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", [
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
        messageHash,
      ]);
    });

    it("Should send max limit ether only message with no fee emitting the MessageSent event", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        0n,
        INITIAL_WITHDRAW_LIMIT,
        1n,
        EMPTY_CALLDATA,
      );
      const messageHash = keccak256(expectedBytes);

      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, 0n, EMPTY_CALLDATA],
        { account: admin.account, value: INITIAL_WITHDRAW_LIMIT },
      );
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", [
        l1MessageService.address,
        notAuthorizedAccount.address,
        0n,
        INITIAL_WITHDRAW_LIMIT,
        1n,
        EMPTY_CALLDATA,
        messageHash,
      ]);
    });

    // this is testing to allow even if claim is blocked
    it("Should send a message even when L2 to L1 communication is paused", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      await l1MessageService.write.pauseByType([L2_L1_PAUSE_TYPE], { account: pauser.account });

      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
        { account: admin.account, value: MESSAGE_FEE + MESSAGE_VALUE_1ETH },
      );
      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", [
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
        messageHash,
      ]);
    });

    it("Should update the rolling hash when sending a message post migration", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);
      const rollingHash = calculateRollingHash(INITIAL_ROLLING_HASH, messageHash);

      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
        { account: admin.account, value: MESSAGE_FEE + MESSAGE_VALUE_1ETH },
      );

      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", [
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      await expectRollingHashUpdatedEvent({
        contract: l1MessageService,
        updateCall: sendMessageCall,
        messageNumber: 1n,
        expectedRollingHash: rollingHash,
        messageHash,
      });

      await validateRollingHashStorage({
        contract: l1MessageService,
        messageNumber: 1n,
        expectedHash: rollingHash,
      });
      await validateRollingHashNotZero({ contract: l1MessageService, messageNumber: 1n });
    });

    it("Should use the previous existing rolling hash when sending a message post migration", async () => {
      let expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      let messageHash = keccak256(expectedBytes);
      let rollingHash = calculateRollingHash(INITIAL_ROLLING_HASH, messageHash);

      await l1MessageService.write.canSendMessage([notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA], {
        account: admin.account,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
      });

      expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2n,
        EMPTY_CALLDATA,
      );

      messageHash = keccak256(expectedBytes);

      rollingHash = calculateRollingHash(rollingHash, messageHash);

      const sendMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
        { account: admin.account, value: MESSAGE_FEE + MESSAGE_VALUE_1ETH },
      );

      await expectEvent(l1MessageService, sendMessageCall, "MessageSent", [
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2n,
        EMPTY_CALLDATA,
        messageHash,
      ]);
      await expectRollingHashUpdatedEvent({
        contract: l1MessageService,
        updateCall: sendMessageCall,
        messageNumber: 2n,
        expectedRollingHash: rollingHash,
        messageHash,
      });

      await validateRollingHashStorage({
        contract: l1MessageService,
        messageNumber: 2n,
        expectedHash: rollingHash,
      });
      await validateRollingHashNotZero({ contract: l1MessageService, messageNumber: 2n });
    });
  });

  describe("Claiming messages", () => {
    it("Should fail when the message hash does not exist", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      const claimMessageCall = l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1n,
      ]);

      await expectRevertWithCustomError(
        l1MessageService,
        claimMessageCall,
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });

    it("Should execute the claim message and send fees to recipient, left over fee to destination", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      await expectNotReverted(
        l1MessageService.write.claimMessage([
          l1MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]),
      );
    });

    it("Should claim message and send the fees when L1 to L2 communication is paused", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      // this is for sending only and should not affect claim
      await l1MessageService.write.pauseByType([L1_L2_PAUSE_TYPE], { account: pauser.account });

      await expectNotReverted(
        l1MessageService.write.claimMessage([
          l1MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]),
      );
    });

    it("Should execute the claim message and emit the MessageClaimed event", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      await l1MessageService.write.addL2L1MessageHash([messageHash]);

      const claimMessageCall = l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1n,
      ]);

      await expectEvent(l1MessageService, claimMessageCall, "MessageClaimed", [messageHash]);
    });

    it("Should fail when the message hash has been claimed", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      await l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1n,
      ]);

      const claimMessageCall = l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1n,
      ]);

      await expectRevertWithCustomError(
        l1MessageService,
        claimMessageCall,
        "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        [messageHash],
      );
    });

    it("Should execute the claim message and send the fees to msg.sender, left over fee to destination", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const expectedSecondBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        2n,
        EMPTY_CALLDATA,
      );

      const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);
      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedSecondBytes)]);

      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      const adminBalance = await publicClient.getBalance({ address: admin.address });

      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          2n,
        ],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.greaterThan(
        destinationBalance + MESSAGE_VALUE_1ETH + MESSAGE_VALUE_1ETH,
      );
      expect(await publicClient.getBalance({ address: admin.address })).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message and send the fees to msg.sender and NOT refund the destination", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        LOW_NO_REFUND_MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );
      const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          LOW_NO_REFUND_MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account, gasPrice: 1000000000n },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
        destinationBalance + MESSAGE_VALUE_1ETH,
      );

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message and send fees to recipient contract and no refund sent", async () => {
      const testContract = await deployFromFactory("TestReceivingContract");

      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        testContract.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const adminBalance = await publicClient.getBalance({ address: admin.address });
      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          testContract.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: testContract.address })).to.be.equal(MESSAGE_VALUE_1ETH);
      expect(await publicClient.getBalance({ address: admin.address })).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message and send fees to EOA with calldata and no refund sent", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        "0x123456789a",
      );

      const startingBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const adminBalance = await publicClient.getBalance({ address: admin.address });
      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          "0x123456789a",
          1n,
        ],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await publicClient.getBalance({ address: admin.address })).to.be.greaterThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message and no fees to EOA with calldata and no refund sent", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        "0x12",
      );

      const startingBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const adminBalance = await publicClient.getBalance({ address: admin.address });
      await l1MessageService.write.claimMessage(
        [l1MessageService.address, notAuthorizedAccount.address, 0n, MESSAGE_VALUE_1ETH, ADDRESS_ZERO, "0x12", 1n],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await publicClient.getBalance({ address: admin.address })).to.be.lessThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message and no fees to EOA with empty calldata and no refund sent", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const startingBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const adminBalance = await publicClient.getBalance({ address: admin.address });
      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
        startingBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await publicClient.getBalance({ address: admin.address })).to.be.lessThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should execute the claim message when there are no fees", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        notAuthorizedAccount.address,
        0n,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );
      const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const adminBalance = await publicClient.getBalance({ address: admin.address });
      await l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
        destinationBalance + MESSAGE_VALUE_1ETH,
      );
      expect(await publicClient.getBalance({ address: admin.address })).to.be.lessThan(adminBalance);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_UNKNOWN),
      );
    });

    it("Should provide the correct origin sender", async () => {
      const claimingCaller = await deployFromFactory("TestClaimingCaller", [l2Sender.address]);

      const expectedBytes = encodeSendMessage(
        l2Sender.address,
        claimingCaller.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const storedSenderBeforeSending = await l1MessageService.read.originalSender();
      expect(storedSenderBeforeSending).to.be.equal(ADDRESS_ZERO);

      await expectNotReverted(
        l1MessageService.write.claimMessage(
          [l2Sender.address, claimingCaller.address, MESSAGE_FEE, MESSAGE_VALUE_1ETH, ADDRESS_ZERO, EMPTY_CALLDATA, 1n],
          { account: admin.account },
        ),
      );
    });

    it("Should allow sending post claiming a message", async () => {
      const sendCalldata = slice(generateKeccak256Hash("sendNewMessage()"), 0, 4);

      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        l1MessageService.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        sendCalldata,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      await expectNotReverted(
        l1MessageService.write.claimMessage(
          [
            l1MessageService.address,
            l1MessageService.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            sendCalldata,
            1n,
          ],
          { account: admin.account },
        ),
      );
    });

    it("Should fail on reentry when sending to recipient", async () => {
      const callSignature = slice(generateKeccak256Hash("doReentry()"), 0, 4);

      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        l1MessageService.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        callSignature,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const claimMessageCall = l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          l1MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          callSignature,
          1n,
        ],
        { account: admin.account },
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "ReentrantCall");
    });

    it("Should fail when the destination errors", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        l1MessageService.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);
      await expectRevertWithCustomError(
        l1MessageService,
        l1MessageService.write.claimMessage(
          [
            l1MessageService.address,
            l1MessageService.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            EMPTY_CALLDATA,
            1n,
          ],
          { account: admin.account },
        ),
        "MessageSendingFailed",
        [l1MessageService.address],
      );

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_RECEIVED),
      );
    });

    it("Should fail when the fee recipient fails errors", async () => {
      const expectedBytes = encodeSendMessage(
        l1MessageService.address,
        admin.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

      const claimMessageCall = l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          admin.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          l1MessageService.address,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "FeePaymentFailed", [
        l1MessageService.address,
      ]);

      expect(await l1MessageService.read.inboxL2L1MessageStatus([keccak256(expectedBytes)])).to.be.equal(
        BigInt(INBOX_STATUS_RECEIVED),
      );
    });

    it("Should revert with send over max limit amount only", async () => {
      await setHash(0n, INITIAL_WITHDRAW_LIMIT + 1n);

      const claimMessageCall = l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          0n,
          INITIAL_WITHDRAW_LIMIT + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });

    it("Should revert with send over max limit amount and fees", async () => {
      await setHash(1n, INITIAL_WITHDRAW_LIMIT + 1n);

      const claimMessageCall = l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          1n,
          INITIAL_WITHDRAW_LIMIT + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });

    it("Should revert with send over max limit amount and fees - multi tx", async () => {
      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      await setHash(1n, INITIAL_WITHDRAW_LIMIT - MESSAGE_VALUE_1ETH - MESSAGE_FEE + 1n);

      const claimMessageCall = l1MessageService.write.claimMessage(
        [
          l1MessageService.address,
          notAuthorizedAccount.address,
          1n,
          INITIAL_WITHDRAW_LIMIT - MESSAGE_VALUE_1ETH - MESSAGE_FEE + 1n,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ],
        { account: admin.account },
      );

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "RateLimitExceeded");
    });
  });

  describe("Claim Message with Proof", () => {
    it("Should be able to claim a message that was sent", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      await expectNotReverted(
        l1MessageServiceMerkleProof.write.claimMessageWithProof([
          {
            proof: VALID_PROOF.proof,
            messageNumber: 1n,
            leafIndex: VALID_PROOF.index,
            from: admin.address,
            to: admin.address,
            fee: MESSAGE_FEE,
            value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
            feeRecipient: ADDRESS_ZERO,
            merkleRoot: VALID_PROOF.merkleRoot,
            data: EMPTY_CALLDATA,
          },
        ]),
      );
    });

    it("Should be able to claim a message and emit a MessageClaimed event", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      const messageLeafHash = keccak256(
        encodeAbiParameters(parseAbiParameters("address, address, uint256, uint256, uint256, bytes"), [
          admin.address,
          admin.address,
          MESSAGE_FEE,
          MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        ]),
      );

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectEvent(l1MessageServiceMerkleProof, claimMessageCall, "MessageClaimed", [messageLeafHash]);
    });

    it("Should fail to claim when the contract is generally paused", async () => {
      await l1MessageServiceMerkleProof.write.pauseByType([GENERAL_PAUSE_TYPE], { account: pauser.account });

      await expectRevertWhenPaused(
        l1MessageService,
        l1MessageServiceMerkleProof.write.claimMessageWithProof([
          {
            proof: VALID_PROOF.proof,
            messageNumber: 1n,
            leafIndex: VALID_PROOF.index,
            from: admin.address,
            to: admin.address,
            fee: MESSAGE_FEE,
            value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
            feeRecipient: ADDRESS_ZERO,
            merkleRoot: VALID_PROOF.merkleRoot,
            data: EMPTY_CALLDATA,
          },
        ]),
        GENERAL_PAUSE_TYPE,
      );
    });

    it("Should fail when the message has already been claimed", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      await l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageCall, "MessageAlreadyClaimed", [1n]);
    });

    it("Should fail when l2 Merkle root does not exist on L1", async () => {
      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "L2MerkleRootDoesNotExist");
    });

    it("Should fail when l2 merkle proof is invalid", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: INVALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "InvalidMerkleProof");
    });

    it("Should fail claiming when the call transaction fails with receive()", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [INVALID_PROOF_REVERT.merkleRoot],
        BigInt(INVALID_PROOF_REVERT.proof.length),
      ]);

      await expectReverted(
        l1MessageServiceMerkleProof.write.claimMessageWithProof([
          {
            proof: INVALID_PROOF_REVERT.proof,
            messageNumber: 1n,
            leafIndex: INVALID_PROOF_REVERT.index,
            from: admin.address,
            to: l1TestRevert.address,
            fee: MESSAGE_FEE,
            value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
            feeRecipient: ADDRESS_ZERO,
            merkleRoot: INVALID_PROOF_REVERT.merkleRoot,
            data: "0xcd4aed30",
          },
        ]),
      );
    });

    // TODO MAKE THIS DYNAMIC BECAUSE THE setNonce makes the testing brittle
    it("Should fail claiming when the call transaction fails with empty fallback", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [PROOF_FALLBACK.merkleRoot],
        BigInt(PROOF_FALLBACK.proof.length),
      ]);

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: PROOF_FALLBACK.proof,
          messageNumber: 1n,
          leafIndex: PROOF_FALLBACK.index,
          from: admin.address,
          to: l1TestRevert.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: PROOF_FALLBACK.merkleRoot,
          data: "0xce398a64",
        },
      ]);

      await expectRevertWithCustomError(l1MessageService, claimMessageCall, "MessageSendingFailed", [
        l1TestRevert.address,
      ]);
    });

    // TODO MAKE THIS DYNAMIC BECAUSE THE setNonce makes the testing brittle
    it("Should fail on reentry", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [PROOF_REENTRY.merkleRoot],
        BigInt(PROOF_REENTRY.proof.length),
      ]);

      const claimMessageWithProof = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: PROOF_REENTRY.proof,
          messageNumber: 1n,
          leafIndex: PROOF_REENTRY.index,
          from: admin.address,
          to: l1MessageServiceMerkleProof.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: PROOF_REENTRY.merkleRoot,
          data: "0xaf5696840000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000014000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266000000000000000000000000dc64a140aa3e981100a9beca4e685f962f0cf6c900000000000000000000000000000000000000000000000000b1a2bc2ec500000000000000000000000000000000000000000000000000000e92596fd62900000000000000000000000000000000000000000000000000000000000000000000c817003bf40005bdd4b6e06fd0ed2d01c27a89a4cf6ee67ea585489af54e4a8c000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        },
      ]);

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageWithProof, "ReentrantCall");
    });

    it("Should fail when the fee recipient fails errors", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof,
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: l1MessageService.address,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectRevertWithCustomError(l1MessageServiceMerkleProof, claimMessageCall, "FeePaymentFailed", [
        l1MessageService.address,
      ]);
    });

    it("Should fail when the merkle depth is different than the proof length", async () => {
      await l1MessageServiceMerkleProof.write.addL2MerkleRoots([
        [VALID_PROOF.merkleRoot],
        BigInt(VALID_PROOF.proof.length),
      ]);

      const merkleDepth = await l1MessageServiceMerkleProof.read.l2MerkleRootsDepths([VALID_PROOF.merkleRoot]);

      const claimMessageCall = l1MessageServiceMerkleProof.write.claimMessageWithProof([
        {
          proof: VALID_PROOF.proof.slice(0, -1),
          messageNumber: 1n,
          leafIndex: VALID_PROOF.index,
          from: admin.address,
          to: admin.address,
          fee: MESSAGE_FEE,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
          feeRecipient: l1MessageService.address,
          merkleRoot: VALID_PROOF.merkleRoot,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectRevertWithCustomError(
        l1MessageServiceMerkleProof,
        claimMessageCall,
        "ProofLengthDifferentThanMerkleDepth",
        [merkleDepth, BigInt(VALID_PROOF.proof.slice(0, -1).length)],
      );
    });
  });

  describe("Resetting limits", () => {
    it("Should reset limits as limitSetter", async () => {
      let usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);

      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);

      await l1MessageService.write.resetAmountUsedInPeriod({ account: limitSetter.account });
      usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);
    });

    it("Should fail reset limits as non-RATE_LIMIT_SETTER_ROLE", async () => {
      let usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);

      await setHashAndClaimMessage(MESSAGE_FEE, MESSAGE_VALUE_1ETH);

      usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);

      await expectRevertWithReason(
        l1MessageService.write.resetAmountUsedInPeriod({ account: admin.account }),
        buildAccessErrorMessage(admin, USED_RATE_LIMIT_RESETTER_ROLE),
      );

      usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(MESSAGE_FEE + MESSAGE_VALUE_1ETH);
    });
  });

  describe("Pausing contracts", () => {
    it("Should fail general pausing as non-pauser", async () => {
      await expectNotPaused(l1MessageService, GENERAL_PAUSE_TYPE);

      await expectRevertWithReason(
        l1MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: admin.account }),
        buildAccessErrorMessage(admin, PAUSE_ALL_ROLE),
      );

      await expectNotPaused(l1MessageService, GENERAL_PAUSE_TYPE);
    });

    it("Should pause generally as pause manager", async () => {
      await expectNotPaused(l1MessageService, GENERAL_PAUSE_TYPE);

      await l1MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: pauser.account });

      await expectPaused(l1MessageService, GENERAL_PAUSE_TYPE);
    });

    it("Should fail when to claim the contract is generally paused", async () => {
      await l1MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: pauser.account });

      const claimMessageCall = l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1n,
      ]);

      await expectRevertWhenPaused(l1MessageService, claimMessageCall, GENERAL_PAUSE_TYPE);
    });

    it("Should fail to claim when the L2 to L1 communication is paused", async () => {
      await l1MessageService.write.pauseByType([L2_L1_PAUSE_TYPE], { account: pauser.account });

      const claimMessageCall = l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        EMPTY_CALLDATA,
        1n,
      ]);

      await expectRevertWhenPaused(l1MessageService, claimMessageCall, L2_L1_PAUSE_TYPE);
    });

    it("Should fail to send if the contract is generally paused", async () => {
      await l1MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: pauser.account });

      const claimMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, 0n, EMPTY_CALLDATA],
        { account: admin.account, value: INITIAL_WITHDRAW_LIMIT },
      );

      await expectRevertWhenPaused(l1MessageService, claimMessageCall, GENERAL_PAUSE_TYPE);

      const usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);
    });

    it("Should fail to send if L1 to L2 communication is paused", async () => {
      await l1MessageService.write.pauseByType([L1_L2_PAUSE_TYPE], { account: pauser.account });

      const claimMessageCall = l1MessageService.write.canSendMessage(
        [notAuthorizedAccount.address, 0n, EMPTY_CALLDATA],
        { account: admin.account, value: INITIAL_WITHDRAW_LIMIT },
      );

      await expectRevertWhenPaused(l1MessageService, claimMessageCall, L1_L2_PAUSE_TYPE);

      const usedAmount = await l1MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);
    });
  });

  async function setHash(fee: bigint, value: bigint) {
    const expectedBytes = encodeSendMessage(
      l1MessageService.address,
      notAuthorizedAccount.address,
      fee,
      value,
      1n,
      EMPTY_CALLDATA,
    );

    await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);
  }

  async function setHashAndClaimMessage(fee: bigint, value: bigint) {
    const expectedBytes = encodeSendMessage(
      l1MessageService.address,
      notAuthorizedAccount.address,
      fee,
      value,
      1n,
      EMPTY_CALLDATA,
    );

    await l1MessageService.write.addL2L1MessageHash([keccak256(expectedBytes)]);

    await expectNotReverted(
      l1MessageService.write.claimMessage([
        l1MessageService.address,
        notAuthorizedAccount.address,
        fee,
        value,
        postmanAddress.address,
        EMPTY_CALLDATA,
        1n,
      ]),
    );
  }
});
