import { expect } from "chai";
import path from "node:path";
import { getAddress, isAddress, pad, slice } from "viem";

import { createParameterContext, loadCompanion, resolveModulePath } from "../../../hardhat-plugins/deploy/companion.js";
import {
  pinnedProxyAdminArtifact,
  pinnedTransparentUpgradeableProxyArtifact,
} from "../../../ignition/modules/lib/pinned.js";

import type { DeploymentParameters } from "@nomicfoundation/ignition-core";
import type { NetworkConnection } from "hardhat/types/network";
import type { Address, PublicClient } from "viem";

/** ERC-1967 slots. */
const IMPLEMENTATION_SLOT = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";
const ADMIN_SLOT = "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103";

/**
 * Optional env vars read by the parameter builders and deploy tasks. Unset in every `withEnv` call so a developer's
 * `contracts/.env` cannot change what the specs deploy.
 */
const OPTIONAL_DEPLOY_ENV_VARS = [
  "VERIFIER_MIMC_ADDRESS",
  "MIN_DELAY",
  "LINETH_ROLLUP_PAUSE_TYPES_ROLES",
  "LINETH_ROLLUP_UNPAUSE_TYPES_ROLES",
  "LINETH_ROLLUP_ROLE_ADDRESSES",
  "VALIDIUM_PAUSE_TYPES_ROLES",
  "VALIDIUM_UNPAUSE_TYPES_ROLES",
  "VALIDIUM_ROLE_ADDRESSES",
  "L2_MESSAGE_SERVICE_PAUSE_TYPES_ROLES",
  "L2_MESSAGE_SERVICE_UNPAUSE_TYPES_ROLES",
  "L2_MESSAGE_SERVICE_ROLE_ADDRESSES",
  "DEPLOY_TOKEN_BRIDGE_ON_L1",
  "TOKEN_BRIDGE_PAUSE_TYPES_ROLES",
  "TOKEN_BRIDGE_UNPAUSE_TYPES_ROLES",
  "TOKEN_BRIDGE_ROLE_ADDRESSES",
  "L1_RESERVED_TOKEN_ADDRESSES",
  "L2_RESERVED_TOKEN_ADDRESSES",
  "YIELD_MANAGER_PAUSE_TYPES_ROLES",
  "YIELD_MANAGER_UNPAUSE_TYPES_ROLES",
  "GI_FIRST_VALIDATOR",
  "GI_PENDING_PARTIAL_WITHDRAWALS_ROOT",
  "VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN",
  "ADDRESS_FILTER_FILE_PATH",
  "VERIFY_CONTRACT",
  "HARDHAT_SIGNER_UI",
  "EXPECTED_SIGNER_ADDRESS",
];

/** Runs `fn` with exactly these deploy env vars (others from `OPTIONAL_DEPLOY_ENV_VARS` unset), then restores `process.env`. */
export async function withEnv<T>(vars: Record<string, string | undefined>, fn: () => Promise<T>): Promise<T> {
  const saved = new Map<string, string | undefined>();
  const names = new Set([...OPTIONAL_DEPLOY_ENV_VARS, ...Object.keys(vars)]);
  for (const name of names) {
    saved.set(name, process.env[name]);
    const value = vars[name];
    if (value === undefined) {
      delete process.env[name];
    } else {
      process.env[name] = value;
    }
  }
  try {
    return await fn();
  } finally {
    for (const [name, value] of saved) {
      if (value === undefined) {
        delete process.env[name];
      } else {
        process.env[name] = value;
      }
    }
  }
}

const IGNITION_DIR = path.join(import.meta.dirname, "..", "..", "..", "ignition");

/** Parameters for `moduleId` built by its `<Module>.deploy.ts` from `env`. */
export async function buildParametersFromEnv(
  connection: NetworkConnection,
  moduleId: string,
  env: Record<string, string | undefined>,
): Promise<DeploymentParameters> {
  const companion = await loadCompanion(resolveModulePath(IGNITION_DIR, moduleId));
  if (companion.parameters === undefined) {
    return {};
  }
  const buildParameters = companion.parameters;
  return withEnv(env, async () => buildParameters(await createParameterContext(connection)));
}

/**
 * Address of an Ignition result contract whose name is only known at module-load time (e.g. the
 * `VERIFIER_CONTRACT_NAME` verifier), which hardhat-viem cannot type.
 */
export function deployedAddress(contract: unknown): Address {
  if (
    contract === null ||
    typeof contract !== "object" ||
    !("address" in contract) ||
    typeof contract.address !== "string" ||
    !isAddress(contract.address)
  ) {
    throw new Error("Expected a deployed contract instance with an address.");
  }
  return getAddress(contract.address);
}

async function readAddressSlot(publicClient: PublicClient, address: Address, slot: `0x${string}`): Promise<Address> {
  const value = await publicClient.getStorageAt({ address, slot });
  return getAddress(slice(pad(value ?? "0x", { size: 32 }), 12));
}

export async function expectCode(publicClient: PublicClient, address: Address): Promise<void> {
  const code = await publicClient.getCode({ address });
  expect(code, `no code at ${address}`).to.not.equal(undefined);
  expect(code).to.not.equal("0x");
}

/**
 * The proxy and its admin run the pinned OpenZeppelin 4.x mainnet bytecode byte for byte, and the ERC-1967 slots point
 * at `proxyAdmin` and `implementation`.
 */
export async function expectPinnedTransparentProxy(
  publicClient: PublicClient,
  proxy: Address,
  proxyAdmin: Address,
  implementation: Address,
): Promise<void> {
  expect((await publicClient.getCode({ address: proxy }))?.toLowerCase()).to.equal(
    pinnedTransparentUpgradeableProxyArtifact.deployedBytecode.toLowerCase(),
  );
  expect((await publicClient.getCode({ address: proxyAdmin }))?.toLowerCase()).to.equal(
    pinnedProxyAdminArtifact.deployedBytecode.toLowerCase(),
  );
  expect(await readAddressSlot(publicClient, proxy, ADMIN_SLOT)).to.equal(getAddress(proxyAdmin));
  expect(await readAddressSlot(publicClient, proxy, IMPLEMENTATION_SLOT)).to.equal(getAddress(implementation));
}
