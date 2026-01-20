import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";

import { MockLineaRollup, TestYieldManager } from "contracts/typechain-types";
import {
  deployMockYieldProvider,
  deployYieldManagerForUnitTest,
  deployYieldManagerForUnitTestWithMutatedInitData,
} from "../helpers/deploy";
import { addMockYieldProvider, buildMockYieldProviderRegistration } from "../helpers/mocks";
import { MINIMUM_FEE, EMPTY_CALLDATA, ONE_THOUSAND_ETHER, MAX_BPS, ZERO_VALUE } from "../../common/constants";
import { buildAccessErrorMessage, expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
import { YieldManagerInitializationData } from "../helpers/types";
import { ZeroAddress } from "ethers";
import { buildSetWithdrawalReserveParams } from "../helpers";

describe("YieldManager contract - basic operations", () => {
  let yieldManager: TestYieldManager;

  let securityCouncil: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;
  let nativeYieldOperator: SignerWithAddress;
  let mockLineaRollup: MockLineaRollup;
  let initializationData: YieldManagerInitializationData;

  before(async () => {
    ({ securityCouncil, operator: nonAuthorizedAccount, nativeYieldOperator } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ yieldManager, initializationData, mockLineaRollup } = await loadFixture(deployYieldManagerForUnitTest));
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: string) => {
      return nativeYieldOperator.sendTransaction({
        to: await yieldManager.getAddress(),
        value: MINIMUM_FEE,
        data,
      });
    };

    it("Should fail to send eth to the yieldManager contract through the fallback function", async () => {
      await expect(sendEthToContract("0x1234")).to.be.reverted;
    });

    it("Should successfully accept ETH via receive() fn", async () => {
      await expect(sendEthToContract(EMPTY_CALLDATA)).to.not.be.reverted;
    });

    it("Should decrement pendingPermissionlessUnstake when ETH is received", async () => {
      await yieldManager.setPendingPermissionlessUnstake(MINIMUM_FEE);
      await expect(sendEthToContract(EMPTY_CALLDATA)).to.not.be.reverted;
      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(ZERO_VALUE);
    });
  });

  describe("Constructor", () => {
    it("Should successfully set the L1MessageService and emit the expected event", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      const yieldManagerFactory = await ethers.getContractFactory("TestYieldManager");
      const deployedYieldManager = await yieldManagerFactory.deploy(l1MessageServiceAddress);
      expect(deployedYieldManager.deploymentTransaction)
        .to.emit(yieldManager, "YieldManagerDeployed")
        .withArgs(l1MessageServiceAddress);
      await deployedYieldManager.waitForDeployment();
      expect(await deployedYieldManager.L1_MESSAGE_SERVICE()).to.equal(l1MessageServiceAddress);
    });
  });

  describe("Initialisation", () => {
    const cloneInitializationData = (
      overrides: Partial<YieldManagerInitializationData> = {},
    ): YieldManagerInitializationData => ({
      pauseTypeRoles: initializationData.pauseTypeRoles.map((role) => ({ ...role })),
      unpauseTypeRoles: initializationData.unpauseTypeRoles.map((role) => ({ ...role })),
      roleAddresses: initializationData.roleAddresses.map((role) => ({ ...role })),
      initialL2YieldRecipients: [...initializationData.initialL2YieldRecipients],
      defaultAdmin: initializationData.defaultAdmin,
      initialMinimumWithdrawalReservePercentageBps: initializationData.initialMinimumWithdrawalReservePercentageBps,
      initialTargetWithdrawalReservePercentageBps: initializationData.initialTargetWithdrawalReservePercentageBps,
      initialMinimumWithdrawalReserveAmount: initializationData.initialMinimumWithdrawalReserveAmount,
      initialTargetWithdrawalReserveAmount: initializationData.initialTargetWithdrawalReserveAmount,
      ...overrides,
    });

    it("Should revert if the default admin address is zero address", async () => {
      const mutatedInitData = cloneInitializationData({ defaultAdmin: ZeroAddress });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if a 0 address is provided for an l2YieldRecipient", async () => {
      const mutatedInitData = cloneInitializationData({ initialL2YieldRecipients: [ZeroAddress] });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if >10000 bps is provided for initialTargetWithdrawalReservePercentageBps", async () => {
      const mutatedInitData = cloneInitializationData({ initialTargetWithdrawalReservePercentageBps: 10001 });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if >10000 bps is provided for initialMinimumWithdrawalReservePercentageBps", async () => {
      const mutatedInitData = cloneInitializationData({ initialMinimumWithdrawalReservePercentageBps: 10001 });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if initialMinimumWithdrawalReservePercentageBps > initialTargetWithdrawalReservePercentageBps", async () => {
      const mutatedInitData = cloneInitializationData({
        initialTargetWithdrawalReservePercentageBps: initializationData.initialMinimumWithdrawalReservePercentageBps,
        initialMinimumWithdrawalReservePercentageBps: initializationData.initialTargetWithdrawalReservePercentageBps,
      });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "TargetReservePercentageMustBeAboveMinimum",
      );
    });

    it("Should revert if initialMinimumWithdrawalReserveAmount > initialTargetWithdrawalReserveAmount", async () => {
      const mutatedInitData = cloneInitializationData({
        initialTargetWithdrawalReserveAmount: initializationData.initialMinimumWithdrawalReserveAmount,
        initialMinimumWithdrawalReserveAmount: initializationData.initialTargetWithdrawalReserveAmount,
      });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "TargetReserveAmountMustBeAboveMinimum",
      );
    });

    it("Should revert if the initialize function is called a second time", async () => {
      await expect(yieldManager.initialize(cloneInitializationData())).to.be.revertedWith(
        "Initializable: contract is already initialized",
      );
    });

    it("Should emit the correct initialization event", async () => {
      const deployTx = await yieldManager.deploymentTransaction();
      expect(deployTx).to.not.equal(null);
      await expect(deployTx!)
        .to.emit(yieldManager, "YieldManagerInitialized")
        .withArgs(initializationData.initialL2YieldRecipients);
    });

    it("Should have the correct L1_MESSAGE_ADDRESS", async () => {
      expect(await yieldManager.L1_MESSAGE_SERVICE()).to.equal(await mockLineaRollup.getAddress());
    });

    it("Should have the initial l2YieldRecipients in state", async () => {
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      expect(await yieldManager.isL2YieldRecipientKnown(existingRecipient)).to.be.true;
    });

    it("Should revert for index 0 yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.yieldProviderByIndex(0),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
    });

    it("Should not register the zero address as a known yield provider", async () => {
      expect(await yieldManager.isYieldProviderKnown(ZeroAddress)).to.equal(false);
    });

    it("Should have the initial minimum reserve percentage in state", async () => {
      expect(await yieldManager.minimumWithdrawalReservePercentageBps()).to.equal(
        BigInt(initializationData.initialMinimumWithdrawalReservePercentageBps),
      );
    });

    it("Should have the initial minimum reserve amount in state", async () => {
      expect(await yieldManager.minimumWithdrawalReserveAmount()).to.equal(
        initializationData.initialMinimumWithdrawalReserveAmount,
      );
    });

    it("Should have the initial target reserve percentage in state", async () => {
      expect(await yieldManager.targetWithdrawalReservePercentageBps()).to.equal(
        BigInt(initializationData.initialTargetWithdrawalReservePercentageBps),
      );
    });

    it("Should have the initial target reserve amount in state", async () => {
      expect(await yieldManager.targetWithdrawalReserveAmount()).to.equal(
        initializationData.initialTargetWithdrawalReserveAmount,
      );
    });

    it("Should have initial userFundsInYieldProvidersTotal = 0", async () => {
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(ZERO_VALUE);
    });

    it("Should have initial pendingPermissionlessUnstake = 0", async () => {
      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(ZERO_VALUE);
    });

    it("Should have initial yieldProviderCount = 0", async () => {
      expect(await yieldManager.yieldProviderCount()).to.equal(ZERO_VALUE);
    });

    it("Should assign the YIELD_PROVIDER_STAKING_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.YIELD_PROVIDER_STAKING_ROLE();
      expect(await yieldManager.hasRole(role, nativeYieldOperator.address)).to.be.true;
    });

    it("Should assign the YIELD_PROVIDER_UNSTAKER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE();
      expect(await yieldManager.hasRole(role, nativeYieldOperator.address)).to.be.true;
    });

    it("Should assign the YIELD_REPORTER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.YIELD_REPORTER_ROLE();
      expect(await yieldManager.hasRole(role, nativeYieldOperator.address)).to.be.true;
    });

    it("Should assign the STAKING_PAUSE_CONTROLLER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.STAKING_PAUSE_CONTROLLER_ROLE();
      expect(await yieldManager.hasRole(role, nativeYieldOperator.address)).to.be.true;
    });

    it("Should assign the OSSIFICATION_PROCESSOR_ROLE to nativeYieldOperator address", async () => {
      const ossifierRole = await yieldManager.OSSIFICATION_PROCESSOR_ROLE();
      expect(await yieldManager.hasRole(ossifierRole, nativeYieldOperator.address)).to.be.true;
    });

    it("Should also assign operator roles to securityCouncil address", async () => {
      const roles = [
        await yieldManager.YIELD_PROVIDER_STAKING_ROLE(),
        await yieldManager.YIELD_PROVIDER_UNSTAKER_ROLE(),
        await yieldManager.STAKING_PAUSE_CONTROLLER_ROLE(),
        await yieldManager.OSSIFICATION_PROCESSOR_ROLE(),
        await yieldManager.YIELD_REPORTER_ROLE(),
      ];
      await Promise.all(
        roles.map(async (role) => expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true),
      );
    });

    it("Should assign the OSSIFICATION_INITIATOR_ROLE to securityCouncil address", async () => {
      const ossifierRole = await yieldManager.OSSIFICATION_INITIATOR_ROLE();
      expect(await yieldManager.hasRole(ossifierRole, securityCouncil.address)).to.be.true;
    });

    it("Should not assign the OSSIFICATION_INITIATOR_ROLE to nativeYieldOperator address", async () => {
      const ossifierRole = await yieldManager.OSSIFICATION_INITIATOR_ROLE();
      expect(await yieldManager.hasRole(ossifierRole, nativeYieldOperator.address)).to.be.false;
    });

    it("Should assign the SET_YIELD_PROVIDER_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.SET_YIELD_PROVIDER_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the SET_L2_YIELD_RECIPIENT_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.SET_L2_YIELD_RECIPIENT_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the WITHDRAWAL_RESERVE_SETTER_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.WITHDRAWAL_RESERVE_SETTER_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should not assign the WITHDRAWAL_RESERVE_SETTER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.WITHDRAWAL_RESERVE_SETTER_ROLE();
      expect(await yieldManager.hasRole(role, nativeYieldOperator.address)).to.be.false;
    });

    it("Should assign the PAUSE_ALL_ROLE to securityCouncil address", async () => {
      const pauseAllRole = await yieldManager.PAUSE_ALL_ROLE();
      expect(await yieldManager.hasRole(pauseAllRole, securityCouncil.address)).to.be.true;
    });

    it("Should assign the UNPAUSE_ALL_ROLE to securityCouncil address", async () => {
      const unpauseAllRole = await yieldManager.UNPAUSE_ALL_ROLE();
      expect(await yieldManager.hasRole(unpauseAllRole, securityCouncil.address)).to.be.true;
    });

    it("Should assign the PAUSE_NATIVE_YIELD_STAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.PAUSE_NATIVE_YIELD_STAKING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_STAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.UNPAUSE_NATIVE_YIELD_STAKING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the PAUSE_NATIVE_YIELD_UNSTAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.PAUSE_NATIVE_YIELD_UNSTAKING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the PAUSE_NATIVE_YIELD_REPORTING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.PAUSE_NATIVE_YIELD_REPORTING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_REPORTING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.UNPAUSE_NATIVE_YIELD_REPORTING_ROLE();
      expect(await yieldManager.hasRole(role, securityCouncil.address)).to.be.true;
    });
  });

  describe("Adding and removing L2YieldRecipients", () => {
    it("Should revert when adding if the caller does not have the SET_L2_YIELD_RECIPIENT_ROLE", async () => {
      const requiredRole = await yieldManager.SET_L2_YIELD_RECIPIENT_ROLE();
      await expect(
        yieldManager.connect(nonAuthorizedAccount).addL2YieldRecipient(nonAuthorizedAccount.address),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, requiredRole));
    });

    it("Should add the new l2YieldRecipient address and emit the correct event", async () => {
      const newRecipient = ethers.Wallet.createRandom().address;
      const addTx = yieldManager.connect(securityCouncil).addL2YieldRecipient(newRecipient);

      await expect(addTx).to.emit(yieldManager, "L2YieldRecipientAdded").withArgs(newRecipient);

      expect(await yieldManager.isL2YieldRecipientKnown(newRecipient)).to.be.true;
    });

    it("Should revert if the address being added has already been added", async () => {
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).addL2YieldRecipient(existingRecipient),
        "L2YieldRecipientAlreadyAdded",
      );
    });

    it("Should revert if the address being removed is unknown", async () => {
      const unknownRecipient = ethers.Wallet.createRandom().address;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).removeL2YieldRecipient(unknownRecipient),
        "UnknownL2YieldRecipient",
      );
    });

    it("Should revert when removing if the caller does not have the SET_L2_YIELD_RECIPIENT_ROLE", async () => {
      const requiredRole = await yieldManager.SET_L2_YIELD_RECIPIENT_ROLE();
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      await expect(
        yieldManager.connect(nonAuthorizedAccount).removeL2YieldRecipient(existingRecipient),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, requiredRole));
    });

    it("Should remove the new l2YieldRecipient address and emit the correct event", async () => {
      const recipientToRemove = initializationData.initialL2YieldRecipients[0];
      const removeTx = yieldManager.connect(securityCouncil).removeL2YieldRecipient(recipientToRemove);

      await expect(removeTx).to.emit(yieldManager, "L2YieldRecipientRemoved").withArgs(recipientToRemove);

      expect(await yieldManager.isL2YieldRecipientKnown(recipientToRemove)).to.be.false;
    });
  });

  describe("Setting withdrawal reserve parameters", () => {
    it("Should revert set withdrawal reserve parameters when the caller does not have the WITHDRAWAL_RESERVE_SETTER_ROLE role", async () => {
      const role = await yieldManager.WITHDRAWAL_RESERVE_SETTER_ROLE();
      await expect(
        yieldManager
          .connect(nonAuthorizedAccount)
          .setWithdrawalReserveParameters(buildSetWithdrawalReserveParams(initializationData)),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, role));
    });

    it("Should revert if minimum withdrawal percentage higher than 10000 bps", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(securityCouncil)
          .setWithdrawalReserveParameters(buildSetWithdrawalReserveParams(initializationData, { minPct: 10001 })),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if target withdrawal percentage higher than 10000 bps", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(securityCouncil)
          .setWithdrawalReserveParameters(buildSetWithdrawalReserveParams(initializationData, { targetPct: 10001 })),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if minimum withdrawal reserve percentage > target", async () => {
      const target = initializationData.initialTargetWithdrawalReservePercentageBps;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(securityCouncil)
          .setWithdrawalReserveParameters(
            buildSetWithdrawalReserveParams(initializationData, { minPct: target + 1, targetPct: target }),
          ),
        "TargetReservePercentageMustBeAboveMinimum",
      );
    });

    it("Should revert if minimum withdrawal amount > target", async () => {
      const target = initializationData.initialTargetWithdrawalReserveAmount;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager
          .connect(securityCouncil)
          .setWithdrawalReserveParameters(
            buildSetWithdrawalReserveParams(initializationData, { minAmount: target + 1n, targetAmount: target }),
          ),
        "TargetReserveAmountMustBeAboveMinimum",
      );
    });

    it("Should successfully set withdrawal reserve parameters and emit logs", async () => {
      const params = buildSetWithdrawalReserveParams(initializationData, {
        minPct: initializationData.initialMinimumWithdrawalReservePercentageBps + 1,
        targetPct: initializationData.initialTargetWithdrawalReservePercentageBps + 2,
        minAmount: initializationData.initialMinimumWithdrawalReserveAmount + 5n,
        targetAmount: initializationData.initialTargetWithdrawalReserveAmount + 10n,
      });

      const prevMinPct = await yieldManager.minimumWithdrawalReservePercentageBps();
      const prevMinAmount = await yieldManager.minimumWithdrawalReserveAmount();
      const prevTargetPct = await yieldManager.targetWithdrawalReservePercentageBps();
      const prevTargetAmount = await yieldManager.targetWithdrawalReserveAmount();

      const tx = yieldManager.connect(securityCouncil).setWithdrawalReserveParameters(params);

      await expect(tx)
        .to.emit(yieldManager, "WithdrawalReserveParametersSet")
        .withArgs(
          prevMinPct,
          prevMinAmount,
          prevTargetPct,
          prevTargetAmount,
          params.minimumWithdrawalReservePercentageBps,
          params.minimumWithdrawalReserveAmount,
          params.targetWithdrawalReservePercentageBps,
          params.targetWithdrawalReserveAmount,
        );

      await tx;

      expect(await yieldManager.minimumWithdrawalReservePercentageBps()).to.equal(
        BigInt(params.minimumWithdrawalReservePercentageBps),
      );
      expect(await yieldManager.minimumWithdrawalReserveAmount()).to.equal(params.minimumWithdrawalReserveAmount);
      expect(await yieldManager.targetWithdrawalReservePercentageBps()).to.equal(
        BigInt(params.targetWithdrawalReservePercentageBps),
      );
      expect(await yieldManager.targetWithdrawalReserveAmount()).to.equal(params.targetWithdrawalReserveAmount);
    });
  });

  describe("getTotalSystemBalance", () => {
    const ONE_HUNDRED_FIFTY_ETHER = ethers.parseEther("150");

    it("Return correct value with 1000 ETH on L1MessageService only", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, ethers.toBeHex(ONE_THOUSAND_ETHER)]);

      const total = await yieldManager.getTotalSystemBalance();
      expect(total).to.equal(ONE_THOUSAND_ETHER);
    });

    it("Return correct value with 1000 ETH on L1MessageService and 150 ETH on YieldManager", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, ethers.toBeHex(ONE_THOUSAND_ETHER)]);
      const yieldManagerAddress = await yieldManager.getAddress();
      await ethers.provider.send("hardhat_setBalance", [yieldManagerAddress, ethers.toBeHex(ONE_HUNDRED_FIFTY_ETHER)]);

      const total = await yieldManager.getTotalSystemBalance();
      expect(total).to.equal(ONE_THOUSAND_ETHER + ONE_HUNDRED_FIFTY_ETHER);
    });
  });

  describe("getEffectiveMinimumWithdrawalReserve", () => {
    it("With 0 balance in total system, should return minimum amount", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, "0x0"]);
      await ethers.provider.send("hardhat_setBalance", [await yieldManager.getAddress(), "0x0"]);
      expect(await yieldManager.getEffectiveMinimumWithdrawalReserve()).to.equal(
        await yieldManager.minimumWithdrawalReserveAmount(),
      );
    });

    it("When total system balance > MINIMUM_AMOUNT / MINIMUM_WITHDRAWAL_PERCENTAGE, should return value dictated by MINIMUM_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [
        l1MessageServiceAddress,
        ethers.toBeHex(systemBalanceThreshold),
      ]);
      await ethers.provider.send("hardhat_setBalance", [
        await yieldManager.getAddress(),
        ethers.toBeHex(systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.getEffectiveMinimumWithdrawalReserve()).to.be.above(
        await yieldManager.minimumWithdrawalReserveAmount(),
      );
      expect(await yieldManager.getEffectiveMinimumWithdrawalReserve()).to.equal(
        ((await yieldManager.getTotalSystemBalance()) * (await yieldManager.minimumWithdrawalReservePercentageBps())) /
          MAX_BPS,
      );
    });
  });

  describe("getEffectiveTargetWithdrawalReserve", () => {
    it("With 0 balance in total system, should return target amount", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, "0x0"]);
      await ethers.provider.send("hardhat_setBalance", [await yieldManager.getAddress(), "0x0"]);
      expect(await yieldManager.getEffectiveTargetWithdrawalReserve()).to.equal(
        await yieldManager.targetWithdrawalReserveAmount(),
      );
    });

    it("When total system balance > TARGET_AMOUNT / TARGET_WITHDRAWAL_PERCENTAGE, should return value dictated by TARGET_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const targetPct = initializationData.initialTargetWithdrawalReservePercentageBps;
      const targetAmount = initializationData.initialTargetWithdrawalReserveAmount;
      const systemBalanceThreshold = (targetAmount * MAX_BPS) / BigInt(targetPct);
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [
        l1MessageServiceAddress,
        ethers.toBeHex(systemBalanceThreshold),
      ]);
      await ethers.provider.send("hardhat_setBalance", [
        await yieldManager.getAddress(),
        ethers.toBeHex(systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.getEffectiveTargetWithdrawalReserve()).to.be.above(
        await yieldManager.targetWithdrawalReserveAmount(),
      );
      expect(await yieldManager.getEffectiveTargetWithdrawalReserve()).to.equal(
        ((await yieldManager.getTotalSystemBalance()) * (await yieldManager.targetWithdrawalReservePercentageBps())) /
          MAX_BPS,
      );
    });
  });

  describe("getMinimumReserveDeficit", () => {
    it("With 0 balance in total system, should equal deficit from minimum amount", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, "0x0"]);
      await ethers.provider.send("hardhat_setBalance", [await yieldManager.getAddress(), "0x0"]);

      expect(await yieldManager.getMinimumReserveDeficit()).to.equal(
        await yieldManager.minimumWithdrawalReserveAmount(),
      );
    });

    it("With 0 L1MessageService balance and effective minimum dictated by percentage, should return deficit from value dictated by MINIMUM_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await ethers.provider.send("hardhat_setBalance", [
        await yieldManager.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      const effectiveMinimum = await yieldManager.getEffectiveMinimumWithdrawalReserve();
      const l1MessageServiceBalance = await ethers.provider.getBalance(await mockLineaRollup.getAddress());
      const expectedDeficit = effectiveMinimum - l1MessageServiceBalance;
      expect(await yieldManager.getMinimumReserveDeficit()).to.equal(expectedDeficit);
    });

    it("With no reserve deficit, should return 0", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await ethers.provider.send("hardhat_setBalance", [
        await mockLineaRollup.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.getMinimumReserveDeficit()).to.equal(0n);
    });
  });

  describe("getTargetReserveDeficit", () => {
    it("With 0 balance in total system, should deficit from target amount", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, "0x0"]);
      await ethers.provider.send("hardhat_setBalance", [await yieldManager.getAddress(), "0x0"]);
      expect(await yieldManager.getTargetReserveDeficit()).to.equal(await yieldManager.targetWithdrawalReserveAmount());
    });

    it("With 0 L1MessageService balance and effective target dictated by percentage, should return deficit from value dictated by TARGET_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const targetPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const targetAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (targetAmount * MAX_BPS) / BigInt(targetPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await ethers.provider.send("hardhat_setBalance", [
        await yieldManager.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      const effectiveTarget = await yieldManager.getEffectiveTargetWithdrawalReserve();
      const l1MessageServiceBalance = await ethers.provider.getBalance(await mockLineaRollup.getAddress());
      const expectedDeficit = effectiveTarget - l1MessageServiceBalance;
      expect(await yieldManager.getTargetReserveDeficit()).to.equal(expectedDeficit);
    });

    it("With no reserve deficit, should return 0", async () => {
      // Arrange
      const minPct = initializationData.initialTargetWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialTargetWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await ethers.provider.send("hardhat_setBalance", [
        await mockLineaRollup.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.getTargetReserveDeficit()).to.equal(0n);
    });
  });

  describe("isWithdrawalReserveBelowMinimum", () => {
    it("With 0 balance in total system and deficit dictated by minimum amount, return true", async () => {
      const l1MessageServiceAddress = await mockLineaRollup.getAddress();
      await ethers.provider.send("hardhat_setBalance", [l1MessageServiceAddress, "0x0"]);
      await ethers.provider.send("hardhat_setBalance", [await yieldManager.getAddress(), "0x0"]);

      expect(await yieldManager.isWithdrawalReserveBelowMinimum()).to.be.true;
    });

    it("With 0 L1MessageService balance and effective minimum dictated by percentage, should return true", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await ethers.provider.send("hardhat_setBalance", [
        await yieldManager.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.isWithdrawalReserveBelowMinimum()).to.be.true;
    });

    it("With no reserve deficit, should return false", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await ethers.provider.send("hardhat_setBalance", [
        await mockLineaRollup.getAddress(),
        ethers.toBeHex(2n * systemBalanceThreshold),
      ]);

      // Assert
      expect(await yieldManager.isWithdrawalReserveBelowMinimum()).to.be.false;
    });
  });

  describe("Safeguards on YieldProvider-scoped read functions", () => {
    it("getYieldProviderData should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.getYieldProviderData(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("userFunds should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.userFunds(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("isStakingPaused should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isStakingPaused(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("isOssified should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssified(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("isOssificationInitiated should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssificationInitiated(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("withdrawableValue should revert for unknown yield provider", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.withdrawableValue(unknownYieldProvider),
        "UnknownYieldProvider",
      );
    });

    it("withdrawableValue should return minimum of delegatecall return and userfunds", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.connect(nativeYieldOperator).setWithdrawableValueReturnVal(mockYieldProviderAddress, 1n);
      const withdrawableValue = await yieldManager.withdrawableValue.staticCall(mockYieldProviderAddress);
      expect(withdrawableValue).eq(0);
    });
  });

  describe("adding yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const requiredRole = await yieldManager.SET_YIELD_PROVIDER_ROLE();

      await expect(
        yieldManager
          .connect(nonAuthorizedAccount)
          .addYieldProvider(await mockYieldProvider.getAddress(), EMPTY_CALLDATA),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, requiredRole));
    });
    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).addYieldProvider(ZeroAddress, EMPTY_CALLDATA),
        "ZeroAddressNotAllowed",
      );
    });
    it("Should successfully add a yield provider, change state to expected and emit correct event", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const registration = buildMockYieldProviderRegistration();
      const providerAddress = await mockYieldProvider.getAddress();
      await yieldManager.setInitializeVendorContractsReturnVal(registration);

      const addYieldProviderTx = yieldManager
        .connect(securityCouncil)
        .addYieldProvider(providerAddress, EMPTY_CALLDATA);

      await expect(addYieldProviderTx)
        .to.emit(yieldManager, "YieldProviderAdded")
        .withArgs(
          providerAddress,
          registration.yieldProviderVendor,
          registration.primaryEntrypoint,
          registration.ossifiedEntrypoint,
          registration.usersFundsIncrement,
        );

      expect(await yieldManager.isYieldProviderKnown(providerAddress)).to.be.true;
      expect(await yieldManager.yieldProviderCount()).to.equal(1n);

      const yieldProviderData = await yieldManager.getYieldProviderData(providerAddress);
      expect(yieldProviderData.yieldProviderVendor).to.equal(BigInt(registration.yieldProviderVendor));
      expect(yieldProviderData.isStakingPaused).to.be.false;
      expect(yieldProviderData.isOssificationInitiated).to.be.false;
      expect(yieldProviderData.isOssified).to.be.false;
      expect(yieldProviderData.primaryEntrypoint).to.equal(registration.primaryEntrypoint);
      expect(yieldProviderData.ossifiedEntrypoint).to.equal(registration.ossifiedEntrypoint);
      expect(yieldProviderData.yieldProviderIndex).to.equal(1n);
      expect(yieldProviderData.userFunds).to.equal(0n);
      expect(yieldProviderData.yieldReportedCumulative).to.equal(0n);
      expect(yieldProviderData.lstLiabilityPrincipal).to.equal(0n);
      expect(yieldProviderData.lastReportedNegativeYield).to.equal(0n);

      expect(await yieldManager.isYieldProviderKnown(providerAddress)).to.equal(true);
      expect(await yieldManager.userFunds(providerAddress)).to.equal(0n);
      expect(await yieldManager.isStakingPaused(providerAddress)).to.be.false;
      expect(await yieldManager.isOssified(providerAddress)).to.be.false;
      expect(await yieldManager.isOssificationInitiated(providerAddress)).to.be.false;
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(0n);
    });
    it("First yield provider successfully added, should have yieldProviderIndex 1", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = await mockYieldProvider.getAddress();
      const registration = buildMockYieldProviderRegistration();
      await yieldManager.setInitializeVendorContractsReturnVal(registration);

      await yieldManager.connect(securityCouncil).addYieldProvider(providerAddress, EMPTY_CALLDATA);

      expect(await yieldManager.yieldProviderCount()).to.equal(1n);
      const yieldProviderData = await yieldManager.getYieldProviderData(providerAddress);
      expect(yieldProviderData.yieldProviderIndex).to.equal(1n);
    });
    it("Second yield provider successfully added, should have yieldProviderIndex 2", async () => {
      // Add first yield provider
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = await mockYieldProvider.getAddress();
      const registration = buildMockYieldProviderRegistration();
      await yieldManager.setInitializeVendorContractsReturnVal(registration);

      await yieldManager.connect(securityCouncil).addYieldProvider(providerAddress, EMPTY_CALLDATA);
      // Add second yield provider
      const mockYieldProvider2 = await deployMockYieldProvider();
      const providerAddress2 = await mockYieldProvider2.getAddress();
      const registration2 = buildMockYieldProviderRegistration();
      await yieldManager.setInitializeVendorContractsReturnVal(registration2);
      await yieldManager.connect(securityCouncil).addYieldProvider(providerAddress2, EMPTY_CALLDATA);
      // Assert
      expect(await yieldManager.isYieldProviderKnown(providerAddress2)).to.equal(true);
      expect(await yieldManager.yieldProviderCount()).to.equal(2n);
      expect(await yieldManager.yieldProviderByIndex(2)).to.equal(mockYieldProvider2);
    });
    it("Should revert when the yieldProvider has been previously added", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = await mockYieldProvider.getAddress();

      await yieldManager.connect(securityCouncil).addYieldProvider(providerAddress, EMPTY_CALLDATA);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).addYieldProvider(providerAddress, EMPTY_CALLDATA),
        "YieldProviderAlreadyAdded",
      );
    });
  });

  describe("removing yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.SET_YIELD_PROVIDER_ROLE();

      await expect(
        yieldManager.connect(nonAuthorizedAccount).removeYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, requiredRole));
    });

    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).removeYieldProvider(ZeroAddress, EMPTY_CALLDATA),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has not previously been added", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).removeYieldProvider(unknownYieldProvider, EMPTY_CALLDATA),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has remaining user funds", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const userFunds = 1n;
      await yieldManager.setYieldProviderUserFunds(mockYieldProviderAddress, userFunds);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).removeYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA),
        "YieldProviderHasRemainingFunds",
        [userFunds],
      );
    });

    it("Should successfully remove the yield provider, emit the correct event and wipe the yield provider state", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expect(yieldManager.connect(securityCouncil).removeYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA))
        .to.emit(yieldManager, "YieldProviderRemoved")
        .withArgs(mockYieldProviderAddress, false);

      expect(await yieldManager.isYieldProviderKnown(mockYieldProviderAddress)).to.be.false;
      expect(await yieldManager.yieldProviderCount()).to.equal(0n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.yieldProviderByIndex(0),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.getYieldProviderData(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.userFunds(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isStakingPaused(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssified(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssificationInitiated(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.pendingPermissionlessUnstake()).to.equal(0n);

      // Ensure state is wiped
      expect(await yieldManager.getYieldProviderVendor(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderIsStakingPaused(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderIsOssificationInitiated(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderIsOssified(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderPrimaryEntrypoint(mockYieldProviderAddress)).to.equal(ZeroAddress);
      expect(await yieldManager.getYieldProviderOssifiedEntrypoint(mockYieldProviderAddress)).to.equal(ZeroAddress);
      expect(await yieldManager.getYieldProviderIndex(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderUserFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderLastReportedNegativeYield(mockYieldProviderAddress)).to.equal(0n);
    });

    it("Adding three providers, then removing the first, should leave the middle provider with stable index", async () => {
      // Arrange
      const { mockYieldProviderAddress: providerOne } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerTwo } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerThree } = await addMockYieldProvider(yieldManager);

      const providerTwoDataBefore = await yieldManager.getYieldProviderData(providerTwo);
      expect(providerTwoDataBefore.yieldProviderIndex).to.equal(2n);

      // Act
      await yieldManager.connect(securityCouncil).removeYieldProvider(providerOne, EMPTY_CALLDATA);

      // Assert
      expect(await yieldManager.isYieldProviderKnown(providerOne)).to.equal(false);
      expect(await yieldManager.isYieldProviderKnown(providerOne)).to.equal(false);

      const providerTwoDataAfter = await yieldManager.getYieldProviderData(providerTwo);
      expect(providerTwoDataAfter.yieldProviderIndex).to.equal(2n);
      expect(await yieldManager.yieldProviderByIndex(2)).eq(providerTwo);
      expect(await yieldManager.isYieldProviderKnown(providerTwo)).to.equal(true);

      const providerThreeDataAfter = await yieldManager.getYieldProviderData(providerThree);
      expect(providerThreeDataAfter.yieldProviderIndex).to.equal(1n);
      expect(await yieldManager.yieldProviderByIndex(1)).eq(providerThree);
      expect(await yieldManager.isYieldProviderKnown(providerThree)).to.equal(true);

      expect(await yieldManager.yieldProviderCount()).equal(2n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.yieldProviderByIndex(3),
        "YieldProviderIndexOutOfBounds",
        [3n, 2n],
      );
    });
  });

  describe("emergency remove yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.SET_YIELD_PROVIDER_ROLE();

      await expect(
        yieldManager
          .connect(nonAuthorizedAccount)
          .emergencyRemoveYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA),
      ).to.be.revertedWith(buildAccessErrorMessage(nonAuthorizedAccount, requiredRole));
    });

    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).emergencyRemoveYieldProvider(ZeroAddress, EMPTY_CALLDATA),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has not previously been added", async () => {
      const unknownYieldProvider = ethers.Wallet.createRandom().address;

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.connect(securityCouncil).emergencyRemoveYieldProvider(unknownYieldProvider, EMPTY_CALLDATA),
        "UnknownYieldProvider",
      );
    });

    it("Should successfully remove the yield provider with outstanding userFunds, emit the correct event and wipe the yield provider state", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.setYieldProviderUserFunds(mockYieldProviderAddress, 1n);
      await yieldManager.setUserFundsInYieldProvidersTotal(1n);

      await expect(
        yieldManager.connect(securityCouncil).emergencyRemoveYieldProvider(mockYieldProviderAddress, EMPTY_CALLDATA),
      )
        .to.emit(yieldManager, "YieldProviderRemoved")
        .withArgs(mockYieldProviderAddress, true);

      expect(await yieldManager.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.isYieldProviderKnown(mockYieldProviderAddress)).to.be.false;
      expect(await yieldManager.yieldProviderCount()).to.equal(0n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.yieldProviderByIndex(0),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.getYieldProviderData(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.userFunds(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isStakingPaused(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssified(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.isOssificationInitiated(mockYieldProviderAddress),
        "UnknownYieldProvider",
      );

      // Ensure state is wiped
      expect(await yieldManager.getYieldProviderVendor(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderIsStakingPaused(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderIsOssificationInitiated(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderIsOssified(mockYieldProviderAddress)).to.equal(false);
      expect(await yieldManager.getYieldProviderPrimaryEntrypoint(mockYieldProviderAddress)).to.equal(ZeroAddress);
      expect(await yieldManager.getYieldProviderOssifiedEntrypoint(mockYieldProviderAddress)).to.equal(ZeroAddress);
      expect(await yieldManager.getYieldProviderIndex(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderUserFunds(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderYieldReportedCumulative(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderLstLiabilityPrincipal(mockYieldProviderAddress)).to.equal(0n);
      expect(await yieldManager.getYieldProviderLastReportedNegativeYield(mockYieldProviderAddress)).to.equal(0n);
    });

    it("Adding three providers, then removing the first, should leave the middle provider with stable index", async () => {
      const { mockYieldProviderAddress: providerOne } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerTwo } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerThree } = await addMockYieldProvider(yieldManager);

      const providerTwoDataBefore = await yieldManager.getYieldProviderData(providerTwo);
      expect(providerTwoDataBefore.yieldProviderIndex).to.equal(2n);

      await yieldManager.connect(securityCouncil).emergencyRemoveYieldProvider(providerOne, EMPTY_CALLDATA);

      const providerTwoDataAfter = await yieldManager.getYieldProviderData(providerTwo);
      expect(providerTwoDataAfter.yieldProviderIndex).to.equal(2n);

      const providerThreeDataAfter = await yieldManager.getYieldProviderData(providerThree);
      expect(providerThreeDataAfter.yieldProviderIndex).to.equal(1n);

      expect(await yieldManager.yieldProviderCount()).equal(2n);
    });
  });
});
