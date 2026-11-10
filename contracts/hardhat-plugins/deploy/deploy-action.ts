import { readDeploymentParameters } from "@nomicfoundation/hardhat-ignition/helpers";
import { DeploymentResultType } from "@nomicfoundation/ignition-core";
import path from "node:path";
import { getAddress, keccak256, toHex } from "viem";

import {
  createParameterContext,
  jsonBigint,
  loadCompanion,
  loadIgnitionModule,
  resolveModulePath,
  writeParametersFile,
} from "./companion.js";
import {
  collectUpgradeTargets,
  constructorArgsOf,
  registerAndReportUpgradeTargets,
  resolveArgument,
  validateUpgradeTargets,
} from "./upgrades.js";
import { withSignerUi } from "../signer-ui/session.js";

import type { DeploymentParameters } from "./types.js";
import type { SuccessfulDeploymentResult } from "@nomicfoundation/ignition-core";
import type { NetworkHooks } from "hardhat/types/hooks";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { ChainType, NetworkConnection } from "hardhat/types/network";
import type { TaskArguments } from "hardhat/types/tasks";
import type { Address } from "viem";

type IgnitionDeployArguments = {
  modulePath: string;
  parameters: string | undefined;
  deploymentId: string | undefined;
  verify: boolean;
};

function readArguments(args: TaskArguments): IgnitionDeployArguments {
  const { modulePath, parameters, deploymentId, verify } = args;
  if (typeof modulePath !== "string") {
    throw new Error("ignition deploy: missing module path or name.");
  }
  return {
    modulePath,
    parameters: typeof parameters === "string" ? parameters : undefined,
    deploymentId: typeof deploymentId === "string" ? deploymentId : undefined,
    verify: verify === true,
  };
}

/**
 * Ignition's deploy task creates its own connection (typed per chain type); OpenZeppelin's viem API takes the default
 * one and only needs the hardhat-viem extension.
 */
function isViemConnection(connection: NetworkConnection<ChainType | string>): connection is NetworkConnection {
  return "viem" in connection;
}

function isSuccessfulDeployment(value: unknown): value is SuccessfulDeploymentResult {
  return (
    typeof value === "object" &&
    value !== null &&
    "type" in value &&
    value.type === DeploymentResultType.SUCCESSFUL_DEPLOYMENT
  );
}

/**
 * `hardhat ignition deploy <module | ModuleName>`, wrapped with:
 * - parameters built by the module's `<Module>.deploy.ts` (unless `--parameters` is given), written to
 *   `ignition/parameters/<network>.json` for review;
 * - OpenZeppelin Upgrades validation of every upgradeable implementation, before anything is sent;
 * - browser signing (`--signer-ui`) for the whole run;
 * - afterwards, `forceImport` of new proxies / implementations, the Security Council calldata of upgrade modules, and
 *   the module's `afterDeploy`.
 */
export default async function deployWithUpgradeChecks(
  taskArguments: TaskArguments,
  hre: HardhatRuntimeEnvironment,
  runSuper: (taskArguments: TaskArguments) => Promise<unknown>,
): Promise<unknown> {
  const args = readArguments(taskArguments);
  const modulePath = resolveModulePath(hre.config.paths.ignition, args.modulePath);
  // Fresh artifacts and OpenZeppelin validations for the checks below. On live networks, build exactly as Ignition
  // does (production profile) so its own build is a cache hit. On the in-process network, a default-profile build
  // instead: it removes the isolated build infos of a production build, which the in-process network cannot load.
  const networkName = hre.globalOptions.network ?? "default"; // Hardhat's DEFAULT_NETWORK_NAME
  const isSimulated = hre.config.networks[networkName]?.type === "edr-simulated";
  await hre.tasks
    .getTask("build")
    .run(isSimulated ? { quiet: true } : { quiet: true, noTests: true, defaultBuildProfile: "production" });
  const ignitionModule = await loadIgnitionModule(modulePath);
  const companion = await loadCompanion(modulePath);

  const connection = await hre.network.getOrCreate();
  const context = await createParameterContext(connection);
  const { chainId } = context;

  let parametersPath = args.parameters;
  let parameters: DeploymentParameters = {};
  if (parametersPath === undefined && companion.parameters !== undefined) {
    parameters = await companion.parameters(context);
    parametersPath = path.join(hre.config.paths.ignition, "parameters", `${connection.networkName}.json`);
    writeParametersFile(parametersPath, parameters);
    console.log(`Ignition parameters for ${ignitionModule.id} written to ${parametersPath}`);
  } else if (parametersPath !== undefined) {
    // Ignition's own reader: JSON5, `"123n"` bigints, unsafe integers rejected.
    parameters = await readDeploymentParameters(parametersPath);
  }

  const targets = collectUpgradeTargets(ignitionModule);
  await validateUpgradeTargets(hre, connection, targets, parameters);

  // Upgrade modules deploy one implementation per bytecode and constructor arguments: identical ones resume the same
  // deployment (like OpenZeppelin's deployImplementation reuse), changed ones get a new deployment id instead of a
  // reconciliation error.
  let deploymentId = args.deploymentId;
  const upgradeTargets = targets.flatMap((target) => (target.kind === "upgrade" ? [target] : []));
  if (deploymentId === undefined && upgradeTargets.length > 0) {
    const parts = await Promise.all(
      upgradeTargets.map(async (target) => {
        const { bytecode } = await hre.artifacts.readArtifact(target.contractName);
        const constructorArgs = constructorArgsOf(target.implementation);
        if (constructorArgs.length === 0) {
          return bytecode.slice(2);
        }
        const resolved = constructorArgs.map((arg) => resolveArgument(arg, { parameters }));
        return `${bytecode.slice(2)}${toHex(JSON.stringify(resolved, jsonBigint)).slice(2)}`;
      }),
    );
    deploymentId = `${ignitionModule.id}-${chainId}-${keccak256(`0x${parts.join("")}`).slice(2, 10)}`;
    // Ignition does not print it; `ignition status | verify | wipe | track-tx` need it.
    console.log(`Deployment id: ${deploymentId}`);
  }

  const verify = !isSimulated && (args.verify || process.env.VERIFY_CONTRACT === "true");

  return withSignerUi(
    hre,
    `ignition deploy ${ignitionModule.id}`,
    async () => {
      // Ignition deploys on its own connection; capture it to reach the same chain afterwards (also in-process).
      // Only the first one: with --verify, Ignition's verify task opens another connection inside runSuper.
      let deployConnection: NetworkConnection | undefined;
      const captureConnection: Partial<NetworkHooks> = {
        async newConnection(hookContext, next) {
          const created = await next(hookContext);
          if (deployConnection === undefined && isViemConnection(created)) {
            deployConnection = created;
          }
          return created;
        },
      };
      hre.hooks.registerHandlers("network", captureConnection);
      let result: unknown;
      try {
        result = await runSuper({ ...taskArguments, modulePath, parameters: parametersPath, deploymentId, verify });
      } finally {
        hre.hooks.unregisterHandlers("network", captureConnection);
      }
      if (!isSuccessfulDeployment(result)) {
        return result;
      }

      deployConnection ??= connection;
      const contracts: Record<string, Address> = Object.fromEntries(
        Object.entries(result.contracts).map(([futureId, contract]) => [futureId, getAddress(contract.address)]),
      );
      await registerAndReportUpgradeTargets(hre, deployConnection, targets, parameters, contracts);
      await companion.afterDeploy?.({
        hre,
        connection: deployConnection,
        networkName: deployConnection.networkName,
        chainId,
        contracts,
        parameters,
      });
      return result;
    },
    { shutdownNextDev: process.env.HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY !== "true" },
  );
}
