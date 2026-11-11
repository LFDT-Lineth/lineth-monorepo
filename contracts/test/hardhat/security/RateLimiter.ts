import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { decodeEventLog, parseEther } from "viem";

import {
  DEFAULT_ADMIN_ROLE,
  INITIALIZED_ERROR_MESSAGE,
  ONE_DAY_IN_SECONDS,
  RATE_LIMIT_SETTER_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
} from "../common/constants/index.js";
import { deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  expectAccessControlRevert,
  expectHasRole,
  getTestAccounts,
  expectRevertWithReason,
  expectRevertWithCustomError,
  expectEvent,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Rate limiter", () => {
  let testRateLimiter: ContractReturnType<"TestRateLimiter">;
  let defaultAdmin: TestAccount;
  let resetAdmin: TestAccount;

  const ONE_HUNDRED_ETH = parseEther("100");
  const NINE_HUNDRED_ETH = parseEther("900");
  const TOTAL_ETH = ONE_HUNDRED_ETH + NINE_HUNDRED_ETH;

  async function deployTestRateLimiterFixture() {
    return deployUpgradableFromFactory("TestRateLimiter", [ONE_DAY_IN_SECONDS, TOTAL_ETH], {
      initializer: "initialize(uint256,uint256)",
    });
  }

  async function increaseTime(seconds: number) {
    const { networkHelpers } = await network.getOrCreate();
    await networkHelpers.time.increase(seconds);
  }

  before(async () => {
    [defaultAdmin, resetAdmin] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    testRateLimiter = await networkHelpers.loadFixture(deployTestRateLimiterFixture);
    await testRateLimiter.write.grantRole([RATE_LIMIT_SETTER_ROLE, resetAdmin.address]);
    await testRateLimiter.write.grantRole([USED_RATE_LIMIT_RESETTER_ROLE, resetAdmin.address]);
  });

  async function assertCurrentPeriodAmount(expectedAmount: bigint) {
    expect(await testRateLimiter.read.currentPeriodAmountInWei()).to.equal(expectedAmount);
  }

  async function resetCurrentPeriodAmountByAccount(admin: TestAccount) {
    await testRateLimiter.write.resetAmountUsedInPeriod({ account: admin.account });
  }

  async function withdrawAndExpectAmount(amount: bigint, expectedAmount: bigint) {
    await testRateLimiter.write.withdrawSomeAmount([amount]);
    expect(await testRateLimiter.read.currentPeriodAmountInWei()).to.equal(expectedAmount);
  }

  describe("Initialization checks", () => {
    it("Deployer has default admin role", async () => {
      await expectHasRole(testRateLimiter, DEFAULT_ADMIN_ROLE, defaultAdmin);
    });

    it("fails to initialize when not initialising", async () => {
      await expectRevertWithReason(
        testRateLimiter.write.tryInitialize([BigInt(ONE_DAY_IN_SECONDS), ONE_HUNDRED_ETH]),
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
      const { viem } = await network.getOrCreate();
      const RateLimitInitializedEvent = "0x8f805c372b66240792580418b7328c0c554ae235f0932475c51b026887fe26a9";

      const limitInWei = ONE_HUNDRED_ETH;

      const publicClient = await viem.getPublicClient();
      const blockBefore = await publicClient.getBlock();

      const currentTimestamp = blockBefore.timestamp + 1n;
      const currentPeriodEnd = currentTimestamp + BigInt(ONE_DAY_IN_SECONDS);

      const contract = await deployUpgradableFromFactory("TestRateLimiter", [ONE_DAY_IN_SECONDS, ONE_HUNDRED_ETH]);

      // The OZ viem API does not expose the deployment transaction: read the logs emitted since the block before.
      const logs = await publicClient.getLogs({ address: contract.address, fromBlock: blockBefore.number + 1n });

      const filteredLogs = logs.filter(
        (log) =>
          log.address.toLowerCase() === contract.address.toLowerCase() && log.topics[0] === RateLimitInitializedEvent,
      );
      expect(filteredLogs.length).to.equal(1);
      const parsedLogs = decodeEventLog({
        abi: contract.abi,
        eventName: "RateLimitInitialized",
        data: filteredLogs[0].data,
        topics: filteredLogs[0].topics,
      });

      expect(parsedLogs.args.periodInSeconds).to.equal(BigInt(ONE_DAY_IN_SECONDS));
      expect(parsedLogs.args.limitInWei).to.equal(limitInWei);
      expect(parsedLogs.args.currentPeriodEnd).to.equal(currentPeriodEnd);
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
      await expectRevertWithCustomError(
        testRateLimiter,
        testRateLimiter.write.withdrawSomeAmount([1n]),
        "RateLimitExceeded",
      );
    });

    it("limit resets with time", async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
      await increaseTime(ONE_DAY_IN_SECONDS);
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
    });
  });

  describe("Limit amount resetting", () => {
    beforeEach(async () => {
      await withdrawAndExpectAmount(ONE_HUNDRED_ETH, ONE_HUNDRED_ETH);
    });

    it("resetting currentPeriodAmountInWei fails if not admin", async () => {
      await expectAccessControlRevert(
        resetCurrentPeriodAmountByAccount(defaultAdmin),
        defaultAdmin,
        USED_RATE_LIMIT_RESETTER_ROLE,
      );
    });

    it("resetting currentPeriodAmountInWei succeeds", async () => {
      await resetCurrentPeriodAmountByAccount(resetAdmin);
      await assertCurrentPeriodAmount(0n);
    });

    it("resetting limit amount fails if not admin", async () => {
      await expectAccessControlRevert(
        testRateLimiter.write.resetRateLimitAmount([NINE_HUNDRED_ETH]),
        defaultAdmin,
        RATE_LIMIT_SETTER_ROLE,
      );
      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("resetting limit amount succeeds resetting amount used", async () => {
      await withdrawAndExpectAmount(NINE_HUNDRED_ETH - ONE_HUNDRED_ETH, NINE_HUNDRED_ETH);

      const resetRateLimitCall = testRateLimiter.write.resetRateLimitAmount([ONE_HUNDRED_ETH], {
        account: resetAdmin.account,
      });

      await expectEvent(testRateLimiter, resetRateLimitCall, "LimitAmountChanged", [
        resetAdmin.address,
        ONE_HUNDRED_ETH,
        true,
        false,
      ]);

      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("resetting limit amount succeeds without resetting amount used", async () => {
      await expectEvent(
        testRateLimiter,
        testRateLimiter.write.resetRateLimitAmount([ONE_HUNDRED_ETH], { account: resetAdmin.account }),
        "LimitAmountChanged",
        [resetAdmin.address, ONE_HUNDRED_ETH, false, false],
      );

      await assertCurrentPeriodAmount(ONE_HUNDRED_ETH);
    });

    it("Resetting limits with period expired fires the correct event", async () => {
      await increaseTime(ONE_DAY_IN_SECONDS);
      await expectEvent(
        testRateLimiter,
        testRateLimiter.write.resetRateLimitAmount([ONE_HUNDRED_ETH], { account: resetAdmin.account }),
        "LimitAmountChanged",
        [resetAdmin.address, ONE_HUNDRED_ETH, false, true],
      );
    });

    it("Resetting limits with period sets the used amount to zero", async () => {
      await increaseTime(ONE_DAY_IN_SECONDS);
      await testRateLimiter.write.resetRateLimitAmount([ONE_HUNDRED_ETH], { account: resetAdmin.account });

      await assertCurrentPeriodAmount(0n);
    });
  });

  describe("When adding used amounts", () => {
    it("should not change currentPeriodAmountInWei when _usedAmount is 0", async () => {
      await testRateLimiter.write.withdrawSomeAmount([100n]);

      const initialAmount = await testRateLimiter.read.currentPeriodAmountInWei();

      await testRateLimiter.write.withdrawSomeAmount([0n]);

      const finalAmount = await testRateLimiter.read.currentPeriodAmountInWei();

      expect(finalAmount).to.equal(initialAmount);
    });

    it("should not change currentPeriodAmountInWei after 24 hours passed when _usedAmount is 0", async () => {
      await testRateLimiter.write.withdrawSomeAmount([100n]);

      const initialAmount = await testRateLimiter.read.currentPeriodAmountInWei();

      // 24 hours pass
      await increaseTime(ONE_DAY_IN_SECONDS);

      await testRateLimiter.write.withdrawSomeAmount([0n]);

      const finalAmount = await testRateLimiter.read.currentPeriodAmountInWei();

      expect(finalAmount).to.equal(initialAmount);
    });
  });
});
