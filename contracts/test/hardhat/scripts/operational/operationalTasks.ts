import { expect } from "chai";
import hre, { network } from "hardhat";
import { before, describe, it } from "node:test";
import { encodeFunctionData, parseAbi, parseEther, zeroHash } from "viem";

import { deployTokenBridgeWithMockMessaging } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";
import {
  DEFAULT_ADMIN_ROLE,
  ONE_DAY_IN_SECONDS,
  RATE_LIMIT_SETTER_ROLE,
  USED_RATE_LIMIT_RESETTER_ROLE,
  VERIFIER_SETTER_ROLE,
} from "../../common/constants/index.js";
import { deployUpgradableFromFactory } from "../../common/deployment.js";
import { type TestAccount, getAccountsFixture } from "../../common/helpers/index.js";
import { deployLinethRollupFixture } from "../../rollup/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

/**
 * Runs the operational task actions (scripts/operational/**) against the in-process network, through the same
 * `hre.tasks` entry point as `pnpm exec hardhat <task>`. The signer is the first configured account (key mode).
 */
async function runTask(name: string, args: Record<string, string>): Promise<string[]> {
  const logged: string[] = [];
  const originalLog = console.log;
  console.log = (...values: unknown[]) => {
    logged.push(values.map((value) => String(value)).join(" "));
    originalLog(...values);
  };
  try {
    await hre.tasks.getTask(name).run(args);
  } finally {
    console.log = originalLog;
  }
  return logged;
}

async function deployRateLimiterFixture() {
  return deployUpgradableFromFactory("TestRateLimiter", [ONE_DAY_IN_SECONDS, parseEther("1000")], {
    initializer: "initialize(uint256,uint256)",
  });
}

describe("Operational tasks", () => {
  let admin: TestAccount;
  let securityCouncil: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin, securityCouncil, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  describe("grantContractRoles / setRateLimit / renounceContractRoles", () => {
    let rateLimiter: ContractReturnType<"TestRateLimiter">;

    before(async () => {
      const { networkHelpers } = await network.getOrCreate();
      rateLimiter = await networkHelpers.loadFixture(deployRateLimiterFixture);
    });

    it("grants the rate limit setter role, sets the limit, then hands the roles over", async () => {
      await runTask("grantContractRoles", {
        adminAddress: admin.address,
        proxyAddress: rateLimiter.address,
        contractType: "TestRateLimiter",
        contractRoles: RATE_LIMIT_SETTER_ROLE,
      });
      expect(await rateLimiter.read.hasRole([RATE_LIMIT_SETTER_ROLE, admin.address])).to.equal(true);

      const logged = await runTask("setRateLimit", {
        messageServiceAddress: rateLimiter.address,
        messageServiceType: "TestRateLimiter",
        withdrawLimit: "123456789",
      });
      expect(await rateLimiter.read.limitInWei()).to.equal(123456789n);
      expect(logged).to.include(`Validated rate limit in wei of 123456789 at ${rateLimiter.address}`);

      await runTask("grantContractRoles", {
        adminAddress: nonAuthorizedAccount.address,
        proxyAddress: rateLimiter.address,
        contractType: "TestRateLimiter",
        contractRoles: `${RATE_LIMIT_SETTER_ROLE},${DEFAULT_ADMIN_ROLE}`,
      });

      const renounceLogs = await runTask("renounceContractRoles", {
        oldAdminAddress: admin.address,
        newAdminAddress: nonAuthorizedAccount.address,
        proxyAddress: rateLimiter.address,
        contractType: "TestRateLimiter",
        contractRoles: `${RATE_LIMIT_SETTER_ROLE},${USED_RATE_LIMIT_RESETTER_ROLE},${DEFAULT_ADMIN_ROLE}`,
      });
      expect(renounceLogs).to.include(`New admin does not have ${USED_RATE_LIMIT_RESETTER_ROLE}, skipping`);

      expect(await rateLimiter.read.hasRole([RATE_LIMIT_SETTER_ROLE, admin.address])).to.equal(false);
      expect(await rateLimiter.read.hasRole([DEFAULT_ADMIN_ROLE, admin.address])).to.equal(false);
      expect(await rateLimiter.read.hasRole([RATE_LIMIT_SETTER_ROLE, nonAuthorizedAccount.address])).to.equal(true);
      expect(await rateLimiter.read.hasRole([DEFAULT_ADMIN_ROLE, nonAuthorizedAccount.address])).to.equal(true);
    });

    it("rejects a malformed role list", async () => {
      let error: unknown;
      try {
        await runTask("grantContractRoles", {
          adminAddress: admin.address,
          proxyAddress: rateLimiter.address,
          contractType: "TestRateLimiter",
          contractRoles: "0x1234",
        });
      } catch (caught) {
        error = caught;
      }
      expect(String(error)).to.contain('Invalid role "0x1234"');
    });
  });

  describe("LinethRollup tasks", () => {
    let linethRollup: ContractReturnType<"TestLinethRollup">;

    before(async () => {
      const { networkHelpers } = await network.getOrCreate();
      ({ linethRollup } = await networkHelpers.loadFixture(deployLinethRollupFixture));
      await linethRollup.write.grantRole([VERIFIER_SETTER_ROLE, admin.address], { account: securityCouncil.account });
      await linethRollup.write.grantRole([DEFAULT_ADMIN_ROLE, admin.address], { account: securityCouncil.account });
    });

    it("getCurrentFinalizedBlockNumber reads currentL2BlockNumber", async () => {
      const logged = await runTask("getCurrentFinalizedBlockNumber", {
        contractType: "TestLinethRollup",
        proxyAddress: linethRollup.address,
      });
      expect(logged).to.include(`Current finalized L2 block number ${await linethRollup.read.currentL2BlockNumber()}`);
    });

    it("setVerifierAddress sets the verifier for a proof type", async () => {
      await runTask("setVerifierAddress", {
        verifierProofType: "7",
        proxyAddress: linethRollup.address,
        verifierAddress: nonAuthorizedAccount.address,
      });
      expect(await linethRollup.read.verifiers([7n])).to.equal(nonAuthorizedAccount.address);
    });

    it("addAndClaimMessage adds a merkle root and claims the message", async () => {
      const { networkHelpers } = await network.getOrCreate();
      const publicClient = await (await network.getOrCreate()).viem.getPublicClient();
      await networkHelpers.setBalance(linethRollup.address, parseEther("10"));
      const recipient = securityCouncil.address;
      const balanceBefore = await publicClient.getBalance({ address: recipient });

      await runTask("addAndClaimMessage", {
        linethRollupAddress: linethRollup.address,
        to: recipient,
        value: "1000",
        data: "0x",
      });

      expect(await publicClient.getBalance({ address: recipient })).to.equal(balanceBefore + 1000n);
    });

    it("addAndClaimMessageForLST fails fast without a yield provider", async () => {
      let error: unknown;
      try {
        await runTask("addAndClaimMessageForLST", {
          linethRollupAddress: linethRollup.address,
          to: securityCouncil.address,
          value: "1000",
        });
      } catch (caught) {
        error = caught;
      }
      expect(String(error)).to.contain("yieldProvider / YIELD_PROVIDER_ADDRESS");
    });
  });

  it("setMessageServiceOnTokenBridge sets the message service", async () => {
    const { l1TokenBridge } = await deployTokenBridgeWithMockMessaging();

    await runTask("setMessageServiceOnTokenBridge", {
      messageServiceAddress: nonAuthorizedAccount.address,
      tokenBridgeAddress: l1TokenBridge.address,
    });

    expect(await l1TokenBridge.read.messageService()).to.equal(nonAuthorizedAccount.address);
  });

  it("prepareInitiateOssification logs the Safe calldata", async () => {
    const yieldManager = admin.address;
    const yieldProvider = securityCouncil.address;
    const dashboard = nonAuthorizedAccount.address;

    const logged = await runTask("prepareInitiateOssification", { yieldManager, yieldProvider, dashboard });

    const pauseStakingCalldata = encodeFunctionData({
      abi: parseAbi(["function pauseStaking(address _yieldProvider)"]),
      functionName: "pauseStaking",
      args: [yieldProvider],
    });
    const grantRoleCalldata = encodeFunctionData({
      abi: parseAbi(["function grantRole(bytes32 role, address account)"]),
      functionName: "grantRole",
      args: [zeroHash, yieldManager],
    });
    expect(logged).to.include(`   Calldata: ${pauseStakingCalldata}`);
    expect(logged).to.include(`   Calldata: ${grantRoleCalldata}`);
  });

  it("addLidoStVaultYieldProvider logs the initialization data", async () => {
    const logged = await runTask("addLidoStVaultYieldProvider", {
      yieldManager: admin.address,
      yieldProvider: securityCouncil.address,
      nodeOperator: nonAuthorizedAccount.address,
      securityCouncil: securityCouncil.address,
      nodeOperatorFee: "100",
      confirmExpiry: "86400",
    });
    expect(logged.some((line) => line.startsWith("  _initializationData: 0x"))).to.equal(true);
  });

  it("unstakePermissionless fails fast on missing params", async () => {
    let error: unknown;
    try {
      await runTask("unstakePermissionless", { yieldManager: admin.address });
    } catch (caught) {
      error = caught;
    }
    expect(String(error)).to.contain("Missing required params/envs");
  });
});
