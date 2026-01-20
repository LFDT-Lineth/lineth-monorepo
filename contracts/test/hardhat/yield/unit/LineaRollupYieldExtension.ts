// TODO rename to LineaRollupYieldExtension
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";

import { MockYieldManager__factory, TestLineaRollup } from "contracts/typechain-types";
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
} from "../../common/constants";
import {
  expectEvent,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectRevertWithReason,
  calculateRollingHash,
  getAccountsFixture,
  encodeSendMessage,
} from "../../common/helpers";
import { deployLineaRollupFixture } from "../../rollup/helpers/deploy";

describe("Linea Rollup Yield Extension", () => {
  let lineaRollup: TestLineaRollup;
  let yieldManager: string;
  let operator: SignerWithAddress;

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ yieldManager, lineaRollup } = await loadFixture(deployLineaRollupFixture));
  });

  describe("Change yield manager address", () => {
    it("Should revert if the caller does not the SET_YIELD_MANAGER_ROLE", async () => {
      const newYieldManager = ethers.Wallet.createRandom().address;
      const setYieldManagerCall = lineaRollup.connect(nonAuthorizedAccount).setYieldManager(newYieldManager);

      await expectRevertWithReason(
        setYieldManagerCall,
        buildAccessErrorMessage(nonAuthorizedAccount, SET_YIELD_MANAGER_ROLE),
      );
    });

    it("Should revert if the address being set is the zero address", async () => {
      const setYieldManagerCall = lineaRollup.connect(securityCouncil).setYieldManager(ADDRESS_ZERO);
      await expectRevertWithCustomError(lineaRollup, setYieldManagerCall, "ZeroAddressNotAllowed");
    });

    it("Should set the new yield manager address", async () => {
      const newYieldManager = ethers.Wallet.createRandom().address;
      await lineaRollup.connect(securityCouncil).setYieldManager(newYieldManager);
      expect(await lineaRollup.yieldManager()).to.be.equal(newYieldManager);
    });

    it("Should emit the correct event", async () => {
      const oldYieldManagerAddress = await lineaRollup.yieldManager();
      const newYieldManagerAddress = ethers.Wallet.createRandom().address;
      const setYieldManagerCall = lineaRollup.connect(securityCouncil).setYieldManager(newYieldManagerAddress);

      await expectEvent(lineaRollup, setYieldManagerCall, "YieldManagerChanged", [
        oldYieldManagerAddress,
        newYieldManagerAddress,
      ]);
    });
  });

  describe("IS_WITHDRAW_LST_ALLOWED toggle", () => {
    it("isWithdrawLSTAllowed should return false", async () => {
      expect(await lineaRollup.isWithdrawLSTAllowed()).to.be.false;
    });
  });

  describe("fund() to receive funding", () => {
    it("Should revert if 0 value received", async () => {
      const fundCall = lineaRollup.connect(nonAuthorizedAccount).fund({ value: ZERO_VALUE });
      expectRevertWithCustomError(lineaRollup, fundCall, "NoEthSent");
    });

    it("Should succeed with permissionless call and emit the correct event", async () => {
      const amount = ethers.parseEther("1");
      const fundCall = lineaRollup.connect(nonAuthorizedAccount).fund({ value: amount });

      await expectEvent(lineaRollup, fundCall, "FundingReceived", [amount]);

      const lineaRollupBalance = await ethers.provider.getBalance(await lineaRollup.getAddress());
      expect(lineaRollupBalance).to.equal(amount);
    });
  });

  describe("transferFundsForNativeYield() to transfer funds to YieldManager", () => {
    it("Should revert if the caller does not the YIELD_PROVIDER_STAKING_ROLE", async () => {
      const transferCall = lineaRollup
        .connect(nonAuthorizedAccount)
        .transferFundsForNativeYield(ethers.parseEther("1"));

      await expectRevertWithReason(
        transferCall,
        buildAccessErrorMessage(nonAuthorizedAccount, YIELD_PROVIDER_STAKING_ROLE),
      );
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const transferCall = lineaRollup.connect(securityCouncil).transferFundsForNativeYield(0n);

      await expectRevertWithCustomError(lineaRollup, transferCall, "IsPaused", [GENERAL_PAUSE_TYPE]);
    });

    it("Should revert if NATIVE_YIELD_STAKING pause type is enabled", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);

      const transferCall = lineaRollup.connect(securityCouncil).transferFundsForNativeYield(0n);

      await expectRevertWithCustomError(lineaRollup, transferCall, "IsPaused", [NATIVE_YIELD_STAKING_PAUSE_TYPE]);
    });

    it("Security council should be able to unpause NATIVE_YIELD_STAKING pause type", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);
      await lineaRollup.connect(securityCouncil).unPauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);
    });

    it("Should revert when non-authorized account enacts NATIVE_YIELD_STAKING pause type", async () => {
      const call = lineaRollup.connect(nonAuthorizedAccount).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);
      await expect(call).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await lineaRollup.PAUSE_NATIVE_YIELD_STAKING_ROLE()),
      );
    });

    it("Should revert when non-authorized account unpauses NATIVE_YIELD_STAKING pause type", async () => {
      await lineaRollup.connect(securityCouncil).pauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);
      const call = lineaRollup.connect(nonAuthorizedAccount).unPauseByType(NATIVE_YIELD_STAKING_PAUSE_TYPE);
      await expect(call).to.be.revertedWith(
        buildAccessErrorMessage(nonAuthorizedAccount, await lineaRollup.UNPAUSE_NATIVE_YIELD_STAKING_ROLE()),
      );
    });

    it("Should revert if LineaRollup has balance < _amount", async () => {
      const amount = ethers.parseEther("1");
      const transferCall = lineaRollup.connect(securityCouncil).transferFundsForNativeYield(amount);

      await expect(transferCall).to.be.reverted;
    });

    it("Should successfully transfer ETH to the YieldManager", async () => {
      const amount = ethers.parseEther("1");
      await lineaRollup.connect(securityCouncil).fund({ value: amount });

      const lineaRollupAddress = await lineaRollup.getAddress();
      const initialContractBalance = await ethers.provider.getBalance(lineaRollupAddress);
      const initialYieldManagerBalance = await ethers.provider.getBalance(yieldManager);

      await lineaRollup.connect(securityCouncil).transferFundsForNativeYield(amount);

      const finalContractBalance = await ethers.provider.getBalance(lineaRollupAddress);
      const finalYieldManagerBalance = await ethers.provider.getBalance(yieldManager);

      expect(finalContractBalance).to.equal(initialContractBalance - amount);
      expect(finalYieldManagerBalance).to.equal(initialYieldManagerBalance + amount);
    });
  });

  describe("Yield reporting", () => {
    async function impersonateYieldManager() {
      await ethers.provider.send("hardhat_impersonateAccount", [yieldManager]);
      await ethers.provider.send("hardhat_setBalance", [yieldManager, ethers.toBeHex(ethers.parseEther("1"))]);
      return await ethers.getSigner(yieldManager);
    }

    async function stopYieldManagerImpersonation() {
      await ethers.provider.send("hardhat_stopImpersonatingAccount", [yieldManager]);
    }

    const abiCoder = ethers.AbiCoder.defaultAbiCoder();

    const computeMessageHash = (
      from: string,
      to: string,
      fee: bigint,
      value: bigint,
      messageNumber: bigint,
      data: string,
    ) =>
      ethers.keccak256(
        abiCoder.encode(
          ["address", "address", "uint256", "uint256", "uint256", "bytes"],
          [from, to, fee, value, messageNumber, data],
        ),
      );

    it("Should revert if caller is not the YieldManager", async () => {
      const reportCall = lineaRollup.connect(securityCouncil).reportNativeYield(1n, operator.address);

      await expectRevertWithCustomError(lineaRollup, reportCall, "CallerIsNotYieldManager");
    });

    it("Should revert if GENERAL_PAUSE_TYPE is enabled", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      await lineaRollup.connect(securityCouncil).pauseByType(GENERAL_PAUSE_TYPE);

      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(1n, operator.address);

      await expectRevertWithCustomError(lineaRollup, reportCall, "IsPaused", [GENERAL_PAUSE_TYPE]);

      await stopYieldManagerImpersonation();
    });

    it("Should revert if L1_L2 pause type is enabled", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      await lineaRollup.connect(securityCouncil).pauseByType(L1_L2_PAUSE_TYPE);

      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(1n, operator.address);

      await expectRevertWithCustomError(lineaRollup, reportCall, "IsPaused", [L1_L2_PAUSE_TYPE]);

      await stopYieldManagerImpersonation();
    });

    it("Should revert if l2YieldRecipient is the 0 address", async () => {
      const yieldManagerSigner = await impersonateYieldManager();

      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(1n, ADDRESS_ZERO);

      await expectRevertWithCustomError(lineaRollup, reportCall, "ZeroAddressNotAllowed");

      await stopYieldManagerImpersonation();
    });

    // Ok to allow 0 amount as this is a permissioned function.

    it("Should successfully emit a synthetic MessageSent event with valid parameters", async () => {
      const yieldManagerSigner = await impersonateYieldManager();
      const amount = ethers.parseEther("1");
      const lineaRollupAddress = await lineaRollup.getAddress();
      const nextMessageNumberBefore = await lineaRollup.nextMessageNumber();
      const l2YieldRecipient = ethers.Wallet.createRandom().address;

      const expectedMessageHash = computeMessageHash(
        lineaRollupAddress,
        l2YieldRecipient,
        0n,
        amount,
        nextMessageNumberBefore,
        EMPTY_CALLDATA,
      );

      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(amount, l2YieldRecipient);

      await expectEvent(lineaRollup, reportCall, "MessageSent", [
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

      expect(await lineaRollup.nextMessageNumber()).to.equal(nextMessageNumberBefore + 1n);

      await stopYieldManagerImpersonation();
    });

    it("Should update the rolling hash when starting with zero hash", async () => {
      // ARRANGE
      const yieldManagerSigner = await impersonateYieldManager();

      const amount = ethers.parseEther("0.5");
      const lineaRollupAddress = await lineaRollup.getAddress();
      const messageNumber = await lineaRollup.nextMessageNumber();
      const l2YieldRecipient = ethers.Wallet.createRandom().address;

      const messageHash = computeMessageHash(
        lineaRollupAddress,
        l2YieldRecipient,
        0n,
        amount,
        messageNumber,
        EMPTY_CALLDATA,
      );
      const expectedRollingHash = calculateRollingHash(ethers.ZeroHash, messageHash);

      // ACT
      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(amount, l2YieldRecipient);

      // ASSERT
      await expectEvent(lineaRollup, reportCall, "RollingHashUpdated", [
        messageNumber,
        expectedRollingHash,
        messageHash,
      ]);

      const storedRollingHash = await lineaRollup.rollingHashes(messageNumber);
      expect(storedRollingHash).to.equal(expectedRollingHash);
      expect(storedRollingHash).to.not.equal(ethers.ZeroHash);

      await stopYieldManagerImpersonation();
    });

    it("Should correctly update the rolling hash after sendMessage", async () => {
      // ARRANGE STAGE 1 - Perform sendMessage
      const lineaRollupAddress = await lineaRollup.getAddress();
      const calldataPayload = ethers.randomBytes(32);
      const calldataHex = ethers.hexlify(calldataPayload);

      const initialMessageNumber = await lineaRollup.nextMessageNumber();
      const sendMessageHash = computeMessageHash(
        securityCouncil.address,
        nonAuthorizedAccount.address,
        MESSAGE_FEE,
        MESSAGE_VALUE_1ETH,
        initialMessageNumber,
        calldataHex,
      );
      const expectedRollingAfterSend = calculateRollingHash(ethers.ZeroHash, sendMessageHash);

      const sendMessageCall = lineaRollup
        .connect(securityCouncil)
        .sendMessage(nonAuthorizedAccount.address, MESSAGE_FEE, calldataHex, {
          value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        });

      await expectEvent(lineaRollup, sendMessageCall, "RollingHashUpdated", [
        initialMessageNumber,
        expectedRollingAfterSend,
        sendMessageHash,
      ]);

      const rollingHashAfterSend = await lineaRollup.rollingHashes(initialMessageNumber);
      expect(rollingHashAfterSend).to.equal(expectedRollingAfterSend);

      // ARRANGE STAGE 2 - Prepare reportNativeYield
      const yieldManagerSigner = await impersonateYieldManager();
      const amount = ethers.parseEther("0.3");
      const messageNumber = await lineaRollup.nextMessageNumber();
      const l2YieldRecipient = operator.address;

      const yieldMessageHash = computeMessageHash(
        lineaRollupAddress,
        l2YieldRecipient,
        0n,
        amount,
        messageNumber,
        EMPTY_CALLDATA,
      );
      const expectedRollingAfterReportYield = calculateRollingHash(rollingHashAfterSend, yieldMessageHash);

      // ACT
      const reportCall = lineaRollup.connect(yieldManagerSigner).reportNativeYield(amount, l2YieldRecipient);

      // ASSERT
      await expectEvent(lineaRollup, reportCall, "RollingHashUpdated", [
        messageNumber,
        expectedRollingAfterReportYield,
        yieldMessageHash,
      ]);

      const storedRollingHash = await lineaRollup.rollingHashes(messageNumber);
      expect(storedRollingHash).to.equal(expectedRollingAfterReportYield);
      expect(storedRollingHash).to.not.equal(ethers.ZeroHash);
      expect(storedRollingHash).to.not.equal(rollingHashAfterSend);

      await stopYieldManagerImpersonation();
    });
  });

  describe("Claiming message with proof and withdrawing LST", () => {
    it("Should revert if L1MessageService has more than sufficient balance to fulfil withdrawal request amount", async () => {
      const preFundAmount = ethers.parseEther("1");
      await lineaRollup.connect(securityCouncil).fund({ value: preFundAmount });

      const params = {
        proof: [] as string[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: ethers.parseEther("0.5"),
        feeRecipient: admin.address,
        merkleRoot: ethers.ZeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = lineaRollup.connect(admin).claimMessageWithProofAndWithdrawLST(params, operator.address);

      await expectRevertWithCustomError(lineaRollup, claimCall, "LSTWithdrawalRequiresDeficit");

      expect(await ethers.provider.getBalance(await lineaRollup.getAddress())).to.equal(preFundAmount);
    });

    it("Should revert if L1MessageService has the exact balance to fulfil withdrawal request amount", async () => {
      const preFundAmount = ethers.parseEther("1");
      await lineaRollup.connect(securityCouncil).fund({ value: preFundAmount });

      const params = {
        proof: [] as string[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: preFundAmount,
        feeRecipient: admin.address,
        merkleRoot: ethers.ZeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = lineaRollup.connect(admin).claimMessageWithProofAndWithdrawLST(params, operator.address);

      await expectRevertWithCustomError(lineaRollup, claimCall, "LSTWithdrawalRequiresDeficit");

      expect(await ethers.provider.getBalance(await lineaRollup.getAddress())).to.equal(preFundAmount);
    });

    it("Should revert if caller is not the LST withdrawal recipient", async () => {
      const preFundAmount = ethers.parseEther("1");
      await lineaRollup.connect(securityCouncil).fund({ value: preFundAmount });

      const params = {
        proof: [] as string[],
        messageNumber: 0n,
        leafIndex: 0,
        from: admin.address,
        to: nonAuthorizedAccount.address,
        fee: 0n,
        value: ethers.parseEther("0.5"),
        feeRecipient: admin.address,
        merkleRoot: ethers.ZeroHash,
        data: EMPTY_CALLDATA,
      };

      const claimCall = lineaRollup.connect(admin).claimMessageWithProofAndWithdrawLST(params, operator.address);

      await expectRevertWithCustomError(lineaRollup, claimCall, "LSTWithdrawalRequiresDeficit");
    });

    it("Should revert on reentry", async () => {
      const lineaRollupAddress = await lineaRollup.getAddress();

      const l2MerkleRootsDepthsSlot = 336n;
      const storageSlot = ethers.keccak256(
        ethers.AbiCoder.defaultAbiCoder().encode(
          ["bytes32", "uint256"],
          [VALID_MERKLE_PROOF.merkleRoot, l2MerkleRootsDepthsSlot],
        ),
      );

      await ethers.provider.send("hardhat_setStorageAt", [
        lineaRollupAddress,
        storageSlot,
        ethers.zeroPadValue(ethers.toBeHex(BigInt(VALID_MERKLE_PROOF.proof.length)), 32),
      ]);

      const claimParams = {
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1n,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      };

      const mockYieldManagerContract = MockYieldManager__factory.connect(yieldManager, securityCouncil);
      await mockYieldManagerContract.connect(admin).setReentryData(claimParams, operator.address);

      const claimCall = lineaRollup.connect(admin).claimMessageWithProofAndWithdrawLST(claimParams, operator.address);

      await expectRevertWithCustomError(lineaRollup, claimCall, "ReentrantCall");
    });

    it("Should claim successfully with correct MessageClaimed event emitted", async () => {
      await lineaRollup
        .connect(securityCouncil)
        .addL2MerkleRoots([VALID_MERKLE_PROOF.merkleRoot], VALID_MERKLE_PROOF.proof.length);

      const expectedBytes = await encodeSendMessage(
        admin.address,
        admin.address,
        MESSAGE_FEE,
        MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        1n,
        EMPTY_CALLDATA,
      );

      const messageHash = ethers.keccak256(expectedBytes);

      const claimParams = {
        proof: VALID_MERKLE_PROOF.proof,
        messageNumber: 1n,
        leafIndex: VALID_MERKLE_PROOF.index,
        from: admin.address,
        to: admin.address,
        fee: MESSAGE_FEE,
        value: MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF.merkleRoot,
        data: EMPTY_CALLDATA,
      };

      // Act
      const claimCall = lineaRollup.connect(admin).claimMessageWithProofAndWithdrawLST(claimParams, operator.address);

      // Assert MessageClaimed event emitted
      await expectEvent(lineaRollup, claimCall, "MessageClaimed", [messageHash]);
      const mockYieldManagerContract = MockYieldManager__factory.connect(yieldManager, securityCouncil);
      // Assert that isWithdrawLSTAllowed() flag is toggled on during the tx - use event on MockYieldManager
      await expectEvent(mockYieldManagerContract, claimCall, "LSTWithdrawalFlag", [true]);
      // Assert that isWithdrawLSTAllowed() flag is toggled off after the tx
      expect(await lineaRollup.isWithdrawLSTAllowed()).to.be.false;
    });
  });
});
