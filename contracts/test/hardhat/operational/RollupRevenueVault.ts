import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, type Hex, encodeFunctionData, getAddress, parseEther, parseEventLogs, parseUnits } from "viem";

import { ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE } from "./constants.js";
import { getRollupRevenueVaultAccountsFixture } from "./helpers/before.js";
import { deployRollupRevenueVaultFixture } from "./helpers/deploy.js";
import { ADDRESS_ZERO, EMPTY_CALLDATA, ONE_DAY_IN_SECONDS, ONE_ETHER } from "../common/constants/index.js";
import { deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  expectNoEvent,
  generateRandomBytes,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
} from "../common/helpers/index.js";

import type { NetworkHelpers } from "@nomicfoundation/hardhat-network-helpers/types";
import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";

describe("RollupRevenueVault", () => {
  let rollupRevenueVault: ContractReturnType<"RollupRevenueVault">;
  let l2LineaToken: ContractReturnType<"TestERC20">;
  let tokenBridge: ContractReturnType<"TokenBridge">;
  let messageService: ContractReturnType<"L2MessageService">;
  let dexSwapAdapter: ContractReturnType<"TestDexSwapAdapter">;

  let admin: TestAccount;
  let invoiceSubmitter: TestAccount;
  let burner: TestAccount;
  let invoicePaymentReceiver: TestAccount;
  let l1LineaTokenBurner: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  let networkHelpers: NetworkHelpers;
  let publicClient: PublicClient;

  before(async () => {
    // Hardhat 2 also reset the chain here (`hardhat_reset`), which Hardhat 3's EDR does not support; the fixtures
    // below do not depend on a fresh chain.
    const connection = await network.getOrCreate();
    networkHelpers = connection.networkHelpers;
    publicClient = await connection.viem.getPublicClient();
    ({ admin, invoiceSubmitter, burner, invoicePaymentReceiver, l1LineaTokenBurner, nonAuthorizedAccount } =
      await networkHelpers.loadFixture(getRollupRevenueVaultAccountsFixture));
  });

  beforeEach(async () => {
    ({ rollupRevenueVault, l2LineaToken, tokenBridge, messageService, dexSwapAdapter } =
      await networkHelpers.loadFixture(deployRollupRevenueVaultFixture));
  });

  const sendEthToContract = async (value: bigint, data: Hex) => {
    return admin.sendTransaction({ to: rollupRevenueVault.address, value, data });
  };
  describe("Fallback/Receive", () => {
    it("Should fail to send eth to the rollupRevenueVault contract through the receive when msg.value == 0", async () => {
      const value = parseEther("0");
      await expectRevertWithCustomError(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "NoEthSent");
    });

    it("Should send eth to the rollupRevenueVault contract through the receive", async () => {
      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);
    });

    it("Should fail to send eth to the rollupRevenueVault contract through the fallback function when msg.value == 0", async () => {
      const value = parseEther("0");
      await expectRevertWithCustomError(rollupRevenueVault, sendEthToContract(value, "0x1234"), "NoEthSent");
    });

    it("Should send eth to the rollupRevenueVault contract through the fallback function", async () => {
      const value = parseEther("1");
      await expectEvent(rollupRevenueVault, sendEthToContract(value, "0x1234"), "EthReceived", [value]);
    });
  });

  describe("initializeRolesAndStorageVariables", () => {
    it("Should revert when reinitializing twice", async () => {
      await expectRevertWithReason(
        rollupRevenueVault.write.initializeRolesAndStorageVariables([
          BigInt(await networkHelpers.time.latest()),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          tokenBridge.address,
          messageService.address,
          l1LineaTokenBurner.address,
          l2LineaToken.address,
          dexSwapAdapter.address,
        ]),
        "Initializable: contract is already initialized",
      );
    });

    // Configuration for initialization arguments
    interface InitConfig {
      lastInvoiceDate: number | bigint;
      defaultAdmin: Address;
      invoiceSubmitter: Address;
      burner: Address;
      invoicePaymentReceiver: Address;
      tokenBridge: Address;
      messageService: Address;
      l1LineaTokenBurner: Address;
      lineaToken: Address;
      dexSwapAdapter: Address;
    }

    // Convert config to args array
    const toArgs = (config: InitConfig): unknown[] => [
      config.lastInvoiceDate,
      config.defaultAdmin,
      config.invoiceSubmitter,
      config.burner,
      config.invoicePaymentReceiver,
      config.tokenBridge,
      config.messageService,
      config.l1LineaTokenBurner,
      config.lineaToken,
      config.dexSwapAdapter,
    ];

    // Helper to create default config
    const createDefaultConfig = async (): Promise<InitConfig> => ({
      lastInvoiceDate: BigInt(await networkHelpers.time.latest()),
      defaultAdmin: admin.address,
      invoiceSubmitter: invoiceSubmitter.address,
      burner: burner.address,
      invoicePaymentReceiver: invoicePaymentReceiver.address,
      tokenBridge: tokenBridge.address,
      messageService: messageService.address,
      l1LineaTokenBurner: l1LineaTokenBurner.address,
      lineaToken: l2LineaToken.address,
      dexSwapAdapter: dexSwapAdapter.address,
    });

    // Helper to deploy with config
    const deployWithConfig = (config: InitConfig) =>
      deployUpgradableFromFactory("RollupRevenueVault", toArgs(config), {
        initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
        unsafeAllow: ["constructor"],
      });

    it("should revert if lastInvoiceDate is zero", async () => {
      const config = await createDefaultConfig();
      const deployCall = deployWithConfig({ ...config, lastInvoiceDate: 0n });
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroTimestampNotAllowed");
    });

    it("should revert if lastInvoiceDate is in the future", async () => {
      const config = await createDefaultConfig();
      const futureInvoiceDate = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);
      const deployCall = deployWithConfig({ ...config, lastInvoiceDate: futureInvoiceDate });
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "FutureInvoicesNotAllowed");
    });

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      description: string;
      field: keyof InitConfig;
    }> = [
      { description: "defaultAdmin", field: "defaultAdmin" },
      { description: "invoiceSubmitter", field: "invoiceSubmitter" },
      { description: "burner", field: "burner" },
      { description: "invoicePaymentReceiver", field: "invoicePaymentReceiver" },
      { description: "tokenBridge", field: "tokenBridge" },
      { description: "messageService", field: "messageService" },
      { description: "l1LineaTokenBurner", field: "l1LineaTokenBurner" },
      { description: "lineaToken", field: "lineaToken" },
      { description: "V3DexSwapAdapter contract", field: "dexSwapAdapter" },
    ];

    zeroAddressValidationCases.forEach(({ description, field }) => {
      it(`Should revert if ${description} address is zero address`, async () => {
        const config = await createDefaultConfig();
        const deployCall = deployWithConfig({ ...config, [field]: ADDRESS_ZERO });
        await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
      });
    });

    it("should emit an event when initialized", async () => {
      const lastInvoiceDate = BigInt(await networkHelpers.time.latest());
      const contract = await deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          lastInvoiceDate,
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          tokenBridge.address,
          messageService.address,
          l1LineaTokenBurner.address,
          l2LineaToken.address,
          dexSwapAdapter.address,
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      // The OZ Upgrades viem API does not expose the proxy deployment transaction: with automine it is the only
      // transaction of the latest block.
      const { transactions } = await publicClient.getBlock();
      const receipt = await publicClient.getTransactionReceipt({ hash: transactions[0] });
      expect(receipt.contractAddress && getAddress(receipt.contractAddress)).to.equal(getAddress(contract.address));
      // OZ 5 proxies (OZ Upgrades 4.x) also deploy their ProxyAdmin in this transaction (`OwnershipTransferred`);
      // the Hardhat 2 setup reused a shared ProxyAdmin, so only the proxy's own logs are counted.
      const logs = receipt.logs.filter((log) => getAddress(log.address) === getAddress(contract.address));

      expect(logs).to.have.lengthOf(7);

      const [event] = parseEventLogs({ abi: contract.abi, logs, eventName: "RollupRevenueVaultInitialized" });
      expect(event).to.not.be.undefined;
      expect(event.eventName).to.equal("RollupRevenueVaultInitialized");
      expect(event.args.lastInvoiceDate).to.equal(lastInvoiceDate);
      expect(event.args.invoicePaymentReceiver).to.equal(getAddress(invoicePaymentReceiver.address));
      expect(event.args.tokenBridge).to.equal(getAddress(tokenBridge.address));
      expect(event.args.messageService).to.equal(getAddress(messageService.address));
      expect(event.args.l1LineaTokenBurner).to.equal(getAddress(l1LineaTokenBurner.address));
      expect(event.args.lineaToken).to.equal(getAddress(l2LineaToken.address));
      expect(event.args.dexSwapAdapter).to.equal(getAddress(dexSwapAdapter.address));
    });

    it("Should initialize correctly the contract", async () => {
      expect(
        await rollupRevenueVault.read.hasRole([await rollupRevenueVault.read.DEFAULT_ADMIN_ROLE(), admin.address]),
      ).to.equal(true);
      expect(
        await rollupRevenueVault.read.hasRole([
          await rollupRevenueVault.read.INVOICE_SUBMITTER_ROLE(),
          invoiceSubmitter.address,
        ]),
      ).to.equal(true);
      expect(
        await rollupRevenueVault.read.hasRole([await rollupRevenueVault.read.BURNER_ROLE(), burner.address]),
      ).to.equal(true);
      expect(await rollupRevenueVault.read.invoicePaymentReceiver()).to.equal(
        getAddress(invoicePaymentReceiver.address),
      );
      expect(await rollupRevenueVault.read.tokenBridge()).to.equal(getAddress(tokenBridge.address));
      expect(await rollupRevenueVault.read.messageService()).to.equal(getAddress(messageService.address));
      expect(await rollupRevenueVault.read.l1LineaTokenBurner()).to.equal(getAddress(l1LineaTokenBurner.address));
      expect(await rollupRevenueVault.read.lineaToken()).to.equal(getAddress(l2LineaToken.address));
      expect(await rollupRevenueVault.read.dexSwapAdapter()).to.equal(getAddress(dexSwapAdapter.address));
    });
  });

  describe("submitInvoice", () => {
    it("Should revert if caller is not invoiceSubmitter", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithReason(
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, 100n], {
          account: nonAuthorizedAccount.account,
        }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.INVOICE_SUBMITTER_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if timestamps not in sequence", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 2n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, 100n], {
          account: invoiceSubmitter.account,
        }),
        "TimestampsNotInSequence",
      );
    });

    it("Should revert if endTimestamp is before startTimestamp", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp - 1n;

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, 100n], {
          account: invoiceSubmitter.account,
        }),
        "EndTimestampMustBeGreaterThanStartTimestamp",
      );
    });

    it("Should revert if endTimestamp is in the future", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, 100n], {
          account: invoiceSubmitter.account,
        }),
        "FutureInvoicesNotAllowed",
      );
    });

    it("Should revert if amount is zero", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = lastInvoiceDate + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, 0n], {
          account: invoiceSubmitter.account,
        }),
        "ZeroInvoiceAmount",
      );
    });

    it("Should not submit invoice if the contract has no balance", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;

      const invoicePaymentReceiverBalanceBefore = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, invoiceAmount], {
          account: invoiceSubmitter.account,
        }),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, 0n, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore);
      expect(await rollupRevenueVault.read.lastInvoiceDate()).to.equal(endTimestamp);
    });

    it("should send the entire contract balance to the receiver when balanceAvailable < totalAmountCostsOwing", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;
      const balanceAvailable = parseEther("0.6");

      const invoicePaymentReceiverBalanceBefore = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });

      await admin.sendTransaction({ to: rollupRevenueVault.address, value: balanceAvailable });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, invoiceAmount], {
          account: invoiceSubmitter.account,
        }),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, balanceAvailable, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore + balanceAvailable);
      expect(await rollupRevenueVault.read.lastInvoiceDate()).to.equal(endTimestamp);
      expect(await rollupRevenueVault.read.invoiceArrears()).to.equal(invoiceAmount - balanceAvailable);
    });

    it("should send the entire totalAmountCostsOwing to the receiver when balanceAvailable >= totalAmountCostsOwing", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;
      const balanceAvailable = parseEther("1.5");

      const invoicePaymentReceiverBalanceBefore = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });

      await admin.sendTransaction({ to: rollupRevenueVault.address, value: balanceAvailable });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, invoiceAmount], {
          account: invoiceSubmitter.account,
        }),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, invoiceAmount, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await publicClient.getBalance({
        address: invoicePaymentReceiver.address,
      });
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore + invoiceAmount);
      expect(await rollupRevenueVault.read.lastInvoiceDate()).to.equal(endTimestamp);
      expect(await rollupRevenueVault.read.invoiceArrears()).to.equal(0n);
    });
  });

  describe("updateInvoiceArrears", () => {
    it("Should revert if caller is not admin", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      await expectRevertWithReason(
        rollupRevenueVault.write.updateInvoiceArrears([100n, lastInvoiceDate], {
          account: nonAuthorizedAccount.account,
        }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if lastInvoiceDate is before the current one", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoiceArrears([100n, lastInvoiceDate - 1n], { account: admin.account }),
        "InvoiceDateTooOld",
      );
    });

    it("Should revert if lastInvoiceDate is in the future", async () => {
      const lastInvoiceDate = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoiceArrears([100n, lastInvoiceDate], { account: admin.account }),
        "FutureInvoicesNotAllowed",
      );
    });

    it("Should update invoice arrears", async () => {
      const newInvoiceArrears = 100n;
      const newInvoiceDate = BigInt(await networkHelpers.time.latest());

      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const lastInvoiceArrears = await rollupRevenueVault.read.invoiceArrears();

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoiceArrears([newInvoiceArrears, newInvoiceDate], { account: admin.account }),
        "InvoiceArrearsUpdated",
        [lastInvoiceArrears, newInvoiceArrears, lastInvoiceDate, newInvoiceDate],
      );

      expect(await rollupRevenueVault.read.invoiceArrears()).to.equal(newInvoiceArrears);
      expect(await rollupRevenueVault.read.lastInvoiceDate()).to.equal(newInvoiceDate);
    });
  });

  describe("updateL1LineaTokenBurner", () => {
    it("Should revert if caller is not admin", async () => {
      const l1LineaTokenBurnerAddress = generateRandomBytes(20);
      await expectRevertWithReason(
        rollupRevenueVault.write.updateL1LineaTokenBurner([l1LineaTokenBurnerAddress], {
          account: nonAuthorizedAccount.account,
        }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if l1LineaTokenBurner address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateL1LineaTokenBurner([ADDRESS_ZERO], { account: admin.account }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if l1LineaTokenBurner address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateL1LineaTokenBurner([l1LineaTokenBurner.address], { account: admin.account }),
        "ExistingAddressTheSame",
      );
    });

    it("Should update l1LineaTokenBurner address", async () => {
      const randomAddress = getAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.updateL1LineaTokenBurner([randomAddress], { account: admin.account }),
        "L1LineaTokenBurnerUpdated",
        [l1LineaTokenBurner.address, randomAddress],
      );

      expect(await rollupRevenueVault.read.l1LineaTokenBurner()).to.equal(randomAddress);
    });
  });

  describe("updateDexAdapter", () => {
    it("Should revert if caller is not admin", async () => {
      const dexAddress = generateRandomBytes(20);
      await expectRevertWithReason(
        rollupRevenueVault.write.updateDexSwapAdapter([dexAddress], { account: nonAuthorizedAccount.account }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if Dex address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateDexSwapAdapter([ADDRESS_ZERO], { account: admin.account }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if Dex adapter address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateDexSwapAdapter([dexSwapAdapter.address], { account: admin.account }),
        "ExistingAddressTheSame",
      );
    });

    it("Should update Dex address", async () => {
      const randomAddress = getAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.updateDexSwapAdapter([randomAddress], { account: admin.account }),
        "DexSwapAdapterUpdated",
        [dexSwapAdapter.address, randomAddress],
      );

      expect(await rollupRevenueVault.read.dexSwapAdapter()).to.equal(randomAddress);
    });
  });

  describe("updateInvoicePaymentReceiver", () => {
    it("Should revert if caller is not admin", async () => {
      const randomAddress = getAddress(generateRandomBytes(20));
      await expectRevertWithReason(
        rollupRevenueVault.write.updateInvoicePaymentReceiver([randomAddress], {
          account: nonAuthorizedAccount.account,
        }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if invoicePaymentReceiver address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoicePaymentReceiver([ADDRESS_ZERO], { account: admin.account }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if invoicePaymentReceiver address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoicePaymentReceiver([invoicePaymentReceiver.address], {
          account: admin.account,
        }),
        "ExistingAddressTheSame",
      );
    });

    it("Should update invoicePaymentReceiver address", async () => {
      const randomAddress = getAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.updateInvoicePaymentReceiver([randomAddress], { account: admin.account }),
        "InvoicePaymentReceiverUpdated",
        [invoicePaymentReceiver.address, randomAddress],
      );

      expect(await rollupRevenueVault.read.invoicePaymentReceiver()).to.equal(randomAddress);
    });
  });

  describe("burnAndBridge", () => {
    const INITIAL_CONTRACT_BALANCE = ONE_ETHER;
    beforeEach(async () => {
      await admin.sendTransaction({ to: rollupRevenueVault.address, value: INITIAL_CONTRACT_BALANCE });
    });

    it("Should revert if caller is not burner", async () => {
      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);

      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "swap",
        args: [minLineaOut, deadline],
      });

      await expectRevertWithReason(
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: nonAuthorizedAccount.account }),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.read.BURNER_ROLE()).toLowerCase(),
      );
    });

    it("Should pay off arrears with no burning", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, parseEther("2.5")], {
        account: invoiceSubmitter.account,
      });

      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);

      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);
      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "swap",
        args: [minLineaOut, deadline],
      });

      const expectedRemainingArrears = parseUnits("0.5", 18);

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "ArrearsPaid",
        [ONE_ETHER, expectedRemainingArrears],
      );

      expect(await rollupRevenueVault.read.invoiceArrears()).equal(expectedRemainingArrears);
    });

    it("Should pay off arrears with no burning due to less than minimumFeeInWei", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, ONE_ETHER * 2n], {
        account: invoiceSubmitter.account,
      });

      const minimumFee = await messageService.read.minimumFeeInWei();
      const expectedRemainingBalance = minimumFee - 1n;
      const ethToSend = ONE_ETHER + expectedRemainingBalance;

      await expectEvent(rollupRevenueVault, sendEthToContract(ethToSend, EMPTY_CALLDATA), "EthReceived", [ethToSend]);

      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);
      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "swap",
        args: [minLineaOut, deadline],
      });

      await expectNoEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "EthBurntSwappedAndBridged",
      );

      expect(await rollupRevenueVault.read.invoiceArrears()).equal(0n);
      expect(await publicClient.getBalance({ address: rollupRevenueVault.address })).equal(expectedRemainingBalance);
    });

    it("Should pay off arrears with burning", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, parseEther("1.5")], {
        account: invoiceSubmitter.account,
      });

      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);

      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);
      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "swap",
        args: [minLineaOut, deadline],
      });

      const minimumFee = await messageService.read.minimumFeeInWei();
      const arrears = await rollupRevenueVault.read.invoiceArrears();
      const balanceAvailable =
        (await publicClient.getBalance({ address: rollupRevenueVault.address })) - minimumFee - arrears;
      const ethToBurn = (balanceAvailable * 20n) / 100n;

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "EthBurntSwappedAndBridged",
        [ethToBurn, (balanceAvailable - ethToBurn) * 2n],
      );

      expect(await rollupRevenueVault.read.invoiceArrears()).equal(0n);
    });

    it("Should revert if swap call fails", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, parseEther("0.5")], {
        account: invoiceSubmitter.account,
      });

      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "testRevertSwap",
        args: [0n, 0n],
      });

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "DexSwapFailed",
      );
    });

    it("Should revert if swap returns insufficient linea tokens", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, parseEther("0.5")], {
        account: invoiceSubmitter.account,
      });

      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);

      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "testSwapInsufficientLineaTokensReceived",
        args: [minLineaOut, deadline],
      });

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "DexSwapFailed",
      );
    });

    it("Should burn ETH, swap to LINEA and bridge the tokens to L1 burner contract", async () => {
      const lastInvoiceDate = await rollupRevenueVault.read.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.write.submitInvoice([startTimestamp, endTimestamp, parseEther("0.5")], {
        account: invoiceSubmitter.account,
      });

      const minimumFee = await messageService.read.minimumFeeInWei();
      const balanceAvailable = (await publicClient.getBalance({ address: rollupRevenueVault.address })) - minimumFee;

      const ethToBurn = (balanceAvailable * 20n) / 100n;

      const minLineaOut = 200n;
      const deadline = BigInt(await networkHelpers.time.latest()) + BigInt(ONE_DAY_IN_SECONDS);

      const encodedSwapData = encodeFunctionData({
        abi: dexSwapAdapter.abi,
        functionName: "swap",
        args: [minLineaOut, deadline],
      });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.write.burnAndBridge([encodedSwapData], { account: burner.account }),
        "EthBurntSwappedAndBridged",
        [ethToBurn, (balanceAvailable - ethToBurn) * 2n],
      );
    });
  });
});
