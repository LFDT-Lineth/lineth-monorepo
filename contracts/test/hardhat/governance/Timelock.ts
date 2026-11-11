import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { zeroAddress } from "viem";

import { CANCELLER_ROLE, EXECUTOR_ROLE, PROPOSER_ROLE, TIMELOCK_ADMIN_ROLE } from "../common/constants/index.js";
import { deployFromFactory } from "../common/deployment.js";
import { type TestAccount, getTestAccounts } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("Timelock", () => {
  let contract: ContractReturnType<"TimeLock">;
  let proposer: TestAccount;
  let executor: TestAccount;

  async function deployTimeLockFixture() {
    return deployFromFactory("TimeLock", [10n, [proposer.address], [executor.address], zeroAddress]);
  }

  before(async () => {
    [, proposer, executor] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    contract = await networkHelpers.loadFixture(deployTimeLockFixture);
  });

  describe("Initialization", () => {
    it("Timelock contract should have the 'TIMELOCK_ADMIN_ROLE' role", async () => {
      expect(await contract.read.hasRole([TIMELOCK_ADMIN_ROLE, contract.address])).to.be.true;
    });

    it("Proposer address should have the 'PROPOSER_ROLE' role", async () => {
      expect(await contract.read.hasRole([PROPOSER_ROLE, proposer.address])).to.be.true;
    });

    it("Proposer address should have the 'CANCELLER_ROLE' role", async () => {
      expect(await contract.read.hasRole([CANCELLER_ROLE, proposer.address])).to.be.true;
    });

    it("Executor address should have the 'EXECUTOR_ROLE' role", async () => {
      expect(await contract.read.hasRole([EXECUTOR_ROLE, executor.address])).to.be.true;
    });

    it("Should set the minDelay state variable with the value passed in the contructor params", async () => {
      expect(await contract.read.getMinDelay()).to.equal(10n);
    });
  });
});
