import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import {
  type Address,
  type Hash,
  type Hex,
  BaseError,
  keccak256,
  pad,
  parseEther,
  slice,
  stringToHex,
  zeroHash,
} from "viem";

import {
  L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
  L2_MESSAGE_SERVICE_ROLES,
  L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
} from "../../../../common/constants/index.js";
import { encodeSendMessage } from "../../../../common/helpers/encoding.js";
import { generateRoleAssignments } from "../../../../common/helpers/index.js";
import {
  ADDRESS_ZERO,
  BLOCK_COINBASE,
  DEFAULT_ADMIN_ROLE,
  EMPTY_CALLDATA,
  GENERAL_PAUSE_TYPE,
  INBOX_STATUS_CLAIMED,
  INBOX_STATUS_RECEIVED,
  INITIAL_WITHDRAW_LIMIT,
  L1_L2_MESSAGE_SETTER_ROLE,
  L1_L2_PAUSE_TYPE,
  L2_L1_PAUSE_TYPE,
  LOW_NO_REFUND_MESSAGE_FEE,
  MESSAGE_FEE,
  MESSAGE_VALUE_1ETH,
  MINIMUM_FEE,
  MINIMUM_FEE_SETTER_ROLE,
  ONE_DAY_IN_SECONDS,
  PAUSE_ALL_ROLE,
  UNPAUSE_ALL_ROLE,
  RATE_LIMIT_SETTER_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
} from "../../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory, getUpgrades } from "../../common/deployment.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  calculateRollingHash,
  calculateRollingHashFromCollection,
  expectEventDirectFromReceiptData,
  expectRevertWhenPaused,
  expectPaused,
  expectNotPaused,
  generateKeccak256Hash,
  getTestAccounts,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectEvent,
} from "../../common/helpers/index.js";

import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";

describe("L2MessageService", () => {
  let l2MessageService: ContractReturnType<"TestL2MessageService">;
  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let l1l2MessageSetter: TestAccount;
  let notAuthorizedAccount: TestAccount;
  let postmanAddress: TestAccount;
  let roleAddresses: { role: Hex; addressWithRole: Address }[];
  let publicClient: PublicClient;

  async function deployL2MessageServiceFixture() {
    return deployUpgradableFromFactory("TestL2MessageService", [
      ONE_DAY_IN_SECONDS,
      INITIAL_WITHDRAW_LIMIT,
      securityCouncil.address,
      roleAddresses,
      L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
      L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
    ]);
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

  /** Receipt of the transaction that deployed the proxy at `proxyAddress` (ethers' `deploymentTransaction().wait()`). */
  async function getProxyDeploymentReceipt(proxyAddress: Address) {
    const block = await publicClient.getBlock();
    for (const hash of block.transactions) {
      const receipt = await publicClient.getTransactionReceipt({ hash });
      if (receipt.contractAddress?.toLowerCase() === proxyAddress.toLowerCase()) {
        return receipt;
      }
    }
    throw new Error(`No deployment transaction found for ${proxyAddress} in the latest block`);
  }

  before(async () => {
    const { viem } = await network.getOrCreate();
    publicClient = await viem.getPublicClient();
    [admin, securityCouncil, l1l2MessageSetter, notAuthorizedAccount, postmanAddress] = await getTestAccounts();
    roleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, securityCouncil.address, [
      { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l1l2MessageSetter.address] },
    ]);
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    l2MessageService = await networkHelpers.loadFixture(deployL2MessageServiceFixture);
  });

  describe("Initialization checks", () => {
    it("Should set minimumFeeInWei to 0.0001 ETH", async () => {
      expect(await l2MessageService.read.minimumFeeInWei()).to.be.equal(parseEther("0.0001"));
    });

    it("Security council should have DEFAULT_ADMIN_ROLE", async () => {
      expect(await l2MessageService.read.hasRole([DEFAULT_ADMIN_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Security council should have MINIMUM_FEE_SETTER_ROLE", async () => {
      expect(await l2MessageService.read.hasRole([MINIMUM_FEE_SETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Security council should have RATE_LIMIT_SETTER_ROLE role", async () => {
      expect(await l2MessageService.read.hasRole([RATE_LIMIT_SETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Security council should have USED_RATE_LIMIT_RESETTER_ROLE role", async () => {
      expect(await l2MessageService.read.hasRole([USED_RATE_LIMIT_RESETTER_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Security council should have PAUSE_ALL_ROLE", async () => {
      expect(await l2MessageService.read.hasRole([PAUSE_ALL_ROLE, securityCouncil.address])).to.be.true;
    });

    it("Security council should have UNPAUSE_ALL_ROLE", async () => {
      expect(await l2MessageService.read.hasRole([UNPAUSE_ALL_ROLE, securityCouncil.address])).to.be.true;
    });

    it("L1->L2 message setter should have L1_L2_MESSAGE_SETTER_ROLE role", async () => {
      expect(await l2MessageService.read.hasRole([L1_L2_MESSAGE_SETTER_ROLE, l1l2MessageSetter.address])).to.be.true;
    });

    it("Should initialize nextMessageNumber", async () => {
      expect(await l2MessageService.read.nextMessageNumber()).to.be.equal(1n);
    });

    it("Should set rate limit and period", async () => {
      expect(await l2MessageService.read.periodInSeconds()).to.be.equal(BigInt(ONE_DAY_IN_SECONDS));
      expect(await l2MessageService.read.limitInWei()).to.be.equal(INITIAL_WITHDRAW_LIMIT);
    });

    it("Should have the correct contract version", async () => {
      const l2MessageServiceLocal = await deployUpgradableFromFactory("TestL2MessageService", [
        ONE_DAY_IN_SECONDS,
        INITIAL_WITHDRAW_LIMIT,
        securityCouncil.address,
        roleAddresses,
        L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
        L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
      ]);

      const receipt = await getProxyDeploymentReceipt(l2MessageServiceLocal.address);

      await expectEventDirectFromReceiptData(
        l2MessageServiceLocal,
        receipt,
        "MinimumFeeChanged",
        [0n, parseEther("0.0001"), admin.address],
        20,
      );

      await expectEventDirectFromReceiptData(
        l2MessageServiceLocal,
        receipt,
        "L2MessageServiceBaseInitialized",
        [pad(stringToHex("1.0"), { dir: "right", size: 8 })],
        21,
      );

      expect(await l2MessageService.read.CONTRACT_VERSION()).to.equal("1.0");
    });

    it("Should fail to deploy if default admin is address zero", async () => {
      const deployCall = deployUpgradableFromFactory("TestL2MessageService", [
        ONE_DAY_IN_SECONDS,
        MESSAGE_VALUE_1ETH + MESSAGE_VALUE_1ETH,
        ADDRESS_ZERO,
        roleAddresses,
        L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
        L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
      ]);

      await expectRevertWithCustomError(l2MessageService, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should fail to deploy missing limit amount", async () => {
      const deployCall = deployUpgradableFromFactory("TestL2MessageService", [
        ONE_DAY_IN_SECONDS,
        0,
        securityCouncil.address,
        roleAddresses,
        L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
        L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
      ]);

      await expectRevertWithCustomError(l2MessageService, deployCall, "LimitIsZero");
    });

    it("Should fail to deploy missing period", async () => {
      const deployCall = deployUpgradableFromFactory("TestL2MessageService", [
        0,
        MESSAGE_VALUE_1ETH + MESSAGE_VALUE_1ETH,
        securityCouncil.address,
        roleAddresses,
        L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
        L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
      ]);

      await expectRevertWithCustomError(l2MessageService, deployCall, "PeriodIsZero");
    });

    it("Should fail on second initialisation", async () => {
      const deployCall = l2MessageService.write.initialize([
        BigInt(ONE_DAY_IN_SECONDS),
        INITIAL_WITHDRAW_LIMIT,
        securityCouncil.address,
        roleAddresses,
        L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES,
        L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES,
      ]);

      await expectRevertWithCustomError(l2MessageService, deployCall, "InitializedVersionWrong", [0n, 3n]);
    });

    it.skip("Can upgrade existing contract", async () => {
      // Deploy V1 from artifact
      // Deploy V-next when we have it
    });
  });

  describe("Send message", () => {
    describe("When the contract is paused", () => {
      it("Should fail to send if the contract is paused", async () => {
        await l2MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
          { account: securityCouncil.account, value: INITIAL_WITHDRAW_LIMIT },
        );

        await expectRevertWhenPaused(l2MessageService, sendMessageCall, GENERAL_PAUSE_TYPE);
      });
    });

    describe("When the L2->L1 messaging service is paused", () => {
      it("Should fail to send if the L2->L1 messaging service is paused", async () => {
        await l2MessageService.write.pauseByType([L2_L1_PAUSE_TYPE], { account: securityCouncil.account });

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
          { account: securityCouncil.account, value: INITIAL_WITHDRAW_LIMIT },
        );

        await expectRevertWhenPaused(l2MessageService, sendMessageCall, L2_L1_PAUSE_TYPE);
      });
    });

    describe("When the contract is not paused", () => {
      it("Should fail when the fee is higher than the amount sent", async () => {
        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_FEE - parseEther("0.01") + parseEther("0.0001") },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "ValueSentTooLow");
      });

      it("Should fail when the coinbase fee transfer fails", async () => {
        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const { networkHelpers } = await network.getOrCreate();
        await networkHelpers.setCoinbase(l2MessageService.address);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE + MINIMUM_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MINIMUM_FEE + MINIMUM_FEE },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "FeePaymentFailed", [
          l2MessageService.address,
        ]);

        await networkHelpers.setCoinbase(BLOCK_COINBASE);
      });

      it("Should fail when the minimumFee is higher than the amount sent", async () => {
        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_FEE + parseEther("0.0001") },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "FeeTooLow");
      });

      it("Should fail when the to address is address 0", async () => {
        const sendMessageCall = l2MessageService.write.canSendMessage([ADDRESS_ZERO, MESSAGE_FEE, EMPTY_CALLDATA], {
          account: admin.account,
          value: MESSAGE_FEE,
        });

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "ZeroAddressNotAllowed");
      });

      it("Should increase the balance of the coinbase with the minimumFee", async () => {
        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const initialCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });

        await l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE + MINIMUM_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MINIMUM_FEE + MESSAGE_FEE + parseEther("0.0001") },
        );

        expect(await publicClient.getBalance({ address: BLOCK_COINBASE })).to.be.gt(
          initialCoinbaseBalance + MINIMUM_FEE,
        );
      });

      it("Should succeed if 'MinimumFeeChanged' event is emitted", async () => {
        const initialMinimumFee = parseEther("0.0001");

        await expectEvent(
          l2MessageService,
          l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account }),
          "MinimumFeeChanged",
          [initialMinimumFee, MINIMUM_FEE, securityCouncil.address],
        );

        // Testing non-zero transition
        await expectEvent(
          l2MessageService,
          l2MessageService.write.setMinimumFee([MINIMUM_FEE + 1n], { account: securityCouncil.account }),
          "MinimumFeeChanged",
          [MINIMUM_FEE, MINIMUM_FEE + 1n, securityCouncil.address],
        );
      });

      it("Should succeed if 'MessageSent' event is emitted", async () => {
        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const expectedBytes = encodeSendMessage(
          securityCouncil.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH - MESSAGE_FEE - MINIMUM_FEE,
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE + MINIMUM_FEE, EMPTY_CALLDATA],
          { account: securityCouncil.account, value: MESSAGE_VALUE_1ETH },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          securityCouncil.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH - MESSAGE_FEE - MINIMUM_FEE,
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);
      });

      it("Should send an ether only message with fees emitting the MessageSent event", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE - parseEther("0.0001"),
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_FEE + MESSAGE_VALUE_1ETH },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE - parseEther("0.0001"),
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);
      });

      it("Should send max limit ether only message with no fee emitting the MessageSent event", async () => {
        const expectedBytes = encodeSendMessage(
          securityCouncil.address,
          notAuthorizedAccount.address,
          0n,
          INITIAL_WITHDRAW_LIMIT - parseEther("0.0001"),
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA],
          { account: securityCouncil.account, value: INITIAL_WITHDRAW_LIMIT },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          securityCouncil.address,
          notAuthorizedAccount.address,
          0n,
          INITIAL_WITHDRAW_LIMIT - parseEther("0.0001"),
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);
      });

      it("Should revert with send over max limit amount only", async () => {
        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA],
          { account: admin.account, value: INITIAL_WITHDRAW_LIMIT + parseEther("0.0002") },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "RateLimitExceeded");
      });

      it("Should revert with send over max limit amount and fees", async () => {
        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA],
          { account: admin.account, value: INITIAL_WITHDRAW_LIMIT + parseEther("0.0002") },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "RateLimitExceeded");
      });

      it("Should fail when the rate limit would be exceeded - multi transactions", async () => {
        await l2MessageService.write.sendMessage([notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA], {
          account: admin.account,
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH + parseEther("0.0001"),
        });

        const breachingAmount = INITIAL_WITHDRAW_LIMIT - MESSAGE_FEE - MESSAGE_VALUE_1ETH + parseEther("0.0002");

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA],
          { account: admin.account, value: breachingAmount },
        );

        await expectRevertWithCustomError(l2MessageService, sendMessageCall, "RateLimitExceeded");
      });

      it("Should not accrue rate limit while sending transaction with coinbaseFee only", async () => {
        const initialCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });
        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const initialRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          0n,
          0n,
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MINIMUM_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MINIMUM_FEE },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          admin.address,
          notAuthorizedAccount.address,
          0n,
          0n,
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);

        const postCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });
        await expect(postCoinbaseBalance).to.be.gt(initialCoinbaseBalance);

        const postRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();
        await expect(postRateLimitUsed).to.be.equal(initialRateLimitUsed);
      });

      it("Should accrue rate limit while sending transaction with 0 value and real fee, postmanFee = fee - coinbaseFee", async () => {
        const initialCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });
        const initialRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_VALUE_1ETH - MINIMUM_FEE,
          0n,
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MESSAGE_VALUE_1ETH, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_VALUE_1ETH },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_VALUE_1ETH - MINIMUM_FEE,
          0n,
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);

        const postCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });

        const postRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        await expect(postCoinbaseBalance).to.be.gt(initialCoinbaseBalance);
        expect(await postRateLimitUsed).to.be.gt(initialRateLimitUsed);
      });

      it("Should accrue rate limit while sending transaction with value with real fee, postmanFee = fee - coinbaseFee", async () => {
        const initialCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });
        const initialRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MINIMUM_FEE + MESSAGE_FEE - MINIMUM_FEE,
          MESSAGE_VALUE_1ETH - (MINIMUM_FEE + MESSAGE_FEE),
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MINIMUM_FEE + MESSAGE_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_VALUE_1ETH },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          admin.address,
          notAuthorizedAccount.address,
          MINIMUM_FEE + MESSAGE_FEE - MINIMUM_FEE,
          MESSAGE_VALUE_1ETH - (MINIMUM_FEE + MESSAGE_FEE),
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);

        const postCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });

        const postRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        expect(postCoinbaseBalance).to.be.gt(initialCoinbaseBalance);
        expect(postRateLimitUsed).to.be.gt(initialRateLimitUsed);
      });

      it("Should accrue rate limit while sending transaction with value with coinbaseFee, postmanFee = 0", async () => {
        const initialCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });
        const initialRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH - MINIMUM_FEE,
          1n,
          EMPTY_CALLDATA,
        );
        const messageHash = keccak256(expectedBytes);

        const sendMessageCall = l2MessageService.write.sendMessage(
          [notAuthorizedAccount.address, MINIMUM_FEE, EMPTY_CALLDATA],
          { account: admin.account, value: MESSAGE_VALUE_1ETH },
        );

        await expectEvent(l2MessageService, sendMessageCall, "MessageSent", [
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH - MINIMUM_FEE,
          1n,
          EMPTY_CALLDATA,
          messageHash,
        ]);

        const postCoinbaseBalance = await publicClient.getBalance({ address: BLOCK_COINBASE });

        const postRateLimitUsed = await l2MessageService.read.currentPeriodAmountInWei();

        expect(postCoinbaseBalance).to.be.gt(initialCoinbaseBalance);
        expect(postRateLimitUsed).to.be.gt(initialRateLimitUsed);
      });
    });
  });

  describe("Claim message", () => {
    describe("When the contract is paused", async () => {
      it("Should revert if the contract is paused", async () => {
        await l2MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

        const claimMessageCall = l2MessageService.write.claimMessage(
          [
            securityCouncil.address,
            notAuthorizedAccount.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            notAuthorizedAccount.address,
            EMPTY_CALLDATA,
            1n,
          ],
          { account: securityCouncil.account },
        );

        await expectRevertWhenPaused(l2MessageService, claimMessageCall, GENERAL_PAUSE_TYPE);
      });
    });

    describe("When L1->L2 messaging service is paused", async () => {
      it("Should revert if the L1->L2 messaging service is paused", async () => {
        await l2MessageService.write.pauseByType([L1_L2_PAUSE_TYPE], { account: securityCouncil.account });

        const claimMessageCall = l2MessageService.write.claimMessage(
          [
            securityCouncil.address,
            notAuthorizedAccount.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            notAuthorizedAccount.address,
            EMPTY_CALLDATA,
            1n,
          ],
          { account: securityCouncil.account },
        );

        await expectRevertWhenPaused(l2MessageService, claimMessageCall, L1_L2_PAUSE_TYPE);
      });
    });

    describe("When the contract is not paused", () => {
      it("Should succeed if 'MessageClaimed' event is emitted", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const sendMessageCall = l2MessageService.write.claimMessage([
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]);

        await expectEvent(l2MessageService, sendMessageCall, "MessageClaimed", [keccak256(expectedBytes)]);
      });

      it("Should fail when the message hash does not exist", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const messageHash = keccak256(expectedBytes);

        const claimMessageCall = l2MessageService.write.claimMessage([
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          ADDRESS_ZERO,
          EMPTY_CALLDATA,
          1n,
        ]);

        await expectRevertWithCustomError(
          l2MessageService,
          claimMessageCall,
          "MessageDoesNotExistOrHasAlreadyBeenClaimed",
          [messageHash],
        );
      });

      it("Should execute the claim message and send fees to recipient, left over fee to destination", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });
        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]);
        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.greaterThan(
          destinationBalance + MESSAGE_VALUE_1ETH,
        );
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.greaterThan(postmanBalance);
      });

      it("Should execute the claim message and send fees to recipient contract and no leftovers", async () => {
        const testContract = await deployFromFactory("TestReceivingContract");

        const expectedBytes = encodeSendMessage(
          admin.address,
          testContract.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          admin.address,
          testContract.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]);
        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: testContract.address })).to.be.equal(MESSAGE_VALUE_1ETH);
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.equal(
          postmanBalance + MESSAGE_FEE,
        );
      });

      it("Should execute the claim message and send the fees to set recipient, and NOT refund fee to EOA", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          LOW_NO_REFUND_MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });
        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage(
          [
            admin.address,
            notAuthorizedAccount.address,
            LOW_NO_REFUND_MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            postmanAddress.address,
            EMPTY_CALLDATA,
            1n,
          ],
          { gasPrice: 1000000000n },
        );

        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
          destinationBalance + MESSAGE_VALUE_1ETH,
        );
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.equal(
          postmanBalance + LOW_NO_REFUND_MESSAGE_FEE,
        );
      });

      it("Should execute the claim message and send fees to EOA with calldata and no refund sent", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          "0x123456789a",
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });
        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          admin.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          "0x123456789a",
          1n,
        ]);
        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
          destinationBalance + MESSAGE_VALUE_1ETH,
        );
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.equal(
          postmanBalance + MESSAGE_FEE,
        );
      });

      it("Should execute the claim message and no fees to EOA with calldata and no refund sent", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          1n,
          "0x123456789a",
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });
        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          "0x123456789a",
          1n,
        ]);
        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
          destinationBalance + MESSAGE_VALUE_1ETH,
        );
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.equal(postmanBalance);
      });

      it("Should execute the claim message and no fees to EOA with no calldata and no refund sent", async () => {
        const expectedBytes = encodeSendMessage(
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });
        const postmanBalance = await publicClient.getBalance({ address: postmanAddress.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          admin.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]);
        // greater due to the gas refund
        expect(await publicClient.getBalance({ address: notAuthorizedAccount.address })).to.be.equal(
          destinationBalance + MESSAGE_VALUE_1ETH,
        );
        expect(await publicClient.getBalance({ address: postmanAddress.address })).to.be.equal(postmanBalance);
      });

      // todo - add tests for refund checks when gas is lower

      it("Should fail to send if the contract is paused", async () => {
        await l2MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

        const sendMessageCall = l2MessageService.write.canSendMessage(
          [notAuthorizedAccount.address, 0n, EMPTY_CALLDATA],
          { account: admin.account, value: INITIAL_WITHDRAW_LIMIT },
        );

        await expectRevertWhenPaused(l2MessageService, sendMessageCall, GENERAL_PAUSE_TYPE);

        const usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
        expect(usedAmount).to.be.equal(0n);
      });

      it("Should fail when the message hash has been claimed", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage([
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          postmanAddress.address,
          EMPTY_CALLDATA,
          1n,
        ]);
        await expectRevertWithCustomError(
          l2MessageService,
          l2MessageService.write.claimMessage([
            l2MessageService.address,
            notAuthorizedAccount.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            postmanAddress.address,
            EMPTY_CALLDATA,
            1n,
          ]),
          "MessageDoesNotExistOrHasAlreadyBeenClaimed",
        );
      });

      it("Should execute the claim message and send the fees to msg.sender", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        const expectedSecondBytes = encodeSendMessage(
          l2MessageService.address,
          notAuthorizedAccount.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          2n,
          EMPTY_CALLDATA,
        );

        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes), keccak256(expectedSecondBytes)];
        const expectedRollingHash = calculateRollingHashFromCollection(zeroHash, expectedBytesArray);

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 3n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
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

        await l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
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
        expect(await publicClient.getBalance({ address: admin.address })).to.be.lessThan(adminBalance + MESSAGE_FEE);

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_CLAIMED),
        );
      });

      // todo also add lower than 5000 gas check for the balances to be equal

      it("Should execute the claim message when there are no fees", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          notAuthorizedAccount.address,
          0n,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );
        const destinationBalance = await publicClient.getBalance({ address: notAuthorizedAccount.address });

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const adminBalance = await publicClient.getBalance({ address: admin.address });
        await l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
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

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_CLAIMED),
        );
      });

      it("Should provide the correct origin sender", async () => {
        const sendCalldata = slice(generateKeccak256Hash("setSender()"), 0, 4);

        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          l2MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          sendCalldata,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const storedSenderBeforeSending = await l2MessageService.read.originalSender();
        expect(storedSenderBeforeSending).to.be.equal(ADDRESS_ZERO);

        await expectNotReverted(
          l2MessageService.write.claimMessage(
            [
              l2MessageService.address,
              l2MessageService.address,
              MESSAGE_FEE,
              MESSAGE_VALUE_1ETH,
              ADDRESS_ZERO,
              sendCalldata,
              1n,
            ],
            { account: admin.account },
          ),
        );

        const newSender = await l2MessageService.read.originalSender();
        expect(newSender).to.be.equal(l2MessageService.address);
      });

      it("Should fail on reentry when sending to recipient", async () => {
        const callSignature = slice(generateKeccak256Hash("doReentry()"), 0, 4);

        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          l2MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          callSignature,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const claimMessageCall = l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
            l2MessageService.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            callSignature,
            1n,
          ],
          { account: admin.account },
        );

        await expectRevertWithCustomError(l2MessageService, claimMessageCall, "ReentrantCall");
      });

      it("Should fail when the destination errors through receive", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          l2MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await expectReverted(
          l2MessageService.write.claimMessage(
            [
              l2MessageService.address,
              l2MessageService.address,
              MESSAGE_FEE,
              MESSAGE_VALUE_1ETH,
              ADDRESS_ZERO,
              EMPTY_CALLDATA,
              1n,
            ],
            { account: admin.account },
          ),
        );

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_RECEIVED),
        );
      });

      it("Should fail when the destination errors through fallback", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          l2MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          "0x1234",
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        await expectReverted(
          l2MessageService.write.claimMessage(
            [
              l2MessageService.address,
              l2MessageService.address,
              MESSAGE_FEE,
              MESSAGE_VALUE_1ETH,
              ADDRESS_ZERO,
              "0x1234",
              1n,
            ],
            { account: admin.account },
          ),
        );

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_RECEIVED),
        );
      });

      it("Should fail when the destination errors on empty receive (makeItReceive function)", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          l2MessageService.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          "0xfc13b6f3",
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const claimMessageCall = l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
            l2MessageService.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            ADDRESS_ZERO,
            "0xfc13b6f3",
            1n,
          ],
          { account: admin.account },
        );

        await expectRevertWithCustomError(l2MessageService, claimMessageCall, "MessageSendingFailed", [
          l2MessageService.address,
        ]);

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_RECEIVED),
        );
      });

      it("Should fail when the fee recipient fails errors", async () => {
        const expectedBytes = encodeSendMessage(
          l2MessageService.address,
          admin.address,
          MESSAGE_FEE,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        );

        await l2MessageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT });

        const expectedBytesArray = [keccak256(expectedBytes)];
        const expectedRollingHash = calculateRollingHash(zeroHash, keccak256(expectedBytes));

        await l2MessageService.write.setLastAnchoredL1MessageNumber([1n]);
        await l2MessageService.write.anchorL1L2MessageHashes([expectedBytesArray, 2n, 2n, expectedRollingHash], {
          account: l1l2MessageSetter.account,
        });

        const claimMessageCall = l2MessageService.write.claimMessage(
          [
            l2MessageService.address,
            admin.address,
            MESSAGE_FEE,
            MESSAGE_VALUE_1ETH,
            l2MessageService.address,
            EMPTY_CALLDATA,
            1n,
          ],
          { account: admin.account },
        );

        await expectRevertWithCustomError(l2MessageService, claimMessageCall, "FeePaymentFailed", [
          l2MessageService.address,
        ]);

        expect(await l2MessageService.read.inboxL1L2MessageStatus([keccak256(expectedBytes)])).to.be.equal(
          BigInt(INBOX_STATUS_RECEIVED),
        );
      });
    });
  });

  describe("Set minimum fee", () => {
    it("Should fail when caller is not allowed", async () => {
      await expectRevertWithReason(
        l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: notAuthorizedAccount.account }),
        buildAccessErrorMessage(notAuthorizedAccount, MINIMUM_FEE_SETTER_ROLE),
      );
    });

    it("Should set the minimum fee", async () => {
      await l2MessageService.write.setMinimumFee([MINIMUM_FEE], { account: securityCouncil.account });

      expect(await l2MessageService.read.minimumFeeInWei()).to.be.equal(MINIMUM_FEE);
    });
  });

  describe("Pausing contracts", () => {
    it("Should fail pausing as non-pauser", async () => {
      await expectNotPaused(l2MessageService, GENERAL_PAUSE_TYPE);

      await expectRevertWithReason(
        l2MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: admin.account }),
        buildAccessErrorMessage(admin, PAUSE_ALL_ROLE),
      );

      await expectNotPaused(l2MessageService, GENERAL_PAUSE_TYPE);
    });

    it("Should pause as pause manager", async () => {
      await expectNotPaused(l2MessageService, GENERAL_PAUSE_TYPE);

      await l2MessageService.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      await expectPaused(l2MessageService, GENERAL_PAUSE_TYPE);
    });
  });

  describe("Resetting limits", () => {
    it("Should reset limits as limitSetter", async () => {
      let usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);

      await l2MessageService.write.sendMessage([notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA], {
        account: admin.account,
        value: INITIAL_WITHDRAW_LIMIT + parseEther("0.0001"),
      });

      usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(INITIAL_WITHDRAW_LIMIT);

      await l2MessageService.write.resetAmountUsedInPeriod({ account: securityCouncil.account });
      usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);
    });

    it("Should fail reset limits as non-pauser", async () => {
      let usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(0n);

      await l2MessageService.write.sendMessage([notAuthorizedAccount.address, parseEther("0.0001"), EMPTY_CALLDATA], {
        account: admin.account,
        value: INITIAL_WITHDRAW_LIMIT + parseEther("0.0001"),
      });

      usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(INITIAL_WITHDRAW_LIMIT);

      await expectRevertWithReason(
        l2MessageService.write.resetAmountUsedInPeriod({ account: admin.account }),
        buildAccessErrorMessage(admin, USED_RATE_LIMIT_RESETTER_ROLE),
      );

      usedAmount = await l2MessageService.read.currentPeriodAmountInWei();
      expect(usedAmount).to.be.equal(INITIAL_WITHDRAW_LIMIT);
    });
  });

  describe("L2MessageService Upgradeable Tests", () => {
    it("Should set initialized version to 3 on fresh deploy", async function () {
      const testL2MessageService = await deployL2MessageServiceFixture();

      const slotValue = await testL2MessageService.read.getSlotValue([0n]);
      expect(slotValue).equal(3n);
    });

    it("Should deploy and manually upgrade the L2MessageService contract", async function () {
      const testL2MessageService = await deployL2MessageServiceFixture();

      // Simulate a pre-upgrade state by lowering the initialized version
      await testL2MessageService.write.setSlotValue([0n, 2n]);

      let slotValue = await testL2MessageService.read.getSlotValue([177n]);
      expect(slotValue).equal(0n);
      await testL2MessageService.write.setSlotValue([177n, 1n]);

      // simulating reentry value at slot 177
      slotValue = await testL2MessageService.read.getSlotValue([177n]);
      expect(slotValue).equal(1n);

      // Deploy new L2MessageService implementation
      const upgrades = await getUpgrades();
      await upgrades.upgradeProxy(
        testL2MessageService,
        "src/_testing/unit/messaging/TestL2MessageService.sol:TestL2MessageService",
        {
          call: { fn: "reinitializeV3" },
          kind: "transparent",
          unsafeAllowRenames: true,
          unsafeAllow: ["incorrect-initializer-order"],
        },
      );

      // reentry slot cleared
      slotValue = await testL2MessageService.read.getSlotValue([177n]);
      expect(slotValue).equal(0n);

      // version changed
      slotValue = await testL2MessageService.read.getSlotValue([0n]);
      expect(slotValue).equal(3n);
    });

    it("Should revert reinitializeV3 if old reentrancy guard slot indicates ENTERED", async function () {
      const testL2MessageService = await deployL2MessageServiceFixture();

      // Simulate a pre-upgrade state by lowering the initialized version
      await testL2MessageService.write.setSlotValue([0n, 2n]);

      // Set legacy reentrancy guard slot 1 to OZ ENTERED value (2) — the assembly checks sload(1)
      await testL2MessageService.write.setSlotValue([177n, 2n]);

      const upgrades = await getUpgrades();

      await expectRevertWithCustomError(
        testL2MessageService,
        upgrades.upgradeProxy(
          testL2MessageService,
          "src/_testing/unit/messaging/TestL2MessageService.sol:TestL2MessageService",
          {
            call: { fn: "reinitializeV3" },
            kind: "transparent",
            unsafeAllowRenames: true,
            unsafeAllow: ["incorrect-initializer-order"],
          },
        ),
        "ReentrantCall",
      );
    });
  });
});
