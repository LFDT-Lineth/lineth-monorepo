import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { expect } from "chai";
import hre, { network } from "hardhat";
import { createHardhatRuntimeEnvironment } from "hardhat/hre";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { describe, it } from "node:test";
import {
  concat,
  encodeAbiParameters,
  encodeFunctionData,
  getAddress,
  parseAbiParameters,
  parseEther,
  zeroAddress,
  zeroHash,
} from "viem";

import { buildParametersFromEnv, deployedAddress, expectCode, withEnv } from "./helpers.js";
import {
  createParameterContext,
  findModuleFiles,
  loadCompanion,
  loadIgnitionModule,
  resolveModulePath,
  writeParametersFile,
} from "../../../hardhat-plugins/deploy/companion.js";
import {
  collectUpgradeTargets,
  encodeProxyAdminUpgrade,
  encodeProxyAdminUpgradeAndCall,
  registerAndReportUpgradeTargets,
  validateUpgradeTargets,
} from "../../../hardhat-plugins/deploy/upgrades.js";
import hardhatConfig from "../../../hardhat.config.js";
import { pinnedTransparentUpgradeableProxyArtifact } from "../../../ignition/modules/lib/pinned.js";
import { buildPlonkVerifierModule } from "../../../ignition/modules/lib/plonkVerifier.js";
import { deployPinnedTransparentProxy } from "../../../ignition/modules/lib/proxy.js";
import L2MessageServiceModule from "../../../ignition/modules/messaging/L2MessageService.js";
import { configureAddressFilter } from "../../../ignition/modules/rollup/AddressFilter.deploy.js";
import AddressFilterModule from "../../../ignition/modules/rollup/AddressFilter.js";
import { afterDeploy as reportSetVerifierAddress } from "../../../ignition/modules/verifiers/PlonkVerifier.deploy.js";
import {
  deployPresignedSystemContract,
  EIP2935_SYSTEM_CONTRACT,
  EIP4788_SYSTEM_CONTRACT,
} from "../../../scripts/hardhat/systemContractTasks.js";
import { getConnection, useFreshChain } from "../common/helpers/connection.js";

import type { DeploymentParameters, IgnitionModule } from "@nomicfoundation/ignition-core";
import type { NetworkConnection } from "hardhat/types/network";
import type { Address } from "viem";

const IGNITION_DIR = path.join(import.meta.dirname, "..", "..", "..", "ignition");

function role(index: number): Address {
  return getAddress(`0x${(0x2000 + index).toString(16).padStart(40, "0")}`);
}

/** Runs `fn` while capturing `console.log` / `console.warn` lines. */
async function captureLogs<T>(fn: () => Promise<T>): Promise<{ result: T; lines: string[] }> {
  const { log, warn } = console;
  const lines: string[] = [];
  const capture = (...args: unknown[]) => {
    lines.push(args.map((arg) => String(arg)).join(" "));
  };
  console.log = capture;
  console.warn = capture;
  try {
    const result = await fn();
    return { result, lines };
  } finally {
    console.log = log;
    console.warn = warn;
  }
}

async function captureError(fn: () => Promise<unknown>): Promise<string> {
  try {
    await fn();
  } catch (error: unknown) {
    return error instanceof Error ? error.message : String(error);
  }
  return "";
}

function temporaryFile(name: string, content: string): string {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "lineth-ignition-"));
  const filePath = path.join(directory, name);
  fs.writeFileSync(filePath, content);
  return filePath;
}

/** Deployed addresses by future id, from a module's results (what `ignition deploy` returns as `contracts`). */
function contractsByFutureId(ignitionModule: IgnitionModule, deployed: Record<string, { address: Address }>) {
  const contracts: Record<string, Address> = {};
  for (const [key, future] of Object.entries(ignitionModule.results)) {
    const contract = deployed[key];
    if (contract !== undefined) {
      contracts[future.id] = getAddress(contract.address);
    }
  }
  return contracts;
}

/**
 * What the `ignition deploy` override does around Ignition, on the test connection: companion parameters,
 * OpenZeppelin validation, deployment, `forceImport` / upgrade calldata. Returns the deployed addresses by future id.
 */
async function deployWithChecks(
  connection: NetworkConnection,
  moduleId: string,
  env: Record<string, string | undefined>,
): Promise<{ contracts: Record<string, Address>; parameters: DeploymentParameters }> {
  return withEnv(env, async () => {
    const modulePath = resolveModulePath(IGNITION_DIR, moduleId);
    const ignitionModule = await loadIgnitionModule(modulePath);
    const companion = await loadCompanion(modulePath);
    const parameters = companion.parameters ? await companion.parameters(await createParameterContext(connection)) : {};
    const targets = collectUpgradeTargets(ignitionModule);
    await validateUpgradeTargets(hre, connection, targets, parameters);
    const deployed = await connection.ignition.deploy(ignitionModule, { parameters });
    const contracts = contractsByFutureId(ignitionModule, deployed);
    await registerAndReportUpgradeTargets(hre, connection, targets, parameters, contracts);
    return { contracts, parameters };
  });
}

function requireContract(contracts: Record<string, Address>, futureId: string): Address {
  const address = contracts[futureId];
  if (address === undefined) {
    throw new Error(`${futureId} was not deployed`);
  }
  return address;
}

const l2MessageServiceEnv = {
  L2_SECURITY_COUNCIL: role(1),
  L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER: role(2),
  L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD: "86400",
  L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT: "1000000000000000000000",
};

describe("Ignition deploy plugin", () => {
  useFreshChain();

  describe("module parameters", () => {
    it("writes Ignition-readable parameters, replacing entries left by an earlier run", async () => {
      const connection = await getConnection();
      const output = temporaryFile("default.json", JSON.stringify({ Other: { stale: "yes" } }));

      writeParametersFile(output, await buildParametersFromEnv(connection, "L2MessageService", l2MessageServiceEnv));

      const written: unknown = JSON.parse(fs.readFileSync(output, "utf-8"));
      expect(written).to.not.have.property("Other");
      expect(written).to.have.nested.property("L2MessageService.rateLimitPeriod", "86400n");
      expect(written).to.have.nested.property("L2MessageService.securityCouncil", role(1));

      // Ignition reads the file (bigints revived) and deploys from it.
      const { l2MessageService } = await connection.ignition.deploy(L2MessageServiceModule, { parameters: output });
      expect(await l2MessageService.read.periodInSeconds()).to.equal(86400n);
    });

    it("fails like the Hardhat 2 script when a required env var is missing", async () => {
      const connection = await getConnection();
      const message = await captureError(() =>
        buildParametersFromEnv(connection, "L2MessageService", {
          ...l2MessageServiceEnv,
          L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD: undefined,
        }),
      );
      expect(message).to.equal(
        'Required environment variable "L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD" is missing or empty.',
      );
    });

    it("resolves a module by name and rejects unknown names", () => {
      expect(resolveModulePath(IGNITION_DIR, "LinethRollup")).to.equal(
        path.join(IGNITION_DIR, "modules", "rollup", "LinethRollup.ts"),
      );
      expect(() => resolveModulePath(IGNITION_DIR, "TimeLockUpgrade")).to.throw(
        'No Ignition module named "TimeLockUpgrade"',
      );
    });
  });

  describe("OpenZeppelin Upgrades checks", () => {
    it("validates the upgradeable implementations of every module with their unsafeAllow", async () => {
      const connection = await getConnection();
      // Upgrade modules point at a proxy with no code here: implementations are validated on their own.
      const parameters: DeploymentParameters = { $global: { proxy: role(99) } };
      await withEnv({ VERIFIER_CONTRACT_NAME: "PlonkVerifierForDataAggregation" }, async () => {
        for (const modulePath of findModuleFiles(path.join(IGNITION_DIR, "modules"))) {
          const targets = collectUpgradeTargets(await loadIgnitionModule(modulePath));
          await captureLogs(() => validateUpgradeTargets(hre, connection, targets, parameters));
        }
      });
    });

    it("rejects an implementation that is not upgrade-safe without its unsafeAllow", async () => {
      const connection = await getConnection();
      // YieldManager needs `state-variable-immutable` and `delegatecall` in its unsafeAllow (yield/YieldManager.ts).
      const unsafe = buildModule("UnsafeYieldManagerTest", (m) => {
        const implementation = m.contract("YieldManager", [zeroAddress], { id: "YieldManagerImplementation" });
        const { proxy } = deployPinnedTransparentProxy(m, {
          name: "YieldManager",
          implementation,
          initializer: "initialize",
          initializerArgs: [m.getParameter("yieldManagerInitializationData")],
          validationConstructorArgs: [zeroAddress],
        });
        return { proxy };
      });
      const message = await captureError(() =>
        validateUpgradeTargets(hre, connection, collectUpgradeTargets(unsafe), {}),
      );
      expect(message).to.contain("YieldManager");
    });

    it("refuses a pinned proxy deployed without the proxy helpers", () => {
      const unchecked = buildModule("UncheckedProxyTest", (m) => {
        const proxy = m.contract("TransparentUpgradeableProxy", pinnedTransparentUpgradeableProxyArtifact, [
          zeroAddress,
          zeroAddress,
          "0x",
        ]);
        return { proxy };
      });
      expect(() => collectUpgradeTargets(unchecked)).to.throw("without the proxy helpers");
    });

    it("forceImports the new proxy, then validates an upgrade against it and rejects an incompatible layout", async () => {
      const connection = await getConnection();
      const { result, lines } = await captureLogs(() =>
        deployWithChecks(connection, "L2MessageService", l2MessageServiceEnv),
      );
      const proxy = requireContract(result.contracts, "L2MessageService#L2MessageServiceProxy");
      expect(lines.some((line) => line.includes("OZ Upgrades manifest: imported L2MessageService"))).to.equal(true);

      const upgradeTargets = async (moduleId: string) =>
        collectUpgradeTargets(await loadIgnitionModule(resolveModulePath(IGNITION_DIR, moduleId)));
      const compatible = await captureLogs(async () =>
        validateUpgradeTargets(hre, connection, await upgradeTargets("L2MessageServiceUpgrade"), {
          L2MessageServiceUpgrade: { proxy },
        }),
      );
      expect(compatible.lines).to.include(`OZ Upgrades: L2MessageService is upgrade-safe for proxy ${proxy}.`);

      const message = await captureError(async () =>
        validateUpgradeTargets(hre, connection, await upgradeTargets("TokenBridgeUpgrade"), {
          TokenBridgeUpgrade: { proxy },
        }),
      );
      expect(message).to.not.equal("");
    });

    it("skips pinned-bytecode implementations", async () => {
      const connection = await getConnection();
      const ignitionModule = await loadIgnitionModule(resolveModulePath(IGNITION_DIR, "YieldManagerArtifacts"));
      const { lines } = await captureLogs(() =>
        validateUpgradeTargets(hre, connection, collectUpgradeTargets(ignitionModule), {}),
      );
      expect(lines.join("\n")).to.contain("OZ validation skipped");
    });
  });

  describe("upgrade modules print the Security Council calldata", () => {
    it("TokenBridgeUpgrade: ProxyAdmin.upgrade", async () => {
      const connection = await getConnection();
      const proxy = role(20);
      const { result, lines } = await captureLogs(() =>
        deployWithChecks(connection, "TokenBridgeUpgrade", { TOKEN_BRIDGE_ADDRESS: proxy }),
      );
      const implementation = requireContract(result.contracts, "TokenBridgeUpgrade#TokenBridgeImplementation");
      await expectCode(await connection.viem.getPublicClient(), implementation);
      expect(lines.join("\n")).to.contain(encodeProxyAdminUpgrade(proxy, implementation));
    });

    it("YieldManagerUpgrade: ProxyAdmin.upgrade", async () => {
      const connection = await getConnection();
      const { result, lines } = await captureLogs(() =>
        deployWithChecks(connection, "YieldManagerUpgrade", {
          LINETH_ROLLUP_ADDRESS: role(5),
          YIELD_MANAGER_ADDRESS: role(6),
        }),
      );
      const implementation = requireContract(result.contracts, "YieldManagerUpgrade#YieldManagerImplementation");
      expect(lines.join("\n")).to.contain(encodeProxyAdminUpgrade(role(6), implementation));
    });

    it("LinethRollupReinitializeV9: ProxyAdmin.upgradeAndCall(reinitializeLineaRollupV9)", async () => {
      const connection = await getConnection();
      const proxy = role(21);
      const addressFilter = role(22);
      const { result, lines } = await captureLogs(() =>
        deployWithChecks(connection, "LinethRollupReinitializeV9", {
          LINETH_ROLLUP_ADDRESS: proxy,
          LINETH_ROLLUP_FORCED_TRANSACTION_FEE_IN_WEI: "1000",
          LINETH_ROLLUP_ADDRESS_FILTER: addressFilter,
        }),
      );
      const implementation = requireContract(result.contracts, "LinethRollupReinitializeV9#LinethRollupImplementation");
      const { abi } = await hre.artifacts.readArtifact("LinethRollup");
      const data = encodeFunctionData({ abi, functionName: "reinitializeLineaRollupV9", args: [1000n, addressFilter] });
      expect(lines.join("\n")).to.contain(encodeProxyAdminUpgradeAndCall(proxy, implementation, data));
    });

    it("L2MessageServiceReinitializeV3 and TokenBridgeReinitializeV3: ProxyAdmin.upgradeAndCall(reinitializeV3)", async () => {
      const connection = await getConnection();
      const l2Proxy = role(23);
      const l2 = await captureLogs(() =>
        deployWithChecks(connection, "L2MessageServiceReinitializeV3", { L2_MESSAGE_SERVICE_ADDRESS: l2Proxy }),
      );
      const { abi: l2Abi } = await hre.artifacts.readArtifact("L2MessageService");
      expect(l2.lines.join("\n")).to.contain(
        encodeProxyAdminUpgradeAndCall(
          l2Proxy,
          requireContract(l2.result.contracts, "L2MessageServiceReinitializeV3#L2MessageServiceImplementation"),
          encodeFunctionData({ abi: l2Abi, functionName: "reinitializeV3" }),
        ),
      );

      const bridgeProxy = role(24);
      const bridge = await captureLogs(() =>
        deployWithChecks(connection, "TokenBridgeReinitializeV3", {
          DEPLOY_TOKEN_BRIDGE_ON_L1: "true",
          TOKEN_BRIDGE_ADDRESS: bridgeProxy,
        }),
      );
      const { abi: bridgeAbi } = await hre.artifacts.readArtifact("TokenBridge");
      expect(bridge.lines.join("\n")).to.contain(
        encodeProxyAdminUpgradeAndCall(
          bridgeProxy,
          requireContract(bridge.result.contracts, "TokenBridgeReinitializeV3#TokenBridgeImplementation"),
          encodeFunctionData({ abi: bridgeAbi, functionName: "reinitializeV3" }),
        ),
      );
    });

    it("RollupRevenueVaultReinitialize: ProxyAdmin.upgradeAndCall(initializeRolesAndStorageVariables)", async () => {
      const connection = await getConnection();
      const proxy = role(30);
      const { result, lines } = await captureLogs(() =>
        deployWithChecks(connection, "RollupRevenueVaultReinitialize", {
          ROLLUP_REVENUE_VAULT_ADDRESS: proxy,
          ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE: "1700000000",
          L2_SECURITY_COUNCIL: role(31),
          ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER: role(32),
          ROLLUP_REVENUE_VAULT_BURNER: role(33),
          ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER: role(34),
          TOKEN_BRIDGE_ADDRESS: role(35),
          L2_MESSAGE_SERVICE_ADDRESS: role(36),
          ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER: role(37),
          ROLLUP_REVENUE_VAULT_LINEA_TOKEN: role(38),
          ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER: role(39),
        }),
      );
      const implementation = requireContract(
        result.contracts,
        "RollupRevenueVaultReinitialize#RollupRevenueVaultImplementation",
      );
      const { abi } = await hre.artifacts.readArtifact("RollupRevenueVault");
      const data = encodeFunctionData({
        abi,
        functionName: "initializeRolesAndStorageVariables",
        args: [1700000000n, role(31), role(32), role(33), role(34), role(35), role(36), role(37), role(38), role(39)],
      });
      expect(lines.join("\n")).to.contain(encodeProxyAdminUpgradeAndCall(proxy, implementation, data));
    });
  });

  describe("afterDeploy", () => {
    it("PlonkVerifier prints the setVerifierAddress calldata", async () => {
      const connection = await getConnection();
      const env = {
        VERIFIER_CONTRACT_NAME: "PlonkVerifierForDataAggregation",
        VERIFIER_PROOF_TYPE: "3",
        VERIFIER_CHAIN_ID: "59144",
        VERIFIER_BASE_FEE: "7",
        VERIFIER_COINBASE: role(3),
        L2_MESSAGE_SERVICE_ADDRESS: role(4),
        VERIFIER_IS_ALLOWED_CIRCUIT_ID: "1",
      };
      const parameters = await buildParametersFromEnv(connection, "PlonkVerifier", env);
      const { verifier } = await connection.ignition.deploy(
        buildPlonkVerifierModule({ verifierContractName: "PlonkVerifierForDataAggregation", reuseMimc: false }),
        { parameters },
      );
      const verifierAddress = deployedAddress(verifier);

      const { lines } = await captureLogs(() =>
        withEnv(env, async () =>
          reportSetVerifierAddress({
            hre,
            connection,
            networkName: connection.networkName,
            chainId: 31337n,
            contracts: { "PlonkVerifier#PlonkVerifier": verifierAddress },
            parameters,
          }),
        ),
      );
      const expected = concat([
        "0xc2116974",
        encodeAbiParameters(parseAbiParameters("address, uint256"), [verifierAddress, 3n]),
      ]);
      expect(lines).to.include(`setVerifierAddress calldata: ${expected}`);
    });

    it("AddressFilter filters the file's addresses in batches and hands DEFAULT_ADMIN_ROLE to the Security Council, once", async () => {
      const connection = await getConnection();
      const [deployer] = await connection.viem.getWalletClients();
      const filterFile = temporaryFile(
        "addresses-filter.txt",
        [
          `- "${role(10)}"`,
          `- "${role(11)}"`,
          `- "${role(10)}"`,
          '- "0x0000000000000000000000000000000000000001"',
        ].join("\n"),
      );
      const council = role(12);
      const { addressFilter } = await connection.ignition.deploy(AddressFilterModule);

      await withEnv({ L1_SECURITY_COUNCIL: council, ADDRESS_FILTER_FILE_PATH: filterFile }, async () => {
        await configureAddressFilter(connection, addressFilter.address, deployer);
        // Re-running after the handover is a no-op instead of a revert.
        await configureAddressFilter(connection, addressFilter.address, deployer);
      });

      expect(await addressFilter.read.addressIsFiltered([role(10)])).to.equal(true);
      expect(await addressFilter.read.addressIsFiltered([role(11)])).to.equal(true);
      expect(await addressFilter.read.hasRole([zeroHash, council])).to.equal(true);
      expect(await addressFilter.read.hasRole([zeroHash, deployer.account.address])).to.equal(false);
    });

    it("AddressFilter keeps the deployer as admin when it is the Security Council", async () => {
      const connection = await getConnection();
      const [deployer] = await connection.viem.getWalletClients();
      const filterFile = temporaryFile("addresses-filter.txt", `- "${role(13)}"\n`);
      const { addressFilter } = await connection.ignition.deploy(AddressFilterModule);

      await withEnv(
        { L1_SECURITY_COUNCIL: getAddress(deployer.account.address), ADDRESS_FILTER_FILE_PATH: filterFile },
        () => configureAddressFilter(connection, addressFilter.address, deployer),
      );

      expect(await addressFilter.read.addressIsFiltered([role(13)])).to.equal(true);
      expect(await addressFilter.read.hasRole([zeroHash, deployer.account.address])).to.equal(true);
    });
  });

  describe("hardhat ignition deploy <Module>", () => {
    it("builds the parameters, validates, deploys and registers the proxy", async () => {
      // `--build-profile default`: Ignition's build reuses the test build. Its default `production` profile compiles
      // every file in isolation, which takes ~90 s and appends ~200 entries to OpenZeppelin's cache/validations.json.
      const defaultProfileHre = await createHardhatRuntimeEnvironment(hardhatConfig, { buildProfile: "default" });
      const parametersFile = path.join(IGNITION_DIR, "parameters", "default.json");
      const hadParametersFile = fs.existsSync(parametersFile);
      try {
        const { result, lines } = await captureLogs(() =>
          withEnv({ L1_SECURITY_COUNCIL: role(40), RECOVERFUNDS_EXECUTOR_ADDRESS: role(41) }, () =>
            defaultProfileHre.tasks.getTask(["ignition", "deploy"]).run({ modulePath: "RecoverFunds" }),
          ),
        );
        expect(result).to.have.property("type", "SUCCESSFUL_DEPLOYMENT");
        expect(lines.join("\n")).to.contain("OZ Upgrades: RecoverFunds is upgrade-safe (transparent).");
        expect(lines.join("\n")).to.contain("OZ Upgrades manifest: imported RecoverFunds (transparent)");
        expect(fs.readFileSync(parametersFile, "utf-8")).to.contain(role(41));
      } finally {
        if (!hadParametersFile) {
          fs.rmSync(path.dirname(parametersFile), { recursive: true, force: true });
        }
      }
    });
  });

  describe("presigned system contract tasks", () => {
    for (const systemContract of [EIP2935_SYSTEM_CONTRACT, EIP4788_SYSTEM_CONTRACT]) {
      it(`deploy:${systemContract.eipLabel.toLowerCase().replace("-", "")}-system-contract broadcasts the EIP's presigned deployment`, async () => {
        // Prague+ chains predeploy both system contracts (nonce 1), so use a Shanghai chain (PUSH0, no predeploys).
        const connection = await network.create({ override: { hardfork: "shanghai" } });
        try {
          const publicClient = await connection.viem.getPublicClient();
          const testClient = await connection.viem.getTestClient();
          await testClient.setBalance({ address: systemContract.predeterminedSenderAddress, value: parseEther("1") });

          const { lines } = await captureLogs(() => deployPresignedSystemContract(connection, systemContract));
          expect(lines.join("\n")).to.contain(
            `contract=${systemContract.contractName} deployed: address=${systemContract.expectedAddress}`,
          );
          await expectCode(publicClient, systemContract.expectedAddress);

          const rerun = await captureLogs(() => deployPresignedSystemContract(connection, systemContract));
          expect(rerun.lines).to.include(
            `${systemContract.eipLabel} system contract already deployed at ${systemContract.expectedAddress}`,
          );
        } finally {
          await connection.close();
        }
      });
    }
  });
});
