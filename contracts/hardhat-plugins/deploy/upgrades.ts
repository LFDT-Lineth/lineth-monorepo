import {
  FutureType,
  isAccountRuntimeValue,
  isContractFuture,
  isFuture,
  isModuleParameterRuntimeValue,
} from "@nomicfoundation/ignition-core";
import { upgrades } from "@openzeppelin/hardhat-upgrades/viem";
import { concat, encodeAbiParameters, encodeFunctionData, getAddress, isAddress, parseAbiParameters } from "viem";

import { getUpgradeTarget } from "./targets.js";

import type { UpgradeTarget } from "./targets.js";
import type { ArgumentType, DeploymentParameters, Future, IgnitionModule } from "@nomicfoundation/ignition-core";
import type { ValidateImplementationOptions } from "@openzeppelin/hardhat-upgrades/viem";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { NetworkConnection } from "hardhat/types/network";
import type { Address, Hex } from "viem";

/** Contract names of the pinned OpenZeppelin 4.x proxy contracts; deploying one without a helper skips the checks. */
const PROXY_CONTRACT_NAMES = new Set(["TransparentUpgradeableProxy", "UpgradeableBeacon"]);

/** Every upgradeable deployment declared by `ignitionModule` and its submodules (recorded by `ignition/modules/lib/proxy.ts`). */
export function collectUpgradeTargets(ignitionModule: IgnitionModule): UpgradeTarget[] {
  const visited = new Set<IgnitionModule>();
  const targets: UpgradeTarget[] = [];
  const visit = (current: IgnitionModule) => {
    if (visited.has(current)) {
      return;
    }
    visited.add(current);
    for (const future of current.futures) {
      const target = getUpgradeTarget(future);
      if (target !== undefined) {
        targets.push(target);
      } else if (
        // Both the artifact form and `m.contract("TransparentUpgradeableProxy", …)`: src/_testing compiles one too.
        (future.type === FutureType.CONTRACT_DEPLOYMENT ||
          future.type === FutureType.NAMED_ARTIFACT_CONTRACT_DEPLOYMENT) &&
        PROXY_CONTRACT_NAMES.has(future.contractName)
      ) {
        throw new Error(
          `${future.id} deploys a ${future.contractName} without the proxy helpers of ignition/modules/lib/proxy.ts, ` +
            "so it would skip the OpenZeppelin Upgrades checks.",
        );
      }
    }
    current.submodules.forEach(visit);
  };
  visit(ignitionModule);
  return targets;
}

/** Pinned (artifact-based) implementations are not built from `src/`: OpenZeppelin validation does not apply. */
function isPinnedImplementation(target: UpgradeTarget): boolean {
  return target.implementation.type === FutureType.CONTRACT_DEPLOYMENT;
}

/** Resolves an Ignition argument (module parameter, account, deployed future, nested values) to a plain value. */
export function resolveArgument(
  argument: ArgumentType,
  context: { parameters: DeploymentParameters; contracts?: Record<string, Address>; accounts?: readonly Address[] },
): unknown {
  if (isModuleParameterRuntimeValue(argument)) {
    const value =
      context.parameters[argument.moduleId]?.[argument.name] ??
      context.parameters.$global?.[argument.name] ??
      argument.defaultValue;
    if (value === undefined) {
      throw new Error(`Parameter "${argument.moduleId}.${argument.name}" is not set.`);
    }
    return value;
  }
  if (isAccountRuntimeValue(argument)) {
    return context.accounts?.[argument.accountIndex];
  }
  if (isFuture(argument)) {
    const address = isContractFuture(argument) ? context.contracts?.[argument.id] : undefined;
    if (address === undefined) {
      throw new Error(`Cannot resolve ${argument.id} to an address.`);
    }
    return address;
  }
  if (Array.isArray(argument)) {
    return argument.map((item) => resolveArgument(item, context));
  }
  if (typeof argument === "object" && argument !== null) {
    return Object.fromEntries(Object.entries(argument).map(([key, value]) => [key, resolveArgument(value, context)]));
  }
  return argument;
}

function resolveAddress(argument: ArgumentType, parameters: DeploymentParameters): Address {
  const value = resolveArgument(argument, { parameters });
  if (typeof value !== "string" || !isAddress(value, { strict: false })) {
    throw new Error(`Expected an address, got ${String(value)}`);
  }
  return getAddress(value);
}

function validationOptions(target: UpgradeTarget) {
  const options: ValidateImplementationOptions & { constructorArgs?: readonly unknown[] } = {
    kind: target.kind === "beacon" ? "beacon" : "transparent",
    unsafeAllow: target.unsafeAllow,
  };
  if (target.validationConstructorArgs !== undefined) {
    options.constructorArgs = target.validationConstructorArgs;
  }
  return options;
}

/**
 * Validates every upgradeable implementation with OpenZeppelin Upgrades before anything is deployed: new proxies and
 * beacons with `validateImplementation`, upgrades with `validateUpgrade` against the existing proxy (falling back to
 * `validateImplementation` when the proxy has no code on this network or its current implementation is not in
 * `.openzeppelin/`).
 */
export async function validateUpgradeTargets(
  hre: HardhatRuntimeEnvironment,
  connection: NetworkConnection,
  targets: readonly UpgradeTarget[],
  parameters: DeploymentParameters,
): Promise<void> {
  if (targets.length === 0) {
    return;
  }
  const oz = await upgrades(hre, connection);
  const publicClient = await connection.viem.getPublicClient();
  for (const target of targets) {
    if (isPinnedImplementation(target)) {
      console.log(`${target.contractName}: pinned audited bytecode (not built from src/); OZ validation skipped.`);
      continue;
    }
    const options = validationOptions(target);
    const proxy = target.kind === "upgrade" ? resolveAddress(target.proxy, parameters) : undefined;
    const proxyCode = proxy === undefined ? undefined : await publicClient.getCode({ address: proxy });
    if (proxy !== undefined && (proxyCode === undefined || proxyCode === "0x")) {
      console.warn(`OZ Upgrades: no contract at proxy ${proxy} on this network; storage layout not compared.`);
    } else if (proxy !== undefined) {
      try {
        await oz.validateUpgrade(proxy, target.contractName, options);
        console.log(`OZ Upgrades: ${target.contractName} is upgrade-safe for proxy ${proxy}.`);
        continue;
      } catch (error: unknown) {
        if (!(error instanceof Error) || !error.message.includes("is not registered")) {
          throw error;
        }
        console.warn(
          `OZ Upgrades: the current implementation of ${proxy} is not in .openzeppelin/; storage layout not compared. ` +
            "forceImport it with the previous implementation to enable the comparison.",
        );
      }
    }
    await oz.validateImplementation(target.contractName, options);
    console.log(`OZ Upgrades: ${target.contractName} is upgrade-safe (${options.kind}).`);
  }
}

/** `ProxyAdmin.upgrade(proxy, implementation)` (selector 0x99a88ec4). */
export function encodeProxyAdminUpgrade(proxy: Address, implementation: Address): Hex {
  return concat(["0x99a88ec4", encodeAbiParameters(parseAbiParameters("address, address"), [proxy, implementation])]);
}

/** `ProxyAdmin.upgradeAndCall(proxy, implementation, data)` (selector 0x9623609d). */
export function encodeProxyAdminUpgradeAndCall(proxy: Address, implementation: Address, data: Hex): Hex {
  return concat([
    "0x9623609d",
    encodeAbiParameters(parseAbiParameters("address, address, bytes"), [proxy, implementation, data]),
  ]);
}

function deployedAddress(contracts: Record<string, Address>, future: Future): Address | undefined {
  return contracts[future.id];
}

/**
 * After a successful deployment: registers new proxies, beacons and upgrade implementations in `.openzeppelin/`
 * (`forceImport`, best effort) and prints the Security Council calldata of upgrade modules.
 */
export async function registerAndReportUpgradeTargets(
  hre: HardhatRuntimeEnvironment,
  connection: NetworkConnection,
  targets: readonly UpgradeTarget[],
  parameters: DeploymentParameters,
  contracts: Record<string, Address>,
): Promise<void> {
  if (targets.length === 0) {
    return;
  }
  const oz = await upgrades(hre, connection);
  for (const target of targets) {
    const implementation = deployedAddress(contracts, target.implementation);
    if (target.kind === "upgrade" && implementation !== undefined) {
      const proxy = resolveAddress(target.proxy, parameters);
      if (target.reinitializer === undefined) {
        console.log("Encoded Tx Upgrade from Security Council:", "\n", encodeProxyAdminUpgrade(proxy, implementation));
      } else {
        const { abi } = await hre.artifacts.readArtifact(target.contractName);
        const data = encodeFunctionData({
          abi,
          functionName: target.reinitializer.functionName,
          args: target.reinitializer.args.map((arg) => resolveArgument(arg, { parameters, contracts })),
        });
        console.log(
          `Encoded Tx Upgrade with Reinitialization (${target.reinitializer.functionName}) from Security Council:`,
          "\n",
          encodeProxyAdminUpgradeAndCall(proxy, implementation, data),
        );
      }
      console.log("\n");
    }

    const imported =
      target.kind === "transparent"
        ? deployedAddress(contracts, target.proxy)
        : target.kind === "beacon"
          ? deployedAddress(contracts, target.beacon)
          : implementation;
    if (imported === undefined || isPinnedImplementation(target)) {
      continue;
    }
    const kind = target.kind === "beacon" ? "beacon" : "transparent";
    try {
      await oz.forceImport(imported, target.contractName, { kind });
      console.log(`OZ Upgrades manifest: imported ${target.contractName} (${target.kind}) at ${imported}`);
    } catch (error: unknown) {
      console.warn(
        `OZ Upgrades manifest: forceImport(${imported}, ${target.contractName}) skipped: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }
}
