import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, type Hex, encodeFunctionData } from "viem";

import {
  ADDRESS_ZERO,
  DEFAULT_ADMIN_ROLE,
  EMPTY_CALLDATA,
  FUNCTION_EXECUTOR_ROLE,
  INITIALIZED_ALREADY_MESSAGE,
  INITIAL_WITHDRAW_LIMIT,
} from "../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  getTestAccounts,
  expectRevertWithCustomError,
  expectRevertWithReason,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("RecoverFunds contract", () => {
  let recoverFunds: ContractReturnType<"RecoverFunds">;
  let recoverFundsAddress: Address;
  let testContract: ContractReturnType<"TestExternalCalls">;
  let testContractAddress: Address;

  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let executor: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  async function deployRecoverFundsFixture() {
    const recoverFunds = await deployUpgradableFromFactory(
      "RecoverFunds",
      [securityCouncil.address, executor.address],
      {
        initializer: "initialize(address,address)",
        unsafeAllow: ["constructor"],
      },
    );

    recoverFundsAddress = recoverFunds.address;
    return recoverFunds;
  }

  async function deployTestContractFixture() {
    const testExternalCalls = await deployFromFactory("TestExternalCalls");
    testContractAddress = testExternalCalls.address;
    return testExternalCalls;
  }

  before(async () => {
    [admin, securityCouncil, nonAuthorizedAccount, executor] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    recoverFunds = await networkHelpers.loadFixture(deployRecoverFundsFixture);
    testContract = await networkHelpers.loadFixture(deployTestContractFixture);
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: Hex) => {
      return admin.sendTransaction({ to: recoverFunds.address, value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the recoverFunds contract through the fallback", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract(EMPTY_CALLDATA));
    });

    it("Should fail to send eth to the recoverFunds contract through the receive function", async () => {
      const { viem } = await network.getOrCreate();
      await viem.assertions.revert(sendEthToContract("0x1234"));
    });
  });

  describe("Initialisation", () => {
    it("Should revert if security council address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory("RecoverFunds", [ADDRESS_ZERO, executor.address], {
        initializer: "initialize(address,address)",
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(recoverFunds, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should revert if security council address is zero address", async () => {
      const deployCall = deployUpgradableFromFactory("RecoverFunds", [securityCouncil.address, ADDRESS_ZERO], {
        initializer: "initialize(address,address)",
        unsafeAllow: ["constructor"],
      });

      await expectRevertWithCustomError(recoverFunds, deployCall, "ZeroAddressNotAllowed");
    });

    it("Should grant default admin role to security council", async () => {
      expect(await recoverFunds.read.hasRole([DEFAULT_ADMIN_ROLE, securityCouncil.address])).true;
    });

    it("Should grant function executor admin role to executor address", async () => {
      expect(await recoverFunds.read.hasRole([FUNCTION_EXECUTOR_ROLE, executor.address])).true;
    });

    it("Should revert if the initialize function is called a second time", async () => {
      const { networkHelpers } = await network.getOrCreate();
      recoverFunds = await networkHelpers.loadFixture(deployRecoverFundsFixture);
      const initializeCall = recoverFunds.write.initialize([securityCouncil.address, executor.address]);

      await expectRevertWithReason(initializeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Executing functions", () => {
    it("Should revert if not security council", async () => {
      const functionCall = recoverFunds.write.executeExternalCall(
        [nonAuthorizedAccount.address, EMPTY_CALLDATA, 1000n],
        { account: nonAuthorizedAccount.account },
      );
      await expectRevertWithReason(functionCall, buildAccessErrorMessage(nonAuthorizedAccount, FUNCTION_EXECUTOR_ROLE));
    });

    it("Should send half of the ETH sent", async () => {
      const { viem } = await network.getOrCreate();
      await recoverFunds.write.executeExternalCall([testContractAddress, EMPTY_CALLDATA, 1000n], {
        account: executor.account,
        value: 2000n,
      });

      const publicClient = await viem.getPublicClient();
      expect(await publicClient.getBalance({ address: recoverFundsAddress })).equal(1000n);
      expect(await publicClient.getBalance({ address: testContractAddress })).equal(1000n);
    });

    it("Should call and set the value on the test contract", async () => {
      const calldata = encodeFunctionData({ abi: testContract.abi, functionName: "setValue", args: [1000n] });

      await recoverFunds.write.executeExternalCall([testContractAddress, calldata, 0n], { account: executor.account });

      expect(await testContract.read.testValue()).equal(1000n);
    });

    it("Should revert with the destinations custom error", async () => {
      const calldata = encodeFunctionData({ abi: testContract.abi, functionName: "revertWithError" });

      const functionCall = recoverFunds.write.executeExternalCall([testContractAddress, calldata, 0n], {
        account: executor.account,
      });

      await expectRevertWithCustomError(testContract, functionCall, "TestError");
    });

    it("Should revert with the recovery contract custom error", async () => {
      const calldata = "0xdeadbeef";

      const functionCall = recoverFunds.write.executeExternalCall([testContractAddress, calldata, 0n], {
        account: executor.account,
      });

      await expectRevertWithCustomError(recoverFunds, functionCall, "ExternalCallFailed", [testContractAddress]);
    });
  });
});
