import { ethers, network } from "hardhat";
import { expect } from "chai";
import { toChecksumAddress } from "@ethereumjs/util";
import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import {
  L2MessageService,
  RollupRevenueVault,
  TestERC20,
  TokenBridge,
  TestDexSwapAdapter,
  TestDexSwapAdapter__factory,
} from "../../../typechain-types";
import { getRollupRevenueVaultAccountsFixture } from "./helpers/before";
import { deployRollupRevenueVaultFixture } from "./helpers/deploy";
import { ADDRESS_ZERO, EMPTY_CALLDATA, ONE_DAY_IN_SECONDS } from "../common/constants";
import {
  expectEvent,
  expectNoEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateRandomBytes,
} from "../common/helpers";
import { deployUpgradableFromFactory } from "../common/deployment";
import { ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE } from "./constants";

describe("RollupRevenueVault", () => {
  let rollupRevenueVault: RollupRevenueVault;
  let l2LineaToken: TestERC20;
  let tokenBridge: TokenBridge;
  let messageService: L2MessageService;
  let dexSwapAdapter: TestDexSwapAdapter;

  let admin: SignerWithAddress;
  let invoiceSubmitter: SignerWithAddress;
  let burner: SignerWithAddress;
  let invoicePaymentReceiver: SignerWithAddress;
  let l1LineaTokenBurner: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  const ONE_ETHER = ethers.parseEther("1");

  before(async () => {
    await network.provider.send("hardhat_reset");
    ({ admin, invoiceSubmitter, burner, invoicePaymentReceiver, l1LineaTokenBurner, nonAuthorizedAccount } =
      await loadFixture(getRollupRevenueVaultAccountsFixture));
  });

  beforeEach(async () => {
    ({ rollupRevenueVault, l2LineaToken, tokenBridge, messageService, dexSwapAdapter } = await loadFixture(
      deployRollupRevenueVaultFixture,
    ));
  });

  const sendEthToContract = async (value: bigint, data: string) => {
    return admin.sendTransaction({ to: await rollupRevenueVault.getAddress(), value, data });
  };
  describe("Fallback/Receive", () => {
    it("Should fail to send eth to the rollupRevenueVault contract through the receive when msg.value == 0", async () => {
      const value = ethers.parseEther("0");
      await expectRevertWithCustomError(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "NoEthSent");
    });

    it("Should send eth to the rollupRevenueVault contract through the receive", async () => {
      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);
    });

    it("Should fail to send eth to the rollupRevenueVault contract through the fallback function when msg.value == 0", async () => {
      const value = ethers.parseEther("0");
      await expectRevertWithCustomError(rollupRevenueVault, sendEthToContract(value, "0x1234"), "NoEthSent");
    });

    it("Should send eth to the rollupRevenueVault contract through the fallback function", async () => {
      const value = ethers.parseEther("1");
      await expectEvent(rollupRevenueVault, sendEthToContract(value, "0x1234"), "EthReceived", [value]);
    });
  });

  describe("initializeRolesAndStorageVariables", () => {
    it("Should revert when reinitializing twice", async () => {
      await expectRevertWithReason(
        rollupRevenueVault.initializeRolesAndStorageVariables(
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ),
        "Initializable: contract is already initialized",
      );
    });

    it("should revert if lastInvoiceDate is zero", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          0n,
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroTimestampNotAllowed");
    });

    it("should revert if lastInvoiceDate is in the future", async () => {
      const futureInvoiceDate = (await time.latest()) + ONE_DAY_IN_SECONDS;
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          futureInvoiceDate,
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "FutureInvoicesNotAllowed");
    });

    it("Should revert if defaultAdmin address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          ADDRESS_ZERO,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if invoiceSubmitter address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          ADDRESS_ZERO,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if burner address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          ADDRESS_ZERO,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if invoicePaymentReceiver address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          ADDRESS_ZERO,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if tokenBridge address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          ADDRESS_ZERO,
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if messageService address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          ADDRESS_ZERO,
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if l1LineaTokenBurner address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          ADDRESS_ZERO,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if lineaToken address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          ADDRESS_ZERO,
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if V3DexSwapAdapter contract address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          await time.latest(),
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          ADDRESS_ZERO,
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      await expectRevertWithCustomError(rollupRevenueVault, deployCall, "ZeroAddressNotAllowed");
    });

    it("should emit an event when initialized", async () => {
      const lastInvoiceDate = await time.latest();
      const contract = await deployUpgradableFromFactory(
        "RollupRevenueVault",
        [
          lastInvoiceDate,
          admin.address,
          invoiceSubmitter.address,
          burner.address,
          invoicePaymentReceiver.address,
          await tokenBridge.getAddress(),
          await messageService.getAddress(),
          l1LineaTokenBurner.address,
          await l2LineaToken.getAddress(),
          await dexSwapAdapter.getAddress(),
        ],
        {
          initializer: ROLLUP_REVENUE_VAULT_REINITIALIZE_SIGNATURE,
          unsafeAllow: ["constructor"],
        },
      );
      const receipt = await contract.deploymentTransaction()?.wait();
      const logs = receipt?.logs;

      expect(logs).to.have.lengthOf(7);

      const eventTopic = contract.interface.getEvent("RollupRevenueVaultInitialized");
      expect(eventTopic).to.not.be.null;
      const log = logs?.find((l) => l.topics[0] === eventTopic!.topicHash);
      expect(log).to.not.be.undefined;

      const event = contract.interface.parseLog(log!);
      expect(event).is.not.null;
      expect(event?.name).to.equal("RollupRevenueVaultInitialized");
      expect(event?.args.lastInvoiceDate).to.equal(lastInvoiceDate);
      expect(event?.args.invoicePaymentReceiver).to.equal(invoicePaymentReceiver.address);
      expect(event?.args.tokenBridge).to.equal(await tokenBridge.getAddress());
      expect(event?.args.messageService).to.equal(await messageService.getAddress());
      expect(event?.args.l1LineaTokenBurner).to.equal(l1LineaTokenBurner.address);
      expect(event?.args.lineaToken).to.equal(await l2LineaToken.getAddress());
      expect(event?.args.dexSwapAdapter).to.equal(await dexSwapAdapter.getAddress());
    });

    it("Should initialize correctly the contract", async () => {
      expect(await rollupRevenueVault.hasRole(await rollupRevenueVault.DEFAULT_ADMIN_ROLE(), admin.address)).to.equal(
        true,
      );
      expect(
        await rollupRevenueVault.hasRole(await rollupRevenueVault.INVOICE_SUBMITTER_ROLE(), invoiceSubmitter.address),
      ).to.equal(true);
      expect(await rollupRevenueVault.hasRole(await rollupRevenueVault.BURNER_ROLE(), burner.address)).to.equal(true);
      expect(await rollupRevenueVault.invoicePaymentReceiver()).to.equal(invoicePaymentReceiver.address);
      expect(await rollupRevenueVault.tokenBridge()).to.equal(await tokenBridge.getAddress());
      expect(await rollupRevenueVault.messageService()).to.equal(await messageService.getAddress());
      expect(await rollupRevenueVault.l1LineaTokenBurner()).to.equal(l1LineaTokenBurner.address);
      expect(await rollupRevenueVault.lineaToken()).to.equal(await l2LineaToken.getAddress());
      expect(await rollupRevenueVault.dexSwapAdapter()).to.equal(await dexSwapAdapter.getAddress());
    });
  });

  describe("submitInvoice", () => {
    it("Should revert if caller is not invoiceSubmitter", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).submitInvoice(startTimestamp, endTimestamp, 100n),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.INVOICE_SUBMITTER_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if timestamps not in sequence", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 2n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, 100n),
        "TimestampsNotInSequence",
      );
    });

    it("Should revert if endTimestamp is before startTimestamp", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp - 1n;

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, 100n),
        "EndTimestampMustBeGreaterThanStartTimestamp",
      );
    });

    it("Should revert if endTimestamp is in the future", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = (await time.latest()) + ONE_DAY_IN_SECONDS;

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, 100n),
        "FutureInvoicesNotAllowed",
      );
    });

    it("Should revert if amount is zero", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = lastInvoiceDate + BigInt(ONE_DAY_IN_SECONDS);

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, 0n),
        "ZeroInvoiceAmount",
      );
    });

    it("Should not submit invoice if the contract has no balance", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;

      const invoicePaymentReceiverBalanceBefore = await ethers.provider.getBalance(invoicePaymentReceiver.address);

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, invoiceAmount),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, 0n, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await ethers.provider.getBalance(invoicePaymentReceiver.address);
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore);
      expect(await rollupRevenueVault.lastInvoiceDate()).to.equal(endTimestamp);
    });

    it("should send the entire contract balance to the receiver when balanceAvailable < totalAmountCostsOwing", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;
      const balanceAvailable = ethers.parseEther("0.6");

      const invoicePaymentReceiverBalanceBefore = await ethers.provider.getBalance(invoicePaymentReceiver.address);

      await admin.sendTransaction({ to: await rollupRevenueVault.getAddress(), value: balanceAvailable });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, invoiceAmount),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, balanceAvailable, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await ethers.provider.getBalance(invoicePaymentReceiver.address);
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore + balanceAvailable);
      expect(await rollupRevenueVault.lastInvoiceDate()).to.equal(endTimestamp);
      expect(await rollupRevenueVault.invoiceArrears()).to.equal(invoiceAmount - balanceAvailable);
    });

    it("should send the entire totalAmountCostsOwing to the receiver when balanceAvailable >= totalAmountCostsOwing", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);
      const invoiceAmount = ONE_ETHER;
      const balanceAvailable = ethers.parseEther("1.5");

      const invoicePaymentReceiverBalanceBefore = await ethers.provider.getBalance(invoicePaymentReceiver.address);

      await admin.sendTransaction({ to: await rollupRevenueVault.getAddress(), value: balanceAvailable });

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, invoiceAmount),
        "InvoiceProcessed",
        [invoicePaymentReceiver.address, startTimestamp, endTimestamp, invoiceAmount, invoiceAmount],
      );

      const invoicePaymentReceiverBalanceAfter = await ethers.provider.getBalance(invoicePaymentReceiver.address);
      expect(invoicePaymentReceiverBalanceAfter).to.equal(invoicePaymentReceiverBalanceBefore + invoiceAmount);
      expect(await rollupRevenueVault.lastInvoiceDate()).to.equal(endTimestamp);
      expect(await rollupRevenueVault.invoiceArrears()).to.equal(0n);
    });
  });

  describe("updateInvoiceArrears", () => {
    it("Should revert if caller is not admin", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).updateInvoiceArrears(100n, lastInvoiceDate),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if lastInvoiceDate is before the current one", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoiceArrears(100n, lastInvoiceDate - 1n),
        "InvoiceDateTooOld",
      );
    });

    it("Should revert if lastInvoiceDate is in the future", async () => {
      const lastInvoiceDate = (await time.latest()) + ONE_DAY_IN_SECONDS;

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoiceArrears(100n, lastInvoiceDate),
        "FutureInvoicesNotAllowed",
      );
    });

    it("Should update invoice arrears", async () => {
      const newInvoiceArrears = 100n;
      const newInvoiceDate = await time.latest();

      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const lastInvoiceArrears = await rollupRevenueVault.invoiceArrears();

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoiceArrears(newInvoiceArrears, newInvoiceDate),
        "InvoiceArrearsUpdated",
        [lastInvoiceArrears, newInvoiceArrears, lastInvoiceDate, newInvoiceDate],
      );

      expect(await rollupRevenueVault.invoiceArrears()).to.equal(newInvoiceArrears);
      expect(await rollupRevenueVault.lastInvoiceDate()).to.equal(newInvoiceDate);
    });
  });

  describe("updateL1LineaTokenBurner", () => {
    it("Should revert if caller is not admin", async () => {
      const l1LineaTokenBurnerAddress = generateRandomBytes(20);
      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).updateL1LineaTokenBurner(l1LineaTokenBurnerAddress),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if l1LineaTokenBurner address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateL1LineaTokenBurner(ADDRESS_ZERO),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if l1LineaTokenBurner address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateL1LineaTokenBurner(l1LineaTokenBurner.address),
        "ExistingAddressTheSame",
      );
    });

    it("Should update l1LineaTokenBurner address", async () => {
      const randomAddress = toChecksumAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateL1LineaTokenBurner(randomAddress),
        "L1LineaTokenBurnerUpdated",
        [l1LineaTokenBurner.address, randomAddress],
      );

      expect(await rollupRevenueVault.l1LineaTokenBurner()).to.equal(randomAddress);
    });
  });

  describe("updateDexAdapter", () => {
    it("Should revert if caller is not admin", async () => {
      const dexAddress = generateRandomBytes(20);
      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).updateDexSwapAdapter(dexAddress),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if Dex address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateDexSwapAdapter(ADDRESS_ZERO),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if Dex adapter address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateDexSwapAdapter(await dexSwapAdapter.getAddress()),
        "ExistingAddressTheSame",
      );
    });

    it("Should update Dex address", async () => {
      const randomAddress = toChecksumAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateDexSwapAdapter(randomAddress),
        "DexSwapAdapterUpdated",
        [await dexSwapAdapter.getAddress(), randomAddress],
      );

      expect(await rollupRevenueVault.dexSwapAdapter()).to.equal(randomAddress);
    });
  });

  describe("updateInvoicePaymentReceiver", () => {
    it("Should revert if caller is not admin", async () => {
      const randomAddress = toChecksumAddress(generateRandomBytes(20));
      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).updateInvoicePaymentReceiver(randomAddress),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.DEFAULT_ADMIN_ROLE()).toLowerCase(),
      );
    });

    it("Should revert if invoicePaymentReceiver address is zero address", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoicePaymentReceiver(ADDRESS_ZERO),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if invoicePaymentReceiver address is already setup", async () => {
      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoicePaymentReceiver(invoicePaymentReceiver.address),
        "ExistingAddressTheSame",
      );
    });

    it("Should update invoicePaymentReceiver address", async () => {
      const randomAddress = toChecksumAddress(generateRandomBytes(20));
      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(admin).updateInvoicePaymentReceiver(randomAddress),
        "InvoicePaymentReceiverUpdated",
        [invoicePaymentReceiver.address, randomAddress],
      );

      expect(await rollupRevenueVault.invoicePaymentReceiver()).to.equal(randomAddress);
    });
  });

  describe("burnAndBridge", () => {
    const INITIAL_CONTRACT_BALANCE = ONE_ETHER;
    beforeEach(async () => {
      await admin.sendTransaction({ to: await rollupRevenueVault.getAddress(), value: INITIAL_CONTRACT_BALANCE });
    });

    it("Should revert if caller is not burner", async () => {
      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;

      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData("swap", [
        minLineaOut,
        deadline,
      ]);

      await expectRevertWithReason(
        rollupRevenueVault.connect(nonAuthorizedAccount).burnAndBridge(encodedSwapData),
        "AccessControl: account " +
          nonAuthorizedAccount.address.toLowerCase() +
          " is missing role " +
          (await rollupRevenueVault.BURNER_ROLE()).toLowerCase(),
      );
    });

    it("Should pay off arrears with no burning", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault
        .connect(invoiceSubmitter)
        .submitInvoice(startTimestamp, endTimestamp, ethers.parseEther("2.5"));

      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);

      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;
      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData("swap", [
        minLineaOut,
        deadline,
      ]);

      const expectedRemainingArrears = ethers.parseUnits("0.5");

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "ArrearsPaid",
        [ONE_ETHER, expectedRemainingArrears],
      );

      expect(await rollupRevenueVault.invoiceArrears()).equal(expectedRemainingArrears);
    });

    it("Should pay off arrears with no burning due to less than minimumFeeInWei", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault.connect(invoiceSubmitter).submitInvoice(startTimestamp, endTimestamp, ONE_ETHER * 2n);

      const minimumFee = await messageService.minimumFeeInWei();
      const expectedRemainingBalance = minimumFee - 1n;
      const ethToSend = ONE_ETHER + expectedRemainingBalance;

      await expectEvent(rollupRevenueVault, sendEthToContract(ethToSend, EMPTY_CALLDATA), "EthReceived", [ethToSend]);

      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;
      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData("swap", [
        minLineaOut,
        deadline,
      ]);

      await expectNoEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "EthBurntSwappedAndBridged",
      );

      expect(await rollupRevenueVault.invoiceArrears()).equal(0n);
      expect(await ethers.provider.getBalance(await rollupRevenueVault.getAddress())).equal(expectedRemainingBalance);
    });

    it("Should pay off arrears with burning", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault
        .connect(invoiceSubmitter)
        .submitInvoice(startTimestamp, endTimestamp, ethers.parseEther("1.5"));

      const value = ONE_ETHER;
      await expectEvent(rollupRevenueVault, sendEthToContract(value, EMPTY_CALLDATA), "EthReceived", [value]);

      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;
      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData("swap", [
        minLineaOut,
        deadline,
      ]);

      const minimumFee = await messageService.minimumFeeInWei();
      const arrears = await rollupRevenueVault.invoiceArrears();
      const balanceAvailable =
        (await ethers.provider.getBalance(rollupRevenueVault.getAddress())) - minimumFee - arrears;
      const ethToBurn = (balanceAvailable * 20n) / 100n;

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "EthBurntSwappedAndBridged",
        [ethToBurn, (balanceAvailable - ethToBurn) * 2n], // We mock the swap to return amountIn * 2
      );

      expect(await rollupRevenueVault.invoiceArrears()).equal(0n);
    });

    it("Should revert if swap call fails", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault
        .connect(invoiceSubmitter)
        .submitInvoice(startTimestamp, endTimestamp, ethers.parseEther("0.5"));

      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData(
        "testRevertSwap",
        [0, 0],
      );

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "DexSwapFailed",
      );
    });

    it("Should revert if swap returns insufficient linea tokens", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault
        .connect(invoiceSubmitter)
        .submitInvoice(startTimestamp, endTimestamp, ethers.parseEther("0.5"));

      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;

      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData(
        "testSwapInsufficientLineaTokensReceived",
        [minLineaOut, deadline],
      );

      await expectRevertWithCustomError(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "DexSwapFailed",
      );
    });

    it("Should burn ETH, swap to LINEA and bridge the tokens to L1 burner contract", async () => {
      const lastInvoiceDate = await rollupRevenueVault.lastInvoiceDate();
      const startTimestamp = lastInvoiceDate + 1n;
      const endTimestamp = startTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      await rollupRevenueVault
        .connect(invoiceSubmitter)
        .submitInvoice(startTimestamp, endTimestamp, ethers.parseEther("0.5"));

      const minimumFee = await messageService.minimumFeeInWei();
      const balanceAvailable = (await ethers.provider.getBalance(rollupRevenueVault.getAddress())) - minimumFee;

      const ethToBurn = (balanceAvailable * 20n) / 100n;

      const minLineaOut = 200n;
      const deadline = (await time.latest()) + ONE_DAY_IN_SECONDS;

      const encodedSwapData = TestDexSwapAdapter__factory.createInterface().encodeFunctionData("swap", [
        minLineaOut,
        deadline,
      ]);

      await expectEvent(
        rollupRevenueVault,
        rollupRevenueVault.connect(burner).burnAndBridge(encodedSwapData),
        "EthBurntSwappedAndBridged",
        [ethToBurn, (balanceAvailable - ethToBurn) * 2n], // We mock the swap to return amountIn * 2
      );
    });
  });
});
