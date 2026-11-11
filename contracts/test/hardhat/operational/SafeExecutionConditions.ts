import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { type Hash, encodeFunctionData } from "viem";

import { deployFromFactory } from "../common/deployment.js";
import { type TestAccount, getTestAccounts, expectRevertWithCustomError } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("SafeExecutionConditions", () => {
  let conditions: ContractReturnType<"SafeExecutionConditions">;
  let safe: ContractReturnType<"TestSafe">;
  let executor: TestAccount;
  let owner: TestAccount;
  let stranger: TestAccount;

  async function deploySafeExecutionConditionsFixture() {
    const [executor, owner, stranger] = await getTestAccounts();

    const conditions = await deployFromFactory("SafeExecutionConditions");
    const safe = await deployFromFactory("TestSafe", [[owner.address]]);

    return { conditions, safe, executor, owner, stranger };
  }

  // Equivalent of chai-matchers' `.to.not.be.reverted` for a sent transaction.
  async function expectNotReverted(call: Promise<Hash>) {
    const { viem } = await network.getOrCreate();
    const publicClient = await viem.getPublicClient();
    const receipt = await publicClient.waitForTransactionReceipt({ hash: await call });
    expect(receipt.status).to.equal("success");
  }

  // Hardhat 2 reset the chain in a `before` hook (`hardhat_reset`), which Hardhat 3's EDR does not support; the
  // fixture below does not depend on a fresh chain.
  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ conditions, safe, executor, owner, stranger } = await networkHelpers.loadFixture(
      deploySafeExecutionConditionsFixture,
    ));
  });

  describe("onlyOnOrAfterTimestamp", () => {
    it("Should not revert when the current timestamp equals the threshold", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const currentTimestamp = await networkHelpers.time.latest();
      await expectNotReverted(conditions.write.onlyOnOrAfterTimestamp([BigInt(currentTimestamp)]));
    });

    it("Should not revert when the current timestamp is after the threshold", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const pastTimestamp = (await networkHelpers.time.latest()) - 1;
      await expectNotReverted(conditions.write.onlyOnOrAfterTimestamp([BigInt(pastTimestamp)]));
    });

    it("Should revert with OnlyOnOrAfter when the current timestamp is before the threshold", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const futureTimestamp = (await networkHelpers.time.latest()) + 3600;
      await expectRevertWithCustomError(
        conditions,
        conditions.write.onlyOnOrAfterTimestamp([BigInt(futureTimestamp)]),
        "OnlyOnOrAfter",
        [BigInt(futureTimestamp)],
      );
    });
  });

  describe("onlyExecutedBy", () => {
    it("Should not revert when the transaction origin is in the executors list", async () => {
      await expectNotReverted(
        conditions.write.onlyExecutedBy([[stranger.address, executor.address]], { account: executor.account }),
      );
    });

    it("Should revert with OnlyAllowedExecutor when the transaction origin is not in the executors list", async () => {
      await expectRevertWithCustomError(
        conditions,
        conditions.write.onlyExecutedBy([[executor.address, owner.address]], { account: stranger.account }),
        "OnlyAllowedExecutor",
      );
    });

    it("Should revert with OnlyAllowedExecutor when the executors list is empty", async () => {
      await expectRevertWithCustomError(
        conditions,
        conditions.write.onlyExecutedBy([[]], { account: executor.account }),
        "OnlyAllowedExecutor",
      );
    });

    it("Should authorise the EOA via tx.origin even when called through a contract", async () => {
      const calldata = encodeFunctionData({
        abi: conditions.abi,
        functionName: "onlyExecutedBy",
        args: [[owner.address]],
      });
      await expectNotReverted(safe.write.execute([conditions.address, calldata], { account: owner.account }));
    });
  });

  describe("onlyExecutedBySafeOwner", () => {
    it("Should not revert when executed through the Safe by one of its owners", async () => {
      const calldata = encodeFunctionData({
        abi: conditions.abi,
        functionName: "onlyExecutedBySafeOwner",
        args: [safe.address],
      });
      await expectNotReverted(safe.write.execute([conditions.address, calldata], { account: owner.account }));
    });

    it("Should revert with OnlySafeOwner when the transaction origin is not a Safe owner", async () => {
      const calldata = encodeFunctionData({
        abi: conditions.abi,
        functionName: "onlyExecutedBySafeOwner",
        args: [safe.address],
      });
      await expectRevertWithCustomError(
        conditions,
        safe.write.execute([conditions.address, calldata], { account: stranger.account }),
        "OnlySafeOwner",
      );
    });

    it("Should revert with OnlyExecutingSafe when called directly rather than through the Safe", async () => {
      // Direct EOA call: msg.sender is the owner EOA, not the Safe referenced by _safe.
      await expectRevertWithCustomError(
        conditions,
        conditions.write.onlyExecutedBySafeOwner([safe.address], { account: owner.account }),
        "OnlyExecutingSafe",
      );
    });

    it("Should revert with OnlyExecutingSafe when the executing Safe references a different Safe", async () => {
      const otherSafe = await deployFromFactory("TestSafe", [[owner.address]]);
      const calldata = encodeFunctionData({
        abi: conditions.abi,
        functionName: "onlyExecutedBySafeOwner",
        args: [otherSafe.address],
      });
      await expectRevertWithCustomError(
        conditions,
        safe.write.execute([conditions.address, calldata], { account: owner.account }),
        "OnlyExecutingSafe",
      );
    });
  });
});
