import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Hex, pad, parseEther, stringToHex, zeroAddress } from "viem";

import { MINIMUM_FEE, EMPTY_CALLDATA, ONE_THOUSAND_ETHER, MAX_BPS, ZERO_VALUE } from "../../common/constants/index.js";
import {
  expectAccessControlRevert,
  expectHasRole,
  getAccountsFixture,
  type TestAccount,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
} from "../../common/helpers/index.js";
import {
  deployMockYieldProvider,
  deployYieldManagerForUnitTest,
  deployYieldManagerForUnitTestWithMutatedInitData,
} from "../helpers/deploy.js";
import {
  buildSetWithdrawalReserveParams,
  expectRevert,
  getBalance,
  randomAddress,
  setBalance,
} from "../helpers/index.js";
import { addMockYieldProvider, buildMockYieldProviderRegistration } from "../helpers/mocks.js";

import type { YieldManagerInitializationData } from "../helpers/types.js";
import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("YieldManager contract - basic operations", () => {
  let yieldManager: ContractReturnType<"TestYieldManager">;

  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;
  let nativeYieldOperator: TestAccount;
  let mockLinethRollup: ContractReturnType<"MockLinethRollup">;
  let initializationData: YieldManagerInitializationData;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      securityCouncil,
      operator: nonAuthorizedAccount,
      nativeYieldOperator,
    } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ yieldManager, initializationData, mockLinethRollup } =
      await networkHelpers.loadFixture(deployYieldManagerForUnitTest));
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: Hex) => {
      return nativeYieldOperator.sendTransaction({
        to: yieldManager.address,
        value: MINIMUM_FEE,
        data,
      });
    };

    it("Should fail to send eth to the yieldManager contract through the fallback function", async () => {
      await expectRevert(sendEthToContract("0x1234"));
    });

    it("Should successfully accept ETH via receive() fn", async () => {
      await sendEthToContract(EMPTY_CALLDATA);
    });

    it("Should decrement pendingPermissionlessUnstake when ETH is received", async () => {
      await yieldManager.write.setPendingPermissionlessUnstake([MINIMUM_FEE]);
      await sendEthToContract(EMPTY_CALLDATA);
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(BigInt(ZERO_VALUE));
    });
  });

  describe("Constructor", () => {
    it("Should successfully set the L1MessageService and emit the expected event", async () => {
      const { viem } = await network.getOrCreate();
      const l1MessageServiceAddress = mockLinethRollup.address;
      const { contract: deployedYieldManager, deploymentTransaction } = await viem.sendDeploymentTransaction(
        "TestYieldManager",
        [l1MessageServiceAddress],
      );
      await expectEvent(deployedYieldManager, Promise.resolve(deploymentTransaction.hash), "YieldManagerDeployed", [
        l1MessageServiceAddress,
      ]);
      expect(await deployedYieldManager.read.L1_MESSAGE_SERVICE()).to.equal(l1MessageServiceAddress);
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
      const mutatedInitData = cloneInitializationData({ defaultAdmin: zeroAddress });
      await expectRevertWithCustomError(
        yieldManager,
        deployYieldManagerForUnitTestWithMutatedInitData(mutatedInitData),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if a 0 address is provided for an l2YieldRecipient", async () => {
      const mutatedInitData = cloneInitializationData({ initialL2YieldRecipients: [zeroAddress] });
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
      await expectRevertWithReason(
        yieldManager.write.initialize([cloneInitializationData()]),
        "Initializable: contract is already initialized",
      );
    });

    it("Should emit the correct initialization event", async () => {
      // The proxy has no deployment transaction handle in viem: locate the initializing transaction from its event log.
      const { viem } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const [initializedLog] = await publicClient.getContractEvents({
        address: yieldManager.address,
        abi: yieldManager.abi,
        eventName: "YieldManagerInitialized",
        fromBlock: 0n,
      });
      expect(initializedLog).to.not.equal(undefined);
      await expectEvent(yieldManager, Promise.resolve(initializedLog.transactionHash), "YieldManagerInitialized", [
        pad(stringToHex("1.0"), { dir: "right", size: 8 }),
        initializationData.initialL2YieldRecipients,
      ]);
    });

    it("Should have the correct L1_MESSAGE_ADDRESS", async () => {
      expect(await yieldManager.read.L1_MESSAGE_SERVICE()).to.equal(mockLinethRollup.address);
    });

    it("Should have the initial l2YieldRecipients in state", async () => {
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      expect(await yieldManager.read.isL2YieldRecipientKnown([existingRecipient])).to.be.true;
    });

    it("Should revert for index 0 yield provider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.yieldProviderByIndex([0n]),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
    });

    it("Should not register the zero address as a known yield provider", async () => {
      expect(await yieldManager.read.isYieldProviderKnown([zeroAddress])).to.equal(false);
    });

    it("Should have the initial minimum reserve percentage in state", async () => {
      expect(await yieldManager.read.minimumWithdrawalReservePercentageBps()).to.equal(
        BigInt(initializationData.initialMinimumWithdrawalReservePercentageBps),
      );
    });

    it("Should have the initial minimum reserve amount in state", async () => {
      expect(await yieldManager.read.minimumWithdrawalReserveAmount()).to.equal(
        initializationData.initialMinimumWithdrawalReserveAmount,
      );
    });

    it("Should have the initial target reserve percentage in state", async () => {
      expect(await yieldManager.read.targetWithdrawalReservePercentageBps()).to.equal(
        BigInt(initializationData.initialTargetWithdrawalReservePercentageBps),
      );
    });

    it("Should have the initial target reserve amount in state", async () => {
      expect(await yieldManager.read.targetWithdrawalReserveAmount()).to.equal(
        initializationData.initialTargetWithdrawalReserveAmount,
      );
    });

    it("Should have initial userFundsInYieldProvidersTotal = 0", async () => {
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(BigInt(ZERO_VALUE));
    });

    it("Should have initial pendingPermissionlessUnstake = 0", async () => {
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(BigInt(ZERO_VALUE));
    });

    it("Should have initial yieldProviderCount = 0", async () => {
      expect(await yieldManager.read.yieldProviderCount()).to.equal(BigInt(ZERO_VALUE));
    });

    it("Should assign the YIELD_PROVIDER_STAKING_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.read.YIELD_PROVIDER_STAKING_ROLE();
      await expectHasRole(yieldManager, role, nativeYieldOperator);
    });

    it("Should assign the YIELD_PROVIDER_UNSTAKER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE();
      await expectHasRole(yieldManager, role, nativeYieldOperator);
    });

    it("Should assign the YIELD_REPORTER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.read.YIELD_REPORTER_ROLE();
      await expectHasRole(yieldManager, role, nativeYieldOperator);
    });

    it("Should assign the STAKING_PAUSE_CONTROLLER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.read.STAKING_PAUSE_CONTROLLER_ROLE();
      await expectHasRole(yieldManager, role, nativeYieldOperator);
    });

    it("Should assign the OSSIFICATION_PROCESSOR_ROLE to nativeYieldOperator address", async () => {
      const ossifierRole = await yieldManager.read.OSSIFICATION_PROCESSOR_ROLE();
      await expectHasRole(yieldManager, ossifierRole, nativeYieldOperator);
    });

    it("Should also assign operator roles to securityCouncil address", async () => {
      const roles = [
        await yieldManager.read.YIELD_PROVIDER_STAKING_ROLE(),
        await yieldManager.read.YIELD_PROVIDER_UNSTAKER_ROLE(),
        await yieldManager.read.STAKING_PAUSE_CONTROLLER_ROLE(),
        await yieldManager.read.OSSIFICATION_PROCESSOR_ROLE(),
        await yieldManager.read.YIELD_REPORTER_ROLE(),
      ];
      await Promise.all(roles.map(async (role) => await expectHasRole(yieldManager, role, securityCouncil)));
    });

    it("Should assign the OSSIFICATION_INITIATOR_ROLE to securityCouncil address", async () => {
      const ossifierRole = await yieldManager.read.OSSIFICATION_INITIATOR_ROLE();
      await expectHasRole(yieldManager, ossifierRole, securityCouncil);
    });

    it("Should not assign the OSSIFICATION_INITIATOR_ROLE to nativeYieldOperator address", async () => {
      const ossifierRole = await yieldManager.read.OSSIFICATION_INITIATOR_ROLE();
      expect(await yieldManager.read.hasRole([ossifierRole, nativeYieldOperator.address])).to.be.false;
    });

    it("Should assign the SET_YIELD_PROVIDER_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.SET_YIELD_PROVIDER_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the SET_L2_YIELD_RECIPIENT_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.SET_L2_YIELD_RECIPIENT_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the WITHDRAWAL_RESERVE_SETTER_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.WITHDRAWAL_RESERVE_SETTER_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should not assign the WITHDRAWAL_RESERVE_SETTER_ROLE to nativeYieldOperator address", async () => {
      const role = await yieldManager.read.WITHDRAWAL_RESERVE_SETTER_ROLE();
      expect(await yieldManager.read.hasRole([role, nativeYieldOperator.address])).to.be.false;
    });

    it("Should assign the PAUSE_ALL_ROLE to securityCouncil address", async () => {
      const pauseAllRole = await yieldManager.read.PAUSE_ALL_ROLE();
      await expectHasRole(yieldManager, pauseAllRole, securityCouncil);
    });

    it("Should assign the UNPAUSE_ALL_ROLE to securityCouncil address", async () => {
      const unpauseAllRole = await yieldManager.read.UNPAUSE_ALL_ROLE();
      await expectHasRole(yieldManager, unpauseAllRole, securityCouncil);
    });

    it("Should assign the PAUSE_NATIVE_YIELD_STAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.PAUSE_NATIVE_YIELD_STAKING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_STAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.UNPAUSE_NATIVE_YIELD_STAKING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the PAUSE_NATIVE_YIELD_UNSTAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.PAUSE_NATIVE_YIELD_UNSTAKING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the PAUSE_NATIVE_YIELD_REPORTING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.PAUSE_NATIVE_YIELD_REPORTING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });

    it("Should assign the UNPAUSE_NATIVE_YIELD_REPORTING_ROLE to securityCouncil address", async () => {
      const role = await yieldManager.read.UNPAUSE_NATIVE_YIELD_REPORTING_ROLE();
      await expectHasRole(yieldManager, role, securityCouncil);
    });
  });

  describe("Adding and removing L2YieldRecipients", () => {
    it("Should revert when adding if the caller does not have the SET_L2_YIELD_RECIPIENT_ROLE", async () => {
      const requiredRole = await yieldManager.read.SET_L2_YIELD_RECIPIENT_ROLE();
      await expectAccessControlRevert(
        yieldManager.write.addL2YieldRecipient([nonAuthorizedAccount.address], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should add the new l2YieldRecipient address and emit the correct event", async () => {
      const newRecipient = randomAddress();
      const addTx = yieldManager.write.addL2YieldRecipient([newRecipient], { account: securityCouncil.account });

      await expectEvent(yieldManager, addTx, "L2YieldRecipientAdded", [newRecipient]);

      expect(await yieldManager.read.isL2YieldRecipientKnown([newRecipient])).to.be.true;
    });

    it("Should revert if the address being added has already been added", async () => {
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.addL2YieldRecipient([existingRecipient], { account: securityCouncil.account }),
        "L2YieldRecipientAlreadyAdded",
      );
    });

    it("Should revert if the address being removed is unknown", async () => {
      const unknownRecipient = randomAddress();
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.removeL2YieldRecipient([unknownRecipient], { account: securityCouncil.account }),
        "UnknownL2YieldRecipient",
      );
    });

    it("Should revert when removing if the caller does not have the SET_L2_YIELD_RECIPIENT_ROLE", async () => {
      const requiredRole = await yieldManager.read.SET_L2_YIELD_RECIPIENT_ROLE();
      const existingRecipient = initializationData.initialL2YieldRecipients[0];
      await expectAccessControlRevert(
        yieldManager.write.removeL2YieldRecipient([existingRecipient], { account: nonAuthorizedAccount.account }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should remove the new l2YieldRecipient address and emit the correct event", async () => {
      const recipientToRemove = initializationData.initialL2YieldRecipients[0];
      const removeTx = yieldManager.write.removeL2YieldRecipient([recipientToRemove], {
        account: securityCouncil.account,
      });

      await expectEvent(yieldManager, removeTx, "L2YieldRecipientRemoved", [recipientToRemove]);

      expect(await yieldManager.read.isL2YieldRecipientKnown([recipientToRemove])).to.be.false;
    });
  });

  describe("Setting withdrawal reserve parameters", () => {
    it("Should revert set withdrawal reserve parameters when the caller does not have the WITHDRAWAL_RESERVE_SETTER_ROLE role", async () => {
      const role = await yieldManager.read.WITHDRAWAL_RESERVE_SETTER_ROLE();
      await expectAccessControlRevert(
        yieldManager.write.setWithdrawalReserveParameters([buildSetWithdrawalReserveParams(initializationData)], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        role,
      );
    });

    it("Should revert if minimum withdrawal percentage higher than 10000 bps", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.setWithdrawalReserveParameters(
          [buildSetWithdrawalReserveParams(initializationData, { minPct: 10001 })],
          { account: securityCouncil.account },
        ),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if target withdrawal percentage higher than 10000 bps", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.setWithdrawalReserveParameters(
          [buildSetWithdrawalReserveParams(initializationData, { targetPct: 10001 })],
          { account: securityCouncil.account },
        ),
        "BpsMoreThan10000",
      );
    });

    it("Should revert if minimum withdrawal reserve percentage > target", async () => {
      const target = initializationData.initialTargetWithdrawalReservePercentageBps;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.setWithdrawalReserveParameters(
          [buildSetWithdrawalReserveParams(initializationData, { minPct: target + 1, targetPct: target })],
          { account: securityCouncil.account },
        ),
        "TargetReservePercentageMustBeAboveMinimum",
      );
    });

    it("Should revert if minimum withdrawal amount > target", async () => {
      const target = initializationData.initialTargetWithdrawalReserveAmount;
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.setWithdrawalReserveParameters(
          [buildSetWithdrawalReserveParams(initializationData, { minAmount: target + 1n, targetAmount: target })],
          { account: securityCouncil.account },
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

      const prevMinPct = await yieldManager.read.minimumWithdrawalReservePercentageBps();
      const prevMinAmount = await yieldManager.read.minimumWithdrawalReserveAmount();
      const prevTargetPct = await yieldManager.read.targetWithdrawalReservePercentageBps();
      const prevTargetAmount = await yieldManager.read.targetWithdrawalReserveAmount();

      const tx = yieldManager.write.setWithdrawalReserveParameters([params], { account: securityCouncil.account });

      await expectEvent(yieldManager, tx, "WithdrawalReserveParametersSet", [
        prevMinPct,
        prevMinAmount,
        prevTargetPct,
        prevTargetAmount,
        BigInt(params.minimumWithdrawalReservePercentageBps),
        params.minimumWithdrawalReserveAmount,
        BigInt(params.targetWithdrawalReservePercentageBps),
        params.targetWithdrawalReserveAmount,
      ]);

      await tx;

      expect(await yieldManager.read.minimumWithdrawalReservePercentageBps()).to.equal(
        BigInt(params.minimumWithdrawalReservePercentageBps),
      );
      expect(await yieldManager.read.minimumWithdrawalReserveAmount()).to.equal(params.minimumWithdrawalReserveAmount);
      expect(await yieldManager.read.targetWithdrawalReservePercentageBps()).to.equal(
        BigInt(params.targetWithdrawalReservePercentageBps),
      );
      expect(await yieldManager.read.targetWithdrawalReserveAmount()).to.equal(params.targetWithdrawalReserveAmount);
    });
  });

  describe("getTotalSystemBalance", () => {
    const ONE_HUNDRED_FIFTY_ETHER = parseEther("150");

    it("Return correct value with 1000 ETH on L1MessageService only", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, ONE_THOUSAND_ETHER);

      const total = await yieldManager.read.getTotalSystemBalance();
      expect(total).to.equal(ONE_THOUSAND_ETHER);
    });

    it("Return correct value with 1000 ETH on L1MessageService and 150 ETH on YieldManager", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, ONE_THOUSAND_ETHER);
      const yieldManagerAddress = yieldManager.address;
      await setBalance(yieldManagerAddress, ONE_HUNDRED_FIFTY_ETHER);

      const total = await yieldManager.read.getTotalSystemBalance();
      expect(total).to.equal(ONE_THOUSAND_ETHER + ONE_HUNDRED_FIFTY_ETHER);
    });
  });

  describe("getEffectiveMinimumWithdrawalReserve", () => {
    it("With 0 balance in total system, should return minimum amount", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, 0n);
      await setBalance(yieldManager.address, 0n);
      expect(await yieldManager.read.getEffectiveMinimumWithdrawalReserve()).to.equal(
        await yieldManager.read.minimumWithdrawalReserveAmount(),
      );
    });

    it("When total system balance > MINIMUM_AMOUNT / MINIMUM_WITHDRAWAL_PERCENTAGE, should return value dictated by MINIMUM_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, systemBalanceThreshold);
      await setBalance(yieldManager.address, systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.getEffectiveMinimumWithdrawalReserve()).to.be.above(
        await yieldManager.read.minimumWithdrawalReserveAmount(),
      );
      expect(await yieldManager.read.getEffectiveMinimumWithdrawalReserve()).to.equal(
        ((await yieldManager.read.getTotalSystemBalance()) *
          BigInt(await yieldManager.read.minimumWithdrawalReservePercentageBps())) /
          MAX_BPS,
      );
    });
  });

  describe("getEffectiveTargetWithdrawalReserve", () => {
    it("With 0 balance in total system, should return target amount", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, 0n);
      await setBalance(yieldManager.address, 0n);
      expect(await yieldManager.read.getEffectiveTargetWithdrawalReserve()).to.equal(
        await yieldManager.read.targetWithdrawalReserveAmount(),
      );
    });

    it("When total system balance > TARGET_AMOUNT / TARGET_WITHDRAWAL_PERCENTAGE, should return value dictated by TARGET_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const targetPct = initializationData.initialTargetWithdrawalReservePercentageBps;
      const targetAmount = initializationData.initialTargetWithdrawalReserveAmount;
      const systemBalanceThreshold = (targetAmount * MAX_BPS) / BigInt(targetPct);
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, systemBalanceThreshold);
      await setBalance(yieldManager.address, systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.getEffectiveTargetWithdrawalReserve()).to.be.above(
        await yieldManager.read.targetWithdrawalReserveAmount(),
      );
      expect(await yieldManager.read.getEffectiveTargetWithdrawalReserve()).to.equal(
        ((await yieldManager.read.getTotalSystemBalance()) *
          BigInt(await yieldManager.read.targetWithdrawalReservePercentageBps())) /
          MAX_BPS,
      );
    });
  });

  describe("getMinimumReserveDeficit", () => {
    it("With 0 balance in total system, should equal deficit from minimum amount", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, 0n);
      await setBalance(yieldManager.address, 0n);

      expect(await yieldManager.read.getMinimumReserveDeficit()).to.equal(
        await yieldManager.read.minimumWithdrawalReserveAmount(),
      );
    });

    it("With 0 L1MessageService balance and effective minimum dictated by percentage, should return deficit from value dictated by MINIMUM_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await setBalance(yieldManager.address, 2n * systemBalanceThreshold);

      // Assert
      const effectiveMinimum = await yieldManager.read.getEffectiveMinimumWithdrawalReserve();
      const l1MessageServiceBalance = await getBalance({ address: mockLinethRollup.address });
      const expectedDeficit = effectiveMinimum - l1MessageServiceBalance;
      expect(await yieldManager.read.getMinimumReserveDeficit()).to.equal(expectedDeficit);
    });

    it("With no reserve deficit, should return 0", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await setBalance(mockLinethRollup.address, 2n * systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.getMinimumReserveDeficit()).to.equal(0n);
    });
  });

  describe("getTargetReserveDeficit", () => {
    it("With 0 balance in total system, should deficit from target amount", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, 0n);
      await setBalance(yieldManager.address, 0n);
      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(
        await yieldManager.read.targetWithdrawalReserveAmount(),
      );
    });

    it("With 0 L1MessageService balance and effective target dictated by percentage, should return deficit from value dictated by TARGET_WITHDRAWAL_PERCENTAGE", async () => {
      // Arrange
      const targetPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const targetAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (targetAmount * MAX_BPS) / BigInt(targetPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await setBalance(yieldManager.address, 2n * systemBalanceThreshold);

      // Assert
      const effectiveTarget = await yieldManager.read.getEffectiveTargetWithdrawalReserve();
      const l1MessageServiceBalance = await getBalance({ address: mockLinethRollup.address });
      const expectedDeficit = effectiveTarget - l1MessageServiceBalance;
      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(expectedDeficit);
    });

    it("With no reserve deficit, should return 0", async () => {
      // Arrange
      const minPct = initializationData.initialTargetWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialTargetWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await setBalance(mockLinethRollup.address, 2n * systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.getTargetReserveDeficit()).to.equal(0n);
    });
  });

  describe("isWithdrawalReserveBelowMinimum", () => {
    it("With 0 balance in total system and deficit dictated by minimum amount, return true", async () => {
      const l1MessageServiceAddress = mockLinethRollup.address;
      await setBalance(l1MessageServiceAddress, 0n);
      await setBalance(yieldManager.address, 0n);

      expect(await yieldManager.read.isWithdrawalReserveBelowMinimum()).to.be.true;
    });

    it("With 0 L1MessageService balance and effective minimum dictated by percentage, should return true", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - no funds on L1MessageService -> Deficit
      await setBalance(yieldManager.address, 2n * systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.isWithdrawalReserveBelowMinimum()).to.be.true;
    });

    it("With no reserve deficit, should return false", async () => {
      // Arrange
      const minPct = initializationData.initialMinimumWithdrawalReservePercentageBps;
      const minAmount = initializationData.initialMinimumWithdrawalReserveAmount;
      const systemBalanceThreshold = (minAmount * MAX_BPS) / BigInt(minPct);
      // Arrange - All funds on L1MessageService -> No deficit
      await setBalance(mockLinethRollup.address, 2n * systemBalanceThreshold);

      // Assert
      expect(await yieldManager.read.isWithdrawalReserveBelowMinimum()).to.be.false;
    });
  });

  describe("Safeguards on YieldProvider-scoped read functions", () => {
    it("getYieldProviderData should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.getYieldProviderData([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("userFunds should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.userFunds([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("isStakingPaused should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isStakingPaused([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("isOssified should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssified([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("isOssificationInitiated should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssificationInitiated([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("withdrawableValue should revert for unknown yield provider", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.withdrawableValue([unknownYieldProvider]),
        "UnknownYieldProvider",
      );
    });

    it("withdrawableValue should return minimum of delegatecall return and userfunds", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);
      await yieldManager.write.setWithdrawableValueReturnVal([mockYieldProviderAddress, 1n], {
        account: nativeYieldOperator.account,
      });
      const withdrawableValue = (await yieldManager.simulate.withdrawableValue([mockYieldProviderAddress])).result;
      expect(withdrawableValue).eq(0n);
    });
  });

  describe("adding yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const requiredRole = await yieldManager.read.SET_YIELD_PROVIDER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.addYieldProvider([mockYieldProvider.address, EMPTY_CALLDATA], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });
    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.addYieldProvider([zeroAddress, EMPTY_CALLDATA], { account: securityCouncil.account }),
        "ZeroAddressNotAllowed",
      );
    });
    it("Should successfully add a yield provider, change state to expected and emit correct event", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const registration = buildMockYieldProviderRegistration();
      const providerAddress = mockYieldProvider.address;
      await yieldManager.write.setInitializeVendorContractsReturnVal([registration]);

      const addYieldProviderTx = yieldManager.write.addYieldProvider([providerAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });

      await expectEvent(yieldManager, addYieldProviderTx, "YieldProviderAdded", [
        providerAddress,
        registration.yieldProviderVendor,
        registration.primaryEntrypoint,
        registration.ossifiedEntrypoint,
        registration.usersFundsIncrement,
      ]);

      expect(await yieldManager.read.isYieldProviderKnown([providerAddress])).to.be.true;
      expect(await yieldManager.read.yieldProviderCount()).to.equal(1n);

      const yieldProviderData = await yieldManager.read.getYieldProviderData([providerAddress]);
      expect(yieldProviderData.yieldProviderVendor).to.equal(registration.yieldProviderVendor);
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

      expect(await yieldManager.read.isYieldProviderKnown([providerAddress])).to.equal(true);
      expect(await yieldManager.read.userFunds([providerAddress])).to.equal(0n);
      expect(await yieldManager.read.isStakingPaused([providerAddress])).to.be.false;
      expect(await yieldManager.read.isOssified([providerAddress])).to.be.false;
      expect(await yieldManager.read.isOssificationInitiated([providerAddress])).to.be.false;
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(0n);
    });
    it("First yield provider successfully added, should have yieldProviderIndex 1", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = mockYieldProvider.address;
      const registration = buildMockYieldProviderRegistration();
      await yieldManager.write.setInitializeVendorContractsReturnVal([registration]);

      await yieldManager.write.addYieldProvider([providerAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });

      expect(await yieldManager.read.yieldProviderCount()).to.equal(1n);
      const yieldProviderData = await yieldManager.read.getYieldProviderData([providerAddress]);
      expect(yieldProviderData.yieldProviderIndex).to.equal(1n);
    });
    it("Second yield provider successfully added, should have yieldProviderIndex 2", async () => {
      // Add first yield provider
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = mockYieldProvider.address;
      const registration = buildMockYieldProviderRegistration();
      await yieldManager.write.setInitializeVendorContractsReturnVal([registration]);

      await yieldManager.write.addYieldProvider([providerAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      // Add second yield provider
      const mockYieldProvider2 = await deployMockYieldProvider();
      const providerAddress2 = mockYieldProvider2.address;
      const registration2 = buildMockYieldProviderRegistration();
      await yieldManager.write.setInitializeVendorContractsReturnVal([registration2]);
      await yieldManager.write.addYieldProvider([providerAddress2, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });
      // Assert
      expect(await yieldManager.read.isYieldProviderKnown([providerAddress2])).to.equal(true);
      expect(await yieldManager.read.yieldProviderCount()).to.equal(2n);
      expect(await yieldManager.read.yieldProviderByIndex([2n])).to.equal(mockYieldProvider2.address);
    });
    it("Should revert when the yieldProvider has been previously added", async () => {
      const mockYieldProvider = await deployMockYieldProvider();
      const providerAddress = mockYieldProvider.address;

      await yieldManager.write.addYieldProvider([providerAddress, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.addYieldProvider([providerAddress, EMPTY_CALLDATA], { account: securityCouncil.account }),
        "YieldProviderAlreadyAdded",
      );
    });
  });

  describe("removing yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.SET_YIELD_PROVIDER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.removeYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.removeYieldProvider([zeroAddress, EMPTY_CALLDATA], { account: securityCouncil.account }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has not previously been added", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.removeYieldProvider([unknownYieldProvider, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has remaining user funds", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const userFunds = 1n;
      await yieldManager.write.setYieldProviderUserFunds([mockYieldProviderAddress, userFunds]);

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.removeYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "YieldProviderHasRemainingFunds",
        [userFunds],
      );
    });

    it("Should successfully remove the yield provider, emit the correct event and wipe the yield provider state", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await expectEvent(
        yieldManager,
        yieldManager.write.removeYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "YieldProviderRemoved",
        [mockYieldProviderAddress, false],
      );

      expect(await yieldManager.read.isYieldProviderKnown([mockYieldProviderAddress])).to.be.false;
      expect(await yieldManager.read.yieldProviderCount()).to.equal(0n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.yieldProviderByIndex([0n]),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.getYieldProviderData([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.userFunds([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isStakingPaused([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssified([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssificationInitiated([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.pendingPermissionlessUnstake()).to.equal(0n);

      // Ensure state is wiped
      expect(await yieldManager.read.getYieldProviderVendor([mockYieldProviderAddress])).to.equal(0);
      expect(await yieldManager.read.getYieldProviderIsStakingPaused([mockYieldProviderAddress])).to.equal(false);
      expect(await yieldManager.read.getYieldProviderIsOssificationInitiated([mockYieldProviderAddress])).to.equal(
        false,
      );
      expect(await yieldManager.read.getYieldProviderIsOssified([mockYieldProviderAddress])).to.equal(false);
      expect(await yieldManager.read.getYieldProviderPrimaryEntrypoint([mockYieldProviderAddress])).to.equal(
        zeroAddress,
      );
      expect(await yieldManager.read.getYieldProviderOssifiedEntrypoint([mockYieldProviderAddress])).to.equal(
        zeroAddress,
      );
      expect(await yieldManager.read.getYieldProviderIndex([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderUserFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderLastReportedNegativeYield([mockYieldProviderAddress])).to.equal(
        0n,
      );
    });

    it("Adding three providers, then removing the first, should leave the middle provider with stable index", async () => {
      // Arrange
      const { mockYieldProviderAddress: providerOne } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerTwo } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerThree } = await addMockYieldProvider(yieldManager);

      const providerTwoDataBefore = await yieldManager.read.getYieldProviderData([providerTwo]);
      expect(providerTwoDataBefore.yieldProviderIndex).to.equal(2n);

      // Act
      await yieldManager.write.removeYieldProvider([providerOne, EMPTY_CALLDATA], { account: securityCouncil.account });

      // Assert
      expect(await yieldManager.read.isYieldProviderKnown([providerOne])).to.equal(false);
      expect(await yieldManager.read.isYieldProviderKnown([providerOne])).to.equal(false);

      const providerTwoDataAfter = await yieldManager.read.getYieldProviderData([providerTwo]);
      expect(providerTwoDataAfter.yieldProviderIndex).to.equal(2n);
      expect(await yieldManager.read.yieldProviderByIndex([2n])).eq(providerTwo);
      expect(await yieldManager.read.isYieldProviderKnown([providerTwo])).to.equal(true);

      const providerThreeDataAfter = await yieldManager.read.getYieldProviderData([providerThree]);
      expect(providerThreeDataAfter.yieldProviderIndex).to.equal(1n);
      expect(await yieldManager.read.yieldProviderByIndex([1n])).eq(providerThree);
      expect(await yieldManager.read.isYieldProviderKnown([providerThree])).to.equal(true);

      expect(await yieldManager.read.yieldProviderCount()).equal(2n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.yieldProviderByIndex([3n]),
        "YieldProviderIndexOutOfBounds",
        [3n, 2n],
      );
    });
  });

  describe("emergency remove yield providers", () => {
    it("Should revert when the caller does not have the SET_YIELD_PROVIDER_ROLE role", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      const requiredRole = await yieldManager.read.SET_YIELD_PROVIDER_ROLE();

      await expectAccessControlRevert(
        yieldManager.write.emergencyRemoveYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
          account: nonAuthorizedAccount.account,
        }),
        nonAuthorizedAccount,
        requiredRole,
      );
    });

    it("Should revert when 0 address is provided for the _yieldProvider", async () => {
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.emergencyRemoveYieldProvider([zeroAddress, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "UnknownYieldProvider",
      );
    });

    it("Should revert when the yield provider has not previously been added", async () => {
      const unknownYieldProvider = randomAddress();

      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.write.emergencyRemoveYieldProvider([unknownYieldProvider, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "UnknownYieldProvider",
      );
    });

    it("Should successfully remove the yield provider with outstanding userFunds, emit the correct event and wipe the yield provider state", async () => {
      const { mockYieldProviderAddress } = await addMockYieldProvider(yieldManager);

      await yieldManager.write.setYieldProviderUserFunds([mockYieldProviderAddress, 1n]);
      await yieldManager.write.setUserFundsInYieldProvidersTotal([1n]);

      await expectEvent(
        yieldManager,
        yieldManager.write.emergencyRemoveYieldProvider([mockYieldProviderAddress, EMPTY_CALLDATA], {
          account: securityCouncil.account,
        }),
        "YieldProviderRemoved",
        [mockYieldProviderAddress, true],
      );

      expect(await yieldManager.read.userFundsInYieldProvidersTotal()).to.equal(0n);
      expect(await yieldManager.read.isYieldProviderKnown([mockYieldProviderAddress])).to.be.false;
      expect(await yieldManager.read.yieldProviderCount()).to.equal(0n);
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.yieldProviderByIndex([0n]),
        "YieldProviderIndexOutOfBounds",
        [0n, 0n],
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.getYieldProviderData([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.userFunds([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isStakingPaused([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssified([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );
      await expectRevertWithCustomError(
        yieldManager,
        yieldManager.read.isOssificationInitiated([mockYieldProviderAddress]),
        "UnknownYieldProvider",
      );

      // Ensure state is wiped
      expect(await yieldManager.read.getYieldProviderVendor([mockYieldProviderAddress])).to.equal(0);
      expect(await yieldManager.read.getYieldProviderIsStakingPaused([mockYieldProviderAddress])).to.equal(false);
      expect(await yieldManager.read.getYieldProviderIsOssificationInitiated([mockYieldProviderAddress])).to.equal(
        false,
      );
      expect(await yieldManager.read.getYieldProviderIsOssified([mockYieldProviderAddress])).to.equal(false);
      expect(await yieldManager.read.getYieldProviderPrimaryEntrypoint([mockYieldProviderAddress])).to.equal(
        zeroAddress,
      );
      expect(await yieldManager.read.getYieldProviderOssifiedEntrypoint([mockYieldProviderAddress])).to.equal(
        zeroAddress,
      );
      expect(await yieldManager.read.getYieldProviderIndex([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderUserFunds([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderYieldReportedCumulative([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderLstLiabilityPrincipal([mockYieldProviderAddress])).to.equal(0n);
      expect(await yieldManager.read.getYieldProviderLastReportedNegativeYield([mockYieldProviderAddress])).to.equal(
        0n,
      );
    });

    it("Adding three providers, then removing the first, should leave the middle provider with stable index", async () => {
      const { mockYieldProviderAddress: providerOne } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerTwo } = await addMockYieldProvider(yieldManager);
      const { mockYieldProviderAddress: providerThree } = await addMockYieldProvider(yieldManager);

      const providerTwoDataBefore = await yieldManager.read.getYieldProviderData([providerTwo]);
      expect(providerTwoDataBefore.yieldProviderIndex).to.equal(2n);

      await yieldManager.write.emergencyRemoveYieldProvider([providerOne, EMPTY_CALLDATA], {
        account: securityCouncil.account,
      });

      const providerTwoDataAfter = await yieldManager.read.getYieldProviderData([providerTwo]);
      expect(providerTwoDataAfter.yieldProviderIndex).to.equal(2n);

      const providerThreeDataAfter = await yieldManager.read.getYieldProviderData([providerThree]);
      expect(providerThreeDataAfter.yieldProviderIndex).to.equal(1n);

      expect(await yieldManager.read.yieldProviderCount()).equal(2n);
    });
  });
});
