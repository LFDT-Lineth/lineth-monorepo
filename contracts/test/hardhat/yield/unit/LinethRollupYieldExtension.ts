// TODO rename to LinethRollupYieldExtension
import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, type Hex, keccak256, parseEther, toHex, zeroHash } from "viem";

import { encodeData, encodeSendMessage } from "../../../../common/helpers/encoding.js";
import {
  ADDRESS_ZERO,
  EMPTY_CALLDATA,
  GENERAL_PAUSE_TYPE,
  L1_L2_PAUSE_TYPE,
  MESSAGE_FEE,
  MESSAGE_VALUE_1ETH,
  NATIVE_YIELD_STAKING_PAUSE_TYPE,
  YIELD_PROVIDER_STAKING_ROLE,
  SET_YIELD_MANAGER_ROLE,
  VALID_MERKLE_PROOF,
  ZERO_VALUE,
} from "../../common/constants/index.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  expectRevertWhenPaused,
  calculateRollingHash,
  generateRandomBytes,
  getAccountsFixture,
  hex,
  expectRevertWithReason,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";
import { deployLinethRollupFixture } from "../../rollup/helpers/deploy.js";
import { expectRevert, getBalance, getImpersonatedAccount, randomAddress, setBalance } from "../helpers/index.js";

import type { ClaimMessageWithProofParams } from "../helpers/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Lineth Rollup Yield Extension", () => {
  let linethRollup: ContractReturnType<"TestLinethRollup">;
  let yieldManager: Address;
  let operator: TestAccount;

  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin, securityCouncil, operator, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ yieldManager, linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
  });

  describe("Change yield manager address", () => {
    it("Should revert if the caller does not the SET_YIELD_MANAGER_ROLE", async () => {
      const newYieldManager = randomAddress();
      const setYieldManagerCall = linethRollup.write.setYieldManager([newYieldManager], {
        account: nonAuthorizedAccount.account,
      });

      await expectRevertWithReason(
        setYieldManagerCall,
        buildAccessErrorMessage(nonAuthorizedAccount, SET_YIELD_MANAGER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      const setYieldManagerCall = linethRollup.write.setYieldManager([ADDRESS_ZERO], {
        account: securityCouncil.account,
      });
      await expectRevertWithCustomError(linethRollup, setYieldManagerCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new yield manager address", async () => {
      const newYieldManager = randomAddress();
      await linethRollup.write.setYieldManager([newYieldManager], { account: securityCouncil.account });
      expect(await linethRollup.read.yieldManager()).to.be.equal(newYieldManager);
    });

    it("Should emit the correct event", async () => {
      const oldYieldManagerAddress = await linethRollup.read.yieldManager();
      const newYieldManagerAddress = randomAddress();
      const setYieldManagerCall = linethRollup.write.setYieldManager([newYieldManagerAddress], {
        account: securityCouncil.account,
      });

      await expectEvent(linethRollup, setYieldManagerCall, "YieldManagerChanged", [
        oldYieldManagerAddress,
        newYieldManagerAddress,
      ]);
    });
  });

  describe("IS_WITHDRAW_LST_ALLOWED toggle", () => {
    it("isWithdrawLSTAllowed should return false", async () => {
      expect(await linethRollup.read.isWithdrawLSTAllowed()).to.be.false;
    });
  });

  describe("fund() to receive funding", () => {
    it("Should revert if 0 value received", async () => {
      const fundCall = linethRollup.write.fund({ account: nonAuthorizedAccount.account, value: BigInt(ZERO_VALUE) });
      await expectRevertWithCustomError(linethRollup, fundCall, "NoEthSent");
    });

    it("Should succeed with permissionless call and emit the correct event", async () => {
      const amount = parseEther("1");
      const fundCall = linethRollup.write.fund({ account: nonAuthorizedAccount.account, value: amount });

      await expectEvent(linethRollup, fundCall, "FundingReceived", [amount]);

      const linethRollupBalance = await getBalance({ address: linethRollup.address });
      expect(linethRollupBalance).to.equal(amount);
    });
  });

  describe("transferFundsForNativeYield() to transfer funds to YieldManager", () => {
    it("Should revert if the caller does not the YIELD_PROVIDER_STAKING_ROLE", async () => {
      const transferCall = linethRollup.write.transferFundsForNativeYield([parseEther("1")], {
        account: nonAuthorizedAccount.account,
      });

      await expectRevertWithReason(
        transferCall,
        buildAccessErrorMessage(nonAuthorizedAccount, YIELD_PROVIDER_STAKING_ROLE),
      );
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      await linethRollup.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      const transferCall = linethRollup.write.transferFundsForNativeYield([0n], { account: securityCouncil.account });

      await expectRevertWhenPaused(linethRollup, transferCall, GENERAL_PAUSE_TYPE);
    });

    it("Should revert if NATIVE_YIELD_STAKING pause type is enabled", async () => {
      await linethRollup.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account });

      const transferCall = linethRollup.write.transferFundsForNativeYield([0n], { account: securityCouncil.account });

      await expectRevertWhenPaused(linethRollup, transferCall, NATIVE_YIELD_STAKING_PAUSE_TYPE);
    });

    it("Security council should be able to unpause NATIVE_YIELD_STAKING pause type", async () => {
      await linethRollup.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account });
      await linethRollup.write.unPauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account });
    });

    it("Should revert when non-authorized account enacts NATIVE_YIELD_STAKING pause type", async () => {
      const call = linethRollup.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithReason(
        call,
        buildAccessErrorMessage(nonAuthorizedAccount, await linethRollup.read.PAUSE_NATIVE_YIELD_STAKING_ROLE()),
      );
    });

    it("Should revert when non-authorized account unpauses NATIVE_YIELD_STAKING pause type", async () => {
      await linethRollup.write.pauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], { account: securityCouncil.account });
      const call = linethRollup.write.unPauseByType([NATIVE_YIELD_STAKING_PAUSE_TYPE], {
        account: nonAuthorizedAccount.account,
      });
      await expectRevertWithReason(
        call,
        buildAccessErrorMessage(nonAuthorizedAccount, await linethRollup.read.UNPAUSE_NATIVE_YIELD_STAKING_ROLE()),
      );
    });

    it("Should revert if LinethRollup has balance < _amount", async () => {
      const amount = parseEther("1");
      const transferCall = linethRollup.write.transferFundsForNativeYield([amount], {
        account: securityCouncil.account,
      });

      await expectRevert(transferCall);
    });

    it("Should successfully transfer ETH to the YieldManager", async () => {
      const amount = parseEther("1");
      await linethRollup.write.fund({ account: securityCouncil.account, value: amount });

      const linethRollupAddress = linethRollup.address;
      const initialContractBalance = await getBalance({ address: linethRollupAddress });
      const initialYieldManagerBalance = await getBalance({ address: yieldManager });

      await linethRollup.write.transferFundsForNativeYield([amount], { account: securityCouncil.account });

      const finalContractBalance = await getBalance({ address: linethRollupAddress });
      const finalYieldManagerBalance = await getBalance({ address: yieldManager });

      expect(finalContractBalance).to.equal(initialContractBalance - amount);
      expect(finalYieldManagerBalance).to.equal(initialYieldManagerBalance + amount);
    });
  });

  describe("Yield reporting", () => {
    async function impersonateYieldManager() {
      const yieldManagerSigner = await getImpersonatedAccount(yieldManager);
      await setBalance(yieldManager, parseEther("1"));
      return yieldManagerSigner;
    }

    async function stopYieldManagerImpersonation() {
      const { viem } = await network.getOrCreate();
      const testClient = await viem.getTestClient();
      await testClient.stopImpersonatingAccount({ address: yieldManager });
    }

    const computeMessageHash = (
      from: Address,
      to: Address,
      fee: bigint,
      value: bigint,
      messageNumber: bigint,
      data: Hex,
    ) =>
      keccak256(
        encodeData(
          ["address", "address", "uint256", "uint256", "uint256", "bytes"],
          [from, to, fee, value, messageNumber, data],
        ),
      );

    it("Should revert if caller is not the YieldManager", async () => {
      const reportCall = linethRollup.write.reportNativeYield([1n, operator.address], {
        account: securityCouncil.account,
      });

      await expectRevertWithCustomError(linethRollup, reportCall, "CallerIsNotYieldManager");
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      await linethRollup.write.pauseByType([GENERAL_PAUSE_TYPE], { account: securityCouncil.account });

      const reportCall = linethRollup.write.reportNativeYield([1n, operator.address], {
        account: yieldManagerSigner.account,
      });

      await expectRevertWhenPaused(linethRollup, reportCall, GENERAL_PAUSE_TYPE);

      await stopYieldManagerImpersonation();
    });

    it("Should revert if L1_L2 pause type is enabled", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      await linethRollup.write.pauseByType([L1_L2_PAUSE_TYPE], { account: securityCouncil.account });

      const reportCall = linethRollup.write.reportNativeYield([1n, operator.address], {
        account: yieldManagerSigner.account,
      });

      await expectRevertWhenPaused(linethRollup, reportCall, L1_L2_PAUSE_TYPE);

      await stopYieldManagerImpersonation();
    });

    it("Should revert if l2YieldRecipient is the 0 address", async () => {
      const yieldManagerSigner = await impersonateYieldManager();

      const reportCall = linethRollup.write.reportNativeYield([1n, ADDRESS_ZERO], {
        account: yieldManagerSigner.account,
      });

      await expectRevertWithCustomError(linethRollup, reportCall, "ZeroAddressNotAllowed");

      await stopYieldManagerImpersonation();
    });

    // Ok to allow 0 amount as this is a permissioned function.

    it("Should successfully emit a synthetic MessageSent event with valid parameters", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      const amount = parseEther("1");
      const nextMessageNumberBefore = await linethRollup.read.nextMessageNumber();
      const l2YieldRecipient = randomAddress();

      const expectedMessageHash = computeMessageHash(
        yieldManager,
        l2YieldRecipient,
        0n,
        amount,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );

      const reportCall = linethRollup.write.reportNativeYield([amount, l2YieldRecipient], {
        account: yieldManagerSigner.account,
      });

      await expectEvent(linethRollup, reportCall, "MessageSent", [
        // _from = YieldManager
        yieldManager,
        // _to = L2YieldRecipient function param
        l2YieldRecipient,
        // _fee = 0
        0n,
        // _value = _amount function param
        amount,
        nextMessageNumberBefore,
        // _data = Empty hexstring
        EMPTY_CALLDATA,
        expectedMessageHash,
      ]);

      expect(await linethRollup.read.nextMessageNumber()).to.equal(nextMessageNumberBefore + 1n);

      await stopYieldManagerImpersonation();
    });

    it("Should update the rolling hash when starting with zero hash", async () => {
      // ARRANGE
      const yieldManagerSigner = await impersonateYieldManager();

      const amount = parseEther("0.5");
      const messageNumber = await linethRollup.read.nextMessageNumber();
      const l2YieldRecipient = randomAddress();

      const messageHash = computeMessageHash(yieldManager, l2YieldRecipient, 0n, amount, messageNumber, EMPTY_CALLDATA);
      const expectedRollingHash = calculateRollingHash(zeroHash, messageHash);

      // ACT
      const reportCall = linethRollup.write.reportNativeYield([amount, l2YieldRecipient], {
        account: yieldManagerSigner.account,
      });

      // ASSERT
      await expectEvent(linethRollup, reportCall, "RollingHashUpdated", [
        messageNumber,
        expectedRollingHash,
        messageHash,
      ]);

      const storedRollingHash = await linethRollup.read.rollingHashes([messageNumber]);
      expect(storedRollingHash).to.equal(expectedRollingHash);
      expect(storedRollingHash).to.not.equal(zeroHash);

      await stopYieldManagerImpersonation();
    });

    it("Should correctly update the rolling hash after sendMessage", async () => {
      // ARRANGE STAGE 1 - Perform sendMessage
      const calldataHex = generateRandomBytes(32);

      const initialMessageNumber = await linethRollup.read.nextMessageNumber();
      const sendMessageHash = computeMessageHash(
        securityCouncil.address,
        nonAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        initialMessageNumber,
        calldataHex,
      );
      const expectedRollingAfterSend = calculateRollingHash(zeroHash, sendMessageHash);

      const sendMessageCall = linethRollup.write.sendMessage([nonAuthorizedAccount.address, MESSAGE_FEE, calldataHex], {
        account: securityCouncil.account,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
      });

      await expectEvent(linethRollup, sendMessageCall, "RollingHashUpdated", [
        initialMessageNumber,
        expectedRollingAfterSend,
        sendMessageHash,
      ]);

      const rollingHashAfterSend = await linethRollup.read.rollingHashes([initialMessageNumber]);
      expect(rollingHashAfterSend).to.equal(expectedRollingAfterSend);

      // ARRANGE STAGE 2 - Prepare reportNativeYield
      const yieldManagerSigner = await impersonateYieldManager();
      const amount = parseEther("0.3");
      const messageNumber = await linethRollup.read.nextMessageNumber();
      const l2YieldRecipient = operator.address;

      const yieldMessageHash = computeMessageHash(
        yieldManager,
        l2YieldRecipient,
        0n,
        amount,
        messageNumber,
        EMPTY_CALLDATA,
      );
      const expectedRollingAfterReportYield = calculateRollingHash(rollingHashAfterSend, yieldMessageHash);

      // ACT
      const reportCall = linethRollup.write.reportNativeYield([amount, l2YieldRecipient], {
        account: yieldManagerSigner.account,
      });

      // ASSERT
      await expectEvent(linethRollup, reportCall, "RollingHashUpdated", [
        messageNumber,
        expectedRollingAfterReportYield,
        yieldMessageHash,
      ]);

      const storedRollingHash = await linethRollup.read.rollingHashes([messageNumber]);
      expect(storedRollingHash).to.equal(expectedRollingAfterReportYield);
      expect(storedRollingHash).to.not.equal(zeroHash);
      expect(storedRollingHash).to.not.equal(rollingHashAfterSend);

      await stopYieldManagerImpersonation();
    });
  });

  describe("Claiming message with proof and withdrawing LST", () => {
    it("Should revert if L1MessageService has more than sufficient balance to fulfil withdrawal request amount", async () => {
      const preFundAmount = parseEther("1");
      await linethRollup.write.fund({ account: securityCouncil.account, value: preFundAmount });

      const params: ClaimMessageWithProofParams = {
        proof: [] as Hex[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: parseEther("0.5"),
        feeRecipient: admin.address,
        merkleRoot: zeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([params, operator.address], {
        account: admin.account,
      });

      await expectRevertWithCustomError(linethRollup, claimCall, "LSTWithdrawalRequiresDeficit");

      expect(await getBalance({ address: linethRollup.address })).to.equal(preFundAmount);
    });

    it("Should revert if L1MessageService has the exact balance to fulfil withdrawal request amount", async () => {
      const preFundAmount = parseEther("1");
      await linethRollup.write.fund({ account: securityCouncil.account, value: preFundAmount });

      const params: ClaimMessageWithProofParams = {
        proof: [] as Hex[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: preFundAmount,
        feeRecipient: admin.address,
        merkleRoot: zeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([params, operator.address], {
        account: admin.account,
      });

      await expectRevertWithCustomError(linethRollup, claimCall, "LSTWithdrawalRequiresDeficit");

      expect(await getBalance({ address: linethRollup.address })).to.equal(preFundAmount);
    });

    it("Should revert if caller is not the LST withdrawal recipient", async () => {
      const preFundAmount = parseEther("1");
      await linethRollup.write.fund({ account: securityCouncil.account, value: preFundAmount });

      const params: ClaimMessageWithProofParams = {
        proof: [] as Hex[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: nonAuthorizedAccount.address,
        fee: 0n,
        value: parseEther("0.5"),
        feeRecipient: admin.address,
        merkleRoot: zeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([params, operator.address], {
        account: admin.account,
      });

      await expectRevertWithCustomError(linethRollup, claimCall, "LSTWithdrawalRequiresDeficit");
    });

    it("Should revert on reentry", async () => {
      const linethRollupAddress = linethRollup.address;

      const l2MerkleRootsDepthsSlot = 336n;
      const storageSlot = keccak256(
        encodeData(["bytes32", "uint256"], [VALID_MERKLE_PROOF.merkleRoot, l2MerkleRootsDepthsSlot]),
      );

      const { viem, networkHelpers } = await network.getOrCreate();
      await networkHelpers.setStorageAt(
        linethRollupAddress,
        storageSlot,
        toHex(BigInt(VALID_MERKLE_PROOF.proof.length), { size: 32 }),
      );

      const claimParams: ClaimMessageWithProofParams = {
        proof: VALID_MERKLE_PROOF.proof.map(hex),
        messageNumber: 1n,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: hex(VALID_MERKLE_PROOF.merkleRoot),
        data: EMPTY_CALLDATA,
      };

      const mockYieldManagerContract = await viem.getContractAt("MockYieldManager", yieldManager, {
        client: { wallet: securityCouncil },
      });
      await mockYieldManagerContract.write.setReentryData([claimParams, operator.address], { account: admin.account });

      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, operator.address], {
        account: admin.account,
      });

      await expectRevertWithCustomError(linethRollup, claimCall, "ReentrantCall");
    });

    it("Should claim successfully with correct MessageClaimed event emitted", async () => {
      const { viem } = await network.getOrCreate();
      await linethRollup.write.addL2MerkleRoots(
        [[hex(VALID_MERKLE_PROOF.merkleRoot)], BigInt(VALID_MERKLE_PROOF.proof.length)],
        { account: securityCouncil.account },
      );

      const expectedBytes = encodeSendMessage(
        admin.address,
        admin.address,
        MESSAGE_FEE,
        MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = keccak256(expectedBytes);

      const claimParams: ClaimMessageWithProofParams = {
        proof: VALID_MERKLE_PROOF.proof.map(hex),
        messageNumber: 1n,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: hex(VALID_MERKLE_PROOF.merkleRoot),
        data: EMPTY_CALLDATA,
      };

      // Act
      const claimCall = linethRollup.write.claimMessageWithProofAndWithdrawLST([claimParams, operator.address], {
        account: admin.account,
      });

      // Assert MessageClaimed event emitted
      await expectEvent(linethRollup, claimCall, "MessageClaimed", [messageHash]);
      const mockYieldManagerContract = await viem.getContractAt("MockYieldManager", yieldManager, {
        client: { wallet: securityCouncil },
      });
      // Assert that isWithdrawLSTAllowed() flag is toggled on during the tx - use event on MockYieldManager
      await expectEvent(mockYieldManagerContract, claimCall, "LSTWithdrawalFlag", [true]);
      // Assert that isWithdrawLSTAllowed() flag is toggled off after the tx
      expect(await linethRollup.read.isWithdrawLSTAllowed()).to.be.false;
    });
  });
});
