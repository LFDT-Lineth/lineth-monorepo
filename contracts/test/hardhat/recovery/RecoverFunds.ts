import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { RecoverFunds, TestExternalCalls } from "../../../typechain-types";
import {
  ADDRESS_ZERO,
  DEFAULT_ADMIN_ROLE,
  EMPTY_CALLDATA,
  FUNCTION_EXECUTOR_ROLE,
  INITIALIZED_ALREADY_MESSAGE,
  INITIAL_WITHDRAW_LIMIT,
} from "../common/constants";
import { deployUpgradableFromFactory } from "../common/deployment";
import { buildAccessErrorMessage, expectRevertWithCustomError, expectRevertWithReason } from "../common/helpers";

describe("RecoverFunds contract", () => {
  let recoverFunds: RecoverFunds;
  let recoverFundsAddress: string;
  let testContract: TestExternalCalls;
  let testContractAddress: string;

  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let executor: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  async function deployRecoverFundsFixture() {
    const recoverFunds = (await deployUpgradableFromFactory(
      "RecoverFunds",
      [securityCouncil.address, executor.address],
      {
        initializer: "initialize(address,address)",
        unsafeAllow: ["constructor"],
      },
    )) as unknown as RecoverFunds;

    recoverFundsAddress = await recoverFunds.getAddress();
    return recoverFunds;
  }

  async function deployTestContractFixture() {
    const testExternalCallsFactory = await ethers.getContractFactory("TestExternalCalls");
    const testExternalCalls = await testExternalCallsFactory.deploy();
    await testExternalCalls.waitForDeployment();
    testContractAddress = await testExternalCalls.getAddress();
    return testExternalCalls;
  }

  before(async () => {
    [admin, securityCouncil, nonAuthorizedAccount, executor] = await ethers.getSigners();
  });

  beforeEach(async () => {
    recoverFunds = await loadFixture(deployRecoverFundsFixture);
    testContract = await loadFixture(deployTestContractFixture);
  });

  describe("Fallback/Receive tests", () => {
    const sendEthToContract = async (data: string) => {
      return admin.sendTransaction({ to: await recoverFunds.getAddress(), value: INITIAL_WITHDRAW_LIMIT, data });
    };

    it("Should fail to send eth to the recoverFunds contract through the fallback", async () => {
      await expect(sendEthToContract(EMPTY_CALLDATA)).to.be.reverted;
    });

    it("Should fail to send eth to the recoverFunds contract through the receive function", async () => {
      await expect(sendEthToContract("0x1234")).to.be.reverted;
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
      expect(await recoverFunds.hasRole(DEFAULT_ADMIN_ROLE, securityCouncil.address)).true;
    });

    it("Should grant function executor admin role to executor address", async () => {
      expect(await recoverFunds.hasRole(FUNCTION_EXECUTOR_ROLE, executor.address)).true;
    });

    it("Should revert if the initialize function is called a second time", async () => {
      recoverFunds = await loadFixture(deployRecoverFundsFixture);
      const initializeCall = recoverFunds.initialize(securityCouncil.address, executor.address);

      await expectRevertWithReason(initializeCall, INITIALIZED_ALREADY_MESSAGE);
    });
  });

  describe("Executing functions", () => {
    it("Should revert if not security council", async () => {
      const functionCall = recoverFunds
        .connect(nonAuthorizedAccount)
        .executeExternalCall(nonAuthorizedAccount, EMPTY_CALLDATA, 1000n);
      await expectRevertWithReason(functionCall, buildAccessErrorMessage(nonAuthorizedAccount, FUNCTION_EXECUTOR_ROLE));
    });

    it("Should send half of the ETH sent", async () => {
      await recoverFunds
        .connect(executor)
        .executeExternalCall(testContractAddress, EMPTY_CALLDATA, 1000n, { value: 2000n });

      expect(await ethers.provider.getBalance(recoverFundsAddress)).equal(1000n);
      expect(await ethers.provider.getBalance(testContractAddress)).equal(1000n);
    });

    it("Should call and set the value on the test contract", async () => {
      const calldata = testContract.interface.encodeFunctionData("setValue", [1000n]);

      await recoverFunds.connect(executor).executeExternalCall(testContractAddress, calldata, 0n);

      expect(await testContract.testValue()).equal(1000n);
    });

    it("Should revert with the destinations custom error", async () => {
      const calldata = testContract.interface.encodeFunctionData("revertWithError");

      const functionCall = recoverFunds.connect(executor).executeExternalCall(testContractAddress, calldata, 0n);

      await expectRevertWithCustomError(testContract, functionCall, "TestError");
    });

    it("Should revert with the recovery contract custom error", async () => {
      const calldata = "0xdeadbeef";

      const functionCall = recoverFunds.connect(executor).executeExternalCall(testContractAddress, calldata, 0n);

      await expectRevertWithCustomError(recoverFunds, functionCall, "ExternalCallFailed", [testContractAddress]);
    });
  });
});
