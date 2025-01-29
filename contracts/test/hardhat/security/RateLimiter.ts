import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers, upgrades } from "hardhat";
import { TestRateLimiter } from "../../../typechain-types";
import {
  DEFAULT_ADMIN_ROLE,
  INITIALIZED_ERROR_MESSAGE,
  ONE_DAY_IN_SECONDS,
  RATE_LIMIT_SETTER_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
} from "../common/constants";
import { deployUpgradableFromFactory } from "../common/deployment";
import {
  buildAccessErrorMessage,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
} from "../common/helpers";

describe("Rate limiter", () => {
  let testRateLimiter: TestRateLimiter;
  let defaultAdmin: SignerWithAddress;
  let resetAdmin: SignerWithAddress;

  const ONE_HUNDRED_ETH = ethers.parseUnits("100", 18);
  const NINE_HUNDRED_ETH = ethers.parseUnits("900", 18);
  const TOTAL_ETH = ONE_HUNDRED_ETH + NINE_HUNDRED_ETH;

  async function deployTestRateLimiterFixture(): Promise<TestRateLimiter> {
    return deployUpgradableFromFactory("TestRateLimiter", [ONE_DAY_IN_SECONDS, TOTAL_ETH], {
      initializer: "initialize(uint256,uint256)",
    }) as unknown as Promise<TestRateLimiter>;
  }

  before(async () => {
    [defaultAdmin, resetAdmin] = await ethers.getSigners();
  });

  beforeEach(async () => {
    testRateLimiter = await loadFixture(deployTestRateLimiterFixture);
    await testRateLimiter.grantRole(RATE_LIMIT_SETTER_ROLE, resetAdmin.address);
    await testRateLimiter.grantRole(USED_RATE_LIMIT_RESETTER_ROLE, resetAdmin.address);
  });

  async function assertCurrentPeriodAmount(expectedAmount: bigint) {
    expect(await testRateLimiter.currentPeriodAmountInWei()).to.equal(expectedAmount);
  }

  async function resetCurrentPeriodAmountByAccount(admin: SignerWithAddress) {
    await testRateLimiter.connect(admin).resetAmountUsedInPeriod();
  }

  async function withdrawAndExpectAmount(amount: bigint, expectedAmount: bigint) {
    await testRateLimiter.withdrawSomeAmount(amount);
    expect(await testRateLimiter.currentPeriodAmountInWei()).to.equal(expectedAmount);
  }

  describe("Initialization checks", () => {
    it("Deployer has default admin role", async () => {
      expect(await testRateLimiter.hasRole(DEFAULT_ADMIN_ROLE, defaultAdmin.address)).to.be.true;
    });

    it("fails to initialize when not initialising", async () => {
      await expectRevertWithReason(
        testRateLimiter.tryInitialize(ONE_DAY_IN_SECONDS, ONE_HUNDRED_ETH),
        INITIALIZED_ERROR_MESSAGE,
      );
    });

    it("fails to initialize when the limit is zero", async () => {
      const upgradeCall = deployUpgradableFromFactory("TestRateLimiter", [ONE_DAY_IN_SECONDS, 0], {
        initializer: "initialize(uint256,uint256)",
      });

      await expectRevertWithCustomError(testRateLimiter, upgradeCall, "LimitIsZero");
    });

    it("Should emit a RateLimitInitialized when initializing", async () => {
      const RateLimitInitializedEvent = "0x8f805c372b66240792580418b7328c0c554ae235f0932475c51b026887fe26a9";

      const limitInWei = ONE_HUNDRED_ETH;

      const blockNumBefore = await ethers.provider.getBlockNumber();
      const blockBefore = await ethers.provider.getBlock(blockNumBefore);

      if (!blockBefore) {
        throw "blockBefore null";
      }

      const currentTimestamp = blockBefore.timestamp + 1;
      const currentPeriodEnd = currentTimestamp + ONE_DAY_IN_SECONDS;

      const factory = await ethers.getContractFactory("TestRateLimiter");
      const contract = await upgrades.deployProxy(factory, [ONE_DAY_IN_SECONDS, ONE_HUNDRED_ETH]);
      await contract.waitForDeployment();

      const receipt = await ethers.provider.getTransactionReceipt(contract.deploymentTransaction()!.hash);

      expect(receipt).to.not.be.null;

      const contractAddress = await contract.getAddress();
      const filteredLogs = receipt!.logs.filter(
        (log) => log.address === contractAddress && log.topics[0] === RateLimitInitializedEvent,
      );
      expect(filteredLogs.length).to.equal(1);
      const parsedLogs = contract.interface.parseLog(filteredLogs[0]);

      expect(parsedLogs!.args.periodInSeconds).to.equal(ONE_DAY_IN_SECONDS);
      expect(parsedLogs!.args.limitInWei).to.equal(limitInWei);
      expect(parsedLogs!.args.currentPeriodEnd).to.equal(currentPeriodEnd);
    });

    it("fails to initialize when the period is zero", async () => {
      const upgradeCall = deployUpgradableFromFactory("TestRateLimiter", [0, ONE_HUNDRED_ETH], {
        initializer: "initialize(uint256,uint256)",
      });

      await expectRevertWithCustomError(testRateLimiter, upgradeCall, "PeriodIsZero");
    });
  });

  describe("Rate limit values", () => {
    it("currentPeriodAmountInWei increases when amounts withdrawn", async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
    });

    it("currentPeriodAmountInWei increases to the limit", async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
      await withdrawAndExpectAmount(NINE_HUNDRED_ETH, TOTAL_ETH);
    });

    it("withdrawing beyond the limit fails", async () => {
      await withdrawAndExpectAmount(TOTAL_ETH, TOTAL_ETH);
      await expectRevertWithCustomError(testRateLimiter, testRateLimiter.withdrawSomeAmount(1), "RateLimitExceeded");
    });

    it("limit resets with time", async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
      await time.increase(ONE_DAY_IN_SECONDS);
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
    });
  });

  describe("Limit amount resetting", () => {
    beforeEach(async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
    });

    it("resetting currentPeriodAmountInWei fails if not admin", async () => {
      await expect(resetCurrentPeriodAmountByAccount(defaultAdmin)).to.be.revertedWith(
        buildAccessErrorMessage(defaultAdmin, USED_RATE_LIMIT_RESETTER_ROLE),
      );
    });

    it("resetting currentPeriodAmountInWei succeeds", async () => {
      await resetCurrentPeriodAmountByAccount(resetAdmin);
      await assertCurrentPeriodAmount(0n);
    });

    it("resetting limit amount fails if not admin", async () => {
      await expectRevertWithReason(
        testRateLimiter.resetRateLimitAmount(NINE_HUNDRED_ETH),
        buildAccessErrorMessage(defaultAdmin, RATE_LIMIT_SETTER_ROLE),
      );
      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("resetting limit amount succeeds resetting amount used", async () => {
      await withdrawAndExpectAmount(NINE_HUNDRED_ETH - ONE_HUNDRED_ETH, NINE_HUNDRED_ETH);

      const resetRateLimitCall = testRateLimiter.connect(resetAdmin).resetRateLimitAmount(ONE_HUNDRED_ETH);

      await expectEvent(testRateLimiter, resetRateLimitCall, "LimitAmountChanged", [
        resetAdmin.address,
        ONE_HUNDRED_ETH,
        true,
        false,
      ]);

      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("resetting limit amount succeeds without resetting amount used", async () => {
      await expect(testRateLimiter.connect(resetAdmin).resetRateLimitAmount(ONE_HUNDRED_ETH))
        .to.emit(testRateLimiter, "LimitAmountChanged")
        .withArgs(resetAdmin.address, ONE_HUNDRED_ETH, false, false);

      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("Resetting limits with period expired fires the correct event", async () => {
      await time.increase(ONE_DAY_IN_SECONDS);
      await expect(testRateLimiter.connect(resetAdmin).resetRateLimitAmount(ONE_HUNDRED_ETH))
        .to.emit(testRateLimiter, "LimitAmountChanged")
        .withArgs(resetAdmin.address, ONE_HUNDRED_ETH, false, true);
    });

    it("Resetting limits with period sets the used amount to zero", async () => {
      await time.increase(ONE_DAY_IN_SECONDS);
      await testRateLimiter.connect(resetAdmin).resetRateLimitAmount(ONE_HUNDRED_ETH);

      await assertCurrentPeriodAmount(0n);
    });
  });

  describe("When adding used amounts", () => {
    it("should not change currentPeriodAmountInWei when _usedAmount is 0", async () => {
      await testRateLimiter.withdrawSomeAmount(100n);

      const initialAmount = await testRateLimiter.currentPeriodAmountInWei();

      await testRateLimiter.withdrawSomeAmount(0);

      const finalAmount = await testRateLimiter.currentPeriodAmountInWei();

      expect(finalAmount).to.equal(initialAmount);
    });

    it("should not change currentPeriodAmountInWei after 24 hours passed when _usedAmount is 0", async () => {
      await testRateLimiter.withdrawSomeAmount(100n);

      const initialAmount = await testRateLimiter.currentPeriodAmountInWei();

      // 24 hours pass
      await time.increase(ONE_DAY_IN_SECONDS);

      await testRateLimiter.withdrawSomeAmount(0);

      const finalAmount = await testRateLimiter.currentPeriodAmountInWei();

      expect(finalAmount).to.equal(initialAmount);
    });
  });
});
