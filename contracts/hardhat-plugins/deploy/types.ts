import type { DeploymentParameters, ModuleParameters } from "@nomicfoundation/ignition-core";
import type { ValidateImplementationOptions } from "@openzeppelin/hardhat-upgrades/viem";
import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { NetworkConnection } from "hardhat/types/network";
import type { Address, Hex } from "viem";

export type UnsafeAllow = NonNullable<ValidateImplementationOptions["unsafeAllow"]>;

/** What a parameter builder may read: the target network (registry file selection) and its code. */
export type ParameterContext = {
  networkName: string;
  chainId: bigint;
  /** `eth_getCode`; `undefined` when the account has no code. */
  getCode(address: Address): Promise<Hex | undefined>;
};

export type AfterDeployContext = {
  hre: HardhatRuntimeEnvironment;
  /** The connection Ignition deployed with. */
  connection: NetworkConnection;
  networkName: string;
  chainId: bigint;
  /** Deployed addresses by Ignition future id (`<ModuleId>#<FutureId>`). */
  contracts: Record<string, Address>;
  parameters: DeploymentParameters;
};

/**
 * Optional `<Module>.deploy.ts` next to an Ignition module. `parameters` builds the module's Ignition parameters from env
 * vars and the address registry; `afterDeploy` runs after a successful deployment (it must be idempotent: re-running
 * `ignition deploy` on a finished deployment runs it again).
 */
export type DeployCompanion = {
  parameters?: (context: ParameterContext) => Promise<DeploymentParameters>;
  afterDeploy?: (context: AfterDeployContext) => Promise<void> | void;
};

export type { DeploymentParameters, ModuleParameters };
