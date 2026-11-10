import { pinnedTransparentUpgradeableProxyArtifact, pinnedUpgradeableBeaconArtifact } from "./pinned.js";
import { recordUpgradeTarget } from "../../../hardhat-plugins/deploy/targets.js";
import ProxyAdminModule from "../proxies/ProxyAdmin.js";

import type { UnsafeAllow } from "../../../hardhat-plugins/deploy/types.js";
import type { ArgumentType, CallableContractFuture, IgnitionModuleBuilder } from "@nomicfoundation/ignition-core";

type UpgradeSafety = {
  /** OpenZeppelin validation exceptions (the Hardhat 2 script's `unsafeAllow`). */
  unsafeAllow?: UnsafeAllow;
};

export type PinnedTransparentProxyOptions = UpgradeSafety & {
  /** Prefix of the future ids: `<name>Implementation` (caller), `<name>InitializeData`, `<name>Proxy`. */
  name: string;
  implementation: CallableContractFuture<string>;
  /** Bare initializer name (Ignition requires the bare name when the function is not overloaded). */
  initializer: string;
  initializerArgs: ArgumentType[];
};

/**
 * Deploys the pinned OpenZeppelin 4.x (v4.8.3) `TransparentUpgradeableProxy` in front of `implementation`, administered
 * by the shared pinned `ProxyAdmin`, initialized in the proxy constructor (the calldata the OZ plugin's `deployProxy`
 * passed). The deploy plugin validates the implementation with OpenZeppelin Upgrades before deploying and
 * `forceImport`s the proxy afterwards. Callers wrap the proxy with `m.contractAt("<Contract>", proxy, { id: "<name>" })`.
 */
export function deployPinnedTransparentProxy(m: IgnitionModuleBuilder, options: PinnedTransparentProxyOptions) {
  const { proxyAdmin } = m.useModule(ProxyAdminModule);
  const initializeData = m.encodeFunctionCall(options.implementation, options.initializer, options.initializerArgs, {
    id: `${options.name}InitializeData`,
  });
  const proxy = m.contract(
    "TransparentUpgradeableProxy",
    pinnedTransparentUpgradeableProxyArtifact,
    [options.implementation, proxyAdmin, initializeData],
    { id: `${options.name}Proxy` },
  );
  recordUpgradeTarget(proxy, {
    kind: "transparent",
    proxy,
    contractName: options.implementation.contractName,
    implementation: options.implementation,
    unsafeAllow: options.unsafeAllow ?? [],
  });
  return { proxy, proxyAdmin };
}

/** Deploys the pinned OpenZeppelin 4.x (v4.4.1) `UpgradeableBeacon` for `implementation`, owned by the deployer. */
export function deployPinnedBeacon(
  m: IgnitionModuleBuilder,
  options: UpgradeSafety & { name: string; implementation: CallableContractFuture<string> },
) {
  const beacon = m.contract("UpgradeableBeacon", pinnedUpgradeableBeaconArtifact, [options.implementation], {
    id: `${options.name}Beacon`,
  });
  recordUpgradeTarget(beacon, {
    kind: "beacon",
    beacon,
    contractName: options.implementation.contractName,
    implementation: options.implementation,
    unsafeAllow: options.unsafeAllow ?? [],
  });
  return beacon;
}

/**
 * Deploys a new implementation for the existing transparent proxy in the `proxy` parameter. The deploy plugin runs
 * OpenZeppelin `validateUpgrade` against that proxy first and prints the Security Council `ProxyAdmin.upgrade` (or
 * `upgradeAndCall`, with `reinitializer`) calldata afterwards.
 */
export function deployUpgradeImplementation<ContractNameT extends string>(
  m: IgnitionModuleBuilder,
  options: UpgradeSafety & {
    contractName: ContractNameT;
    constructorArgs?: ArgumentType[];
    reinitializer?: { functionName: string; args?: ArgumentType[] };
  },
) {
  const implementation = m.contract(options.contractName, options.constructorArgs ?? [], {
    id: `${options.contractName}Implementation`,
  });
  recordUpgradeTarget(implementation, {
    kind: "upgrade",
    proxy: m.getParameter<string>("proxy"),
    contractName: options.contractName,
    implementation,
    unsafeAllow: options.unsafeAllow ?? [],
    ...(options.reinitializer === undefined
      ? {}
      : { reinitializer: { functionName: options.reinitializer.functionName, args: options.reinitializer.args ?? [] } }),
  });
  return implementation;
}
