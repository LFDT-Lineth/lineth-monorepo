import type { UnsafeAllow } from "./types.js";
import type { ArgumentType, Future } from "@nomicfoundation/ignition-core";

type ValidationOptions = {
  /** Contract name of the implementation (OpenZeppelin validation and `forceImport`). */
  contractName: string;
  /** Implementation future; an artifact-based (pinned bytecode) implementation is not validated. */
  implementation: Future;
  unsafeAllow: UnsafeAllow;
  /** Placeholder constructor arguments for validating an implementation with a constructor (count and types matter). */
  validationConstructorArgs?: readonly unknown[];
};

/** Upgradeable deployments an Ignition module declares; the deploy plugin validates and registers them. */
export type UpgradeTarget =
  | (ValidationOptions & { kind: "transparent"; proxy: Future })
  | (ValidationOptions & { kind: "beacon"; beacon: Future })
  | (ValidationOptions & {
      kind: "upgrade";
      /** Address of the existing transparent proxy (usually a module parameter). */
      proxy: ArgumentType;
      /** Constructor arguments of the implementation (part of the default deployment id). */
      constructorArgs: ArgumentType[];
      /** Reinitializer called through `ProxyAdmin.upgradeAndCall`; omitted → `ProxyAdmin.upgrade`. */
      reinitializer?: { functionName: string; args: ArgumentType[] };
    });

const targets = new WeakMap<Future, UpgradeTarget>();

/** Called by the module helpers that deploy proxies, beacons or upgrade implementations. */
export function recordUpgradeTarget(future: Future, target: UpgradeTarget): void {
  targets.set(future, target);
}

export function getUpgradeTarget(future: Future): UpgradeTarget | undefined {
  return targets.get(future);
}
