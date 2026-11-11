import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, decodeEventLog, getAddress, parseEther, parseUnits } from "viem";

import { deployWETH9Fixture } from "./helpers/deploy.js";
import { ADDRESS_ZERO, ONE_MINUTE_IN_SECONDS } from "../common/constants/index.js";
import { deployFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  generateRandomBytes,
  getTestAccounts,
  expectRevertWithCustomError,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("V3DexSwapWethDepositAdapter", () => {
  let dexSwap: ContractReturnType<"V3DexSwapWethDepositAdapter">;
  let rollupRevenueVault: TestAccount;
  let lineaToken: ContractReturnType<"TestERC20">;
  let testWETH9Address: Address;
  let router: ContractReturnType<"TestDexRouter">;

  async function deployDexSwapContractFixture() {
    const lineaToken = await deployFromFactory("TestERC20", ["TestERC20", "TEST", parseUnits("1000000000", 18)]);

    const testWETH9 = await deployWETH9Fixture();

    const router = await deployFromFactory("TestDexRouter");

    const dexSwap = await deployFromFactory("V3DexSwapWethDepositAdapter", [
      router.address,
      testWETH9,
      lineaToken.address,
      50,
    ]);

    return { dexSwap, lineaToken, testWETH9, router };
  }

  before(async () => {
    // Hardhat 2 also reset the chain here (`hardhat_reset`), which Hardhat 3's EDR does not support; the fixture
    // below does not depend on a fresh chain.
    [, rollupRevenueVault] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      dexSwap,
      lineaToken,
      testWETH9: testWETH9Address,
      router,
    } = await networkHelpers.loadFixture(deployDexSwapContractFixture));
  });

  describe("construtor", () => {
    it("Should revert when router address is zero", async () => {
      const randomnAddress = getAddress(generateRandomBytes(20));
      await expectRevertWithCustomError(
        dexSwap,
        deployFromFactory("V3DexSwapWethDepositAdapter", [ADDRESS_ZERO, randomnAddress, randomnAddress, 50]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert when WETH token address is zero", async () => {
      const randomnAddress = getAddress(generateRandomBytes(20));
      await expectRevertWithCustomError(
        dexSwap,
        deployFromFactory("V3DexSwapWethDepositAdapter", [randomnAddress, ADDRESS_ZERO, randomnAddress, 50]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert when LINEA token address is zero", async () => {
      const randomnAddress = getAddress(generateRandomBytes(20));
      await expectRevertWithCustomError(
        dexSwap,
        deployFromFactory("V3DexSwapWethDepositAdapter", [randomnAddress, randomnAddress, ADDRESS_ZERO, 50]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert when tick spacing is zero", async () => {
      const randomnAddress = getAddress(generateRandomBytes(20));
      await expectRevertWithCustomError(
        dexSwap,
        deployFromFactory("V3DexSwapWethDepositAdapter", [randomnAddress, randomnAddress, randomnAddress, 0]),
        "ZeroTickSpacingNotAllowed",
      );
    });

    it("Should emit an event when initialized", async () => {
      const { viem } = await network.getOrCreate();
      const randomnAddress = getAddress(generateRandomBytes(20));
      const publicClient = await viem.getPublicClient();
      const { contract, deploymentTransaction } = await viem.sendDeploymentTransaction("V3DexSwapWethDepositAdapter", [
        randomnAddress,
        randomnAddress,
        randomnAddress,
        50,
      ]);

      const receipt = await publicClient.waitForTransactionReceipt({ hash: deploymentTransaction.hash });
      const logs = receipt.logs;

      expect(logs).to.have.lengthOf(1);

      const event = decodeEventLog({ abi: contract.abi, data: logs[0].data, topics: logs[0].topics });
      expect(event.eventName).to.equal("V3DexSwapAdapterInitialized");
      expect(event.args.router).to.equal(randomnAddress);
      expect(event.args.wethToken).to.equal(randomnAddress);
      expect(event.args.lineaToken).to.equal(randomnAddress);
      expect(event.args.poolTickSpacing).to.equal(50);
    });

    it("Should set the correct addresses and values", async () => {
      const lineaTokenAddress = await dexSwap.read.LINEA_TOKEN();
      const wethTokenAddress = await dexSwap.read.WETH_TOKEN();
      const routerAddress = await dexSwap.read.ROUTER();
      const poolTickSpacing = await dexSwap.read.POOL_TICK_SPACING();

      expect(lineaTokenAddress).to.equal(getAddress(lineaToken.address));
      expect(wethTokenAddress).to.equal(testWETH9Address);
      expect(routerAddress).to.equal(getAddress(router.address));
      expect(poolTickSpacing).to.equal(50);
    });
  });

  describe("swap", () => {
    it("Should revert when msg.value == 0", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const minLineaOut = 200n;
      const deadline = (await networkHelpers.time.latest()) + ONE_MINUTE_IN_SECONDS;
      await expectRevertWithCustomError(
        dexSwap,
        dexSwap.write.swap([minLineaOut, BigInt(deadline)], { value: 0n }),
        "NoEthSent",
      );
    });

    it("Should revert when deadline is in the past", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const minLineaOut = 200n;
      const deadline = (await networkHelpers.time.latest()) - ONE_MINUTE_IN_SECONDS;
      const ethValueToSwap = parseEther("1");
      await expectRevertWithCustomError(
        dexSwap,
        dexSwap.write.swap([minLineaOut, BigInt(deadline)], { value: ethValueToSwap }),
        "DeadlineInThePast",
      );
    });

    it("Should revert when minLineaOut == 0", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const deadline = (await networkHelpers.time.latest()) + ONE_MINUTE_IN_SECONDS;
      const ethValueToSwap = parseEther("1");
      await expectRevertWithCustomError(
        dexSwap,
        dexSwap.write.swap([0n, BigInt(deadline)], { value: ethValueToSwap }),
        "ZeroMinLineaOutNotAllowed",
      );
    });

    it("Should swap ETH to LINEA tokens", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const minLineaOut = parseUnits("2", 18);
      const deadline = (await networkHelpers.time.latest()) + ONE_MINUTE_IN_SECONDS;

      const ethValueToSwap = parseEther("1");
      const rollupRevenueVaultLineaTokensBalanceBefore = await lineaToken.read.balanceOf([rollupRevenueVault.address]);
      await dexSwap.write.swap([minLineaOut, BigInt(deadline)], {
        account: rollupRevenueVault.account,
        value: ethValueToSwap,
      });

      const rollupRevenueVaultLineaTokensBalanceAfter = await lineaToken.read.balanceOf([rollupRevenueVault.address]);
      expect(rollupRevenueVaultLineaTokensBalanceAfter).to.equal(
        rollupRevenueVaultLineaTokensBalanceBefore + ethValueToSwap * 2n, // 1 ETH = 2 LINEA in the TestDexRouter
      );
    });

    it("Should swap ETH to LINEA tokens if deadline in same block", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const minLineaOut = parseUnits("2", 18);
      const deadline = (await networkHelpers.time.latest()) + ONE_MINUTE_IN_SECONDS;
      await networkHelpers.time.setNextBlockTimestamp(deadline);

      const ethValueToSwap = parseEther("1");
      const rollupRevenueVaultLineaTokensBalanceBefore = await lineaToken.read.balanceOf([rollupRevenueVault.address]);
      await dexSwap.write.swap([minLineaOut, BigInt(deadline)], {
        account: rollupRevenueVault.account,
        value: ethValueToSwap,
      });

      const rollupRevenueVaultLineaTokensBalanceAfter = await lineaToken.read.balanceOf([rollupRevenueVault.address]);
      expect(rollupRevenueVaultLineaTokensBalanceAfter).to.equal(
        rollupRevenueVaultLineaTokensBalanceBefore + ethValueToSwap * 2n, // 1 ETH = 2 LINEA in the TestDexRouter
      );
    });
  });
});
