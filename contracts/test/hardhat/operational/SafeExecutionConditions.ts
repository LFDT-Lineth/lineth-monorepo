import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers, network } from "hardhat";

import { SafeExecutionConditions, TestSafe } from "../../../typechain-types";
import { deployFromFactory } from "../common/deployment";
import { expectRevertWithCustomError } from "../common/helpers";

describe("SafeExecutionConditions", () => {
  let conditions: SafeExecutionConditions;
  let safe: TestSafe;
  let executor: SignerWithAddress;
  let owner: SignerWithAddress;
  let stranger: SignerWithAddress;

  async function deploySafeExecutionConditionsFixture() {
    const [executor, owner, stranger] = await ethers.getSigners();

    const conditions = (await deployFromFactory("SafeExecutionConditions")) as unknown as SafeExecutionConditions;
    const safe = (await deployFromFactory("TestSafe", [owner.address])) as unknown as TestSafe;

    return { conditions, safe, executor, owner, stranger };
  }

  before(async () => {
    await network.provider.send("hardhat_reset");
  });

  beforeEach(async () => {
    ({ conditions, safe, executor, owner, stranger } = await loadFixture(deploySafeExecutionConditionsFixture));
  });

  describe("onlyAfterTimestamp", () => {
    it("Should not revert when the current timestamp equals the threshold", async () => {
      const currentTimestamp = await time.latest();
      await expect(conditions.onlyAfterTimestamp(currentTimestamp)).to.not.be.reverted;
    });

    it("Should not revert when the current timestamp is after the threshold", async () => {
      const pastTimestamp = (await time.latest()) - 1;
      await expect(conditions.onlyAfterTimestamp(pastTimestamp)).to.not.be.reverted;
    });

    it("Should revert with OnlyOnOrAfter when the current timestamp is before the threshold", async () => {
      const futureTimestamp = (await time.latest()) + 3600;
      await expectRevertWithCustomError(conditions, conditions.onlyAfterTimestamp(futureTimestamp), "OnlyOnOrAfter", [
        futureTimestamp,
      ]);
    });
  });

  describe("onlyExecutedBy", () => {
    it("Should not revert when the transaction origin is in the executors list", async () => {
      await expect(conditions.connect(executor).onlyExecutedBy([stranger.address, executor.address])).to.not.be
        .reverted;
    });

    it("Should revert with OnlyAllowedExecutor when the transaction origin is not in the executors list", async () => {
      await expectRevertWithCustomError(
        conditions,
        conditions.connect(stranger).onlyExecutedBy([executor.address, owner.address]),
        "OnlyAllowedExecutor",
      );
    });

    it("Should revert with OnlyAllowedExecutor when the executors list is empty", async () => {
      await expectRevertWithCustomError(
        conditions,
        conditions.connect(executor).onlyExecutedBy([]),
        "OnlyAllowedExecutor",
      );
    });

    it("Should authorise the EOA via tx.origin even when called through a contract", async () => {
      const calldata = conditions.interface.encodeFunctionData("onlyExecutedBy", [[owner.address]]);
      await expect(safe.connect(owner).execute(await conditions.getAddress(), calldata)).to.not.be.reverted;
    });
  });

  describe("onlyExecutedBySafeOwner", () => {
    it("Should not revert when executed through the Safe by one of its owners", async () => {
      const calldata = conditions.interface.encodeFunctionData("onlyExecutedBySafeOwner", [await safe.getAddress()]);
      await expect(safe.connect(owner).execute(await conditions.getAddress(), calldata)).to.not.be.reverted;
    });

    it("Should revert with OnlySafeOwner when the transaction origin is not a Safe owner", async () => {
      const calldata = conditions.interface.encodeFunctionData("onlyExecutedBySafeOwner", [await safe.getAddress()]);
      await expectRevertWithCustomError(
        conditions,
        safe.connect(stranger).execute(await conditions.getAddress(), calldata),
        "OnlySafeOwner",
      );
    });

    it("Should revert with OnlyExecutingSafe when called directly rather than through the Safe", async () => {
      // Direct EOA call: msg.sender is the owner EOA, not the Safe referenced by _safe.
      await expectRevertWithCustomError(
        conditions,
        conditions.connect(owner).onlyExecutedBySafeOwner(await safe.getAddress()),
        "OnlyExecutingSafe",
      );
    });

    it("Should revert with OnlyExecutingSafe when the executing Safe references a different Safe", async () => {
      const otherSafe = (await deployFromFactory("TestSafe", [owner.address])) as unknown as TestSafe;
      const calldata = conditions.interface.encodeFunctionData("onlyExecutedBySafeOwner", [
        await otherSafe.getAddress(),
      ]);
      await expectRevertWithCustomError(
        conditions,
        safe.connect(owner).execute(await conditions.getAddress(), calldata),
        "OnlyExecutingSafe",
      );
    });
  });
});
