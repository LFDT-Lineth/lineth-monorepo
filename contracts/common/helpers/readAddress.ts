import { ethers } from "ethers";
import fs from "fs";
import path from "path";

import {
  getPopulatedAddresses,
  isMultiRegistryEntry,
  loadRegistry,
  lookupRegistryEntry,
  parseEnvAddress,
  parseEnvAddressList,
  REGISTRY_NETWORKS,
  sameAddressSet,
} from "./addressRegistry";
import { getHandoffAddress } from "./deploymentHandoff";
import { formatEnvVarForLog, formatEnvVarValueForMessage } from "./envVarLogging";

/**
 * Resolves the deployment network for standalone scripts without Hardhat runtime.
 * Names outside {@link REGISTRY_NETWORKS} (e.g. `custom`) skip registry lookup and require env vars.
 */
export function getDeploymentNetworkName(): string {
  return process.env.HARDHAT_NETWORK ?? process.env.NETWORK ?? "custom";
}

/**
 * Reads a single contract address from the per-network registry file.
 * Returns `undefined` if the network has no registry, the key is missing, or the
 * address is the zero address placeholder. Throws if the registry entry is multi-address.
 */
export function getAddressFromRegistry(
  networkName: string,
  contractKey: string,
  envVarName?: string,
): string | undefined {
  if (!REGISTRY_NETWORKS.has(networkName)) {
    return undefined;
  }
  const registry = loadRegistry(networkName);
  if (!registry) {
    return undefined;
  }
  const entry = lookupRegistryEntry(registry, contractKey, envVarName);
  if (!entry) {
    return undefined;
  }
  if (isMultiRegistryEntry(entry)) {
    throw new Error(
      `Registry entry for "${contractKey}" on "${networkName}" contains an addresses array. ` +
        "Use getAddressesFromRegistry or requireAddressesFromRegistryOrEnv instead.",
    );
  }
  return getPopulatedAddresses(entry)?.[0];
}

/**
 * Reads multiple contract addresses from the per-network registry file.
 * Single-address entries are returned as a one-item array.
 */
export function getAddressesFromRegistry(
  networkName: string,
  contractKey: string,
  envVarName?: string,
): string[] | undefined {
  if (!REGISTRY_NETWORKS.has(networkName)) {
    return undefined;
  }
  const registry = loadRegistry(networkName);
  if (!registry) {
    return undefined;
  }
  const entry = lookupRegistryEntry(registry, contractKey, envVarName);
  if (!entry) {
    return undefined;
  }
  return getPopulatedAddresses(entry);
}

/**
 * Resolves an address for deployment, combining the registry with an optional env var.
 *
 * Resolution order and conflict rules:
 *
 * | Registry entry        | Env var set              | Outcome                                              |
 * |-----------------------|--------------------------|------------------------------------------------------|
 * | Present (non-zero)    | Not set                  | Returns registry address — env var is optional       |
 * | Present (non-zero)    | Matches registry         | Returns registry address                             |
 * | Present (non-zero)    | Conflicts with registry  | Hard fail — both values printed, deploy aborted      |
 * | Absent / zero         | Set (valid address)      | Returns env var value (validated and checksummed)    |
 * | Absent / zero         | Not set                  | Hard fail — no source available                      |
 *
 * Networks without a registry file (custom, zkevm_dev, l2) bypass registry lookup
 * entirely and fall back to the env var only.
 *
 * Lookup tries `contractKey` first, then `envVarName` when they differ.
 */
export function requireAddressFromRegistryOrEnv(networkName: string, contractKey: string, envVarName?: string): string {
  const handoffAddress = envVarName ? getHandoffAddress(envVarName) : undefined;
  if (handoffAddress !== undefined) {
    console.log(`Using in-process handoff address for ${contractKey}: ${handoffAddress}`);
    return handoffAddress;
  }

  const registryAddress = getAddressFromRegistry(networkName, contractKey, envVarName);
  const envAddress = envVarName ? parseEnvAddress(envVarName) : undefined;

  if (registryAddress !== undefined) {
    if (envAddress !== undefined && envAddress !== registryAddress) {
      throw new Error(
        `Address conflict for "${contractKey}" on network "${networkName}":\n` +
          `  Registry (contracts/deployments/addresses/${networkName}.json): ${registryAddress}\n` +
          `  Environment variable ${envVarName}: ${formatEnvVarValueForMessage(envVarName!, envAddress)}\n` +
          `Either remove the env var override or update the registry to match.`,
      );
    }
    console.log(
      `Using registry address for ${contractKey} on ${networkName}: ${registryAddress}${envAddress ? " (matches env var)" : ""}`,
    );
    return registryAddress;
  }

  if (envAddress !== undefined) {
    console.log(
      `Using environment variable ${formatEnvVarForLog(envVarName!, envAddress)} for ${contractKey} (no registry entry)`,
    );
    return envAddress;
  }

  const registryNote = REGISTRY_NETWORKS.has(networkName)
    ? ` Add it to contracts/deployments/addresses/${networkName}.json, or set env var ${envVarName ?? "<env var>"}.`
    : ` Set env var ${envVarName ?? "<env var>"}.`;

  throw new Error(`No address found for "${contractKey}" on network "${networkName}".${registryNote}`);
}

/**
 * Resolves multiple addresses for deployment, combining the registry with a comma-delimited env var.
 * Registry entries may use either a single `address` or an `addresses` array.
 * Env var conflicts are detected using order-independent set comparison.
 */
export function requireAddressesFromRegistryOrEnv(
  networkName: string,
  contractKey: string,
  envVarName: string,
): string[] {
  const registryAddresses = getAddressesFromRegistry(networkName, contractKey, envVarName);
  const envAddresses = parseEnvAddressList(envVarName);

  if (registryAddresses !== undefined) {
    if (envAddresses !== undefined && !sameAddressSet(registryAddresses, envAddresses)) {
      throw new Error(
        `Address conflict for "${contractKey}" on network "${networkName}":\n` +
          `  Registry (contracts/deployments/addresses/${networkName}.json): ${registryAddresses.join(", ")}\n` +
          `  Environment variable ${envVarName}: ${envAddresses.map((value) => formatEnvVarValueForMessage(envVarName, value)).join(", ")}\n` +
          `Either remove the env var override or update the registry to match.`,
      );
    }
    console.log(
      `Using registry addresses for ${contractKey} on ${networkName}: ${registryAddresses.join(", ")}${envAddresses ? " (matches env var)" : ""}`,
    );
    return registryAddresses;
  }

  if (envAddresses !== undefined) {
    console.log(
      `Using environment variable ${formatEnvVarForLog(envVarName, envAddresses.join(", "))} for ${contractKey} (no registry entry)`,
    );
    return envAddresses;
  }

  const registryNote = REGISTRY_NETWORKS.has(networkName)
    ? ` Add it to contracts/deployments/addresses/${networkName}.json, or set env var ${envVarName}.`
    : ` Set env var ${envVarName}.`;

  throw new Error(`No addresses found for "${contractKey}" on network "${networkName}".${registryNote}`);
}

/**
 * Like {@link requireAddressesFromRegistryOrEnv} but returns an empty array instead of
 * throwing when neither a registry entry nor an env var is present.
 *
 * Use this for optional address lists (e.g. reserved token addresses) where an empty
 * configuration is a valid deployment choice.
 */
export function getAddressesFromRegistryOrEnv(networkName: string, contractKey: string, envVarName: string): string[] {
  const registryAddresses = getAddressesFromRegistry(networkName, contractKey, envVarName);
  const envAddresses = parseEnvAddressList(envVarName);

  if (registryAddresses !== undefined) {
    if (envAddresses !== undefined && !sameAddressSet(registryAddresses, envAddresses)) {
      throw new Error(
        `Address conflict for "${contractKey}" on network "${networkName}":\n` +
          `  Registry (contracts/deployments/addresses/${networkName}.json): ${registryAddresses.join(", ")}\n` +
          `  Environment variable ${envVarName}: ${envAddresses.map((value) => formatEnvVarValueForMessage(envVarName, value)).join(", ")}\n` +
          `Either remove the env var override or update the registry to match.`,
      );
    }
    console.log(
      `Using registry addresses for ${contractKey} on ${networkName}: ${registryAddresses.join(", ")}${envAddresses ? " (matches env var)" : ""}`,
    );
    return registryAddresses;
  }

  if (envAddresses !== undefined) {
    console.log(
      `Using environment variable ${formatEnvVarForLog(envVarName, envAddresses.join(", "))} for ${contractKey} (no registry entry)`,
    );
    return envAddresses;
  }

  return [];
}

/**
 * @deprecated Use getAddressFromRegistry instead.
 *
 * Reads a deployed contract address from the legacy hardhat-deploy artefact format at
 * `deployments/<networkName>/<contractName>.json`. This path is no longer written by
 * deploy scripts; prefer the address registry under `deployments/addresses/`.
 */
export const getDeployedContractOnNetwork = async (
  networkName: string,
  contractName: string,
): Promise<string | undefined> => {
  const filePath = path.join(__dirname, "..", "..", "deployments", `${networkName}`, `${contractName}.json`);
  if (!fs.existsSync(filePath)) {
    return undefined;
  }
  const data = fs.readFileSync(filePath, "utf-8");
  const address: string = JSON.parse(data).address;
  return ethers.isAddress(address) ? ethers.getAddress(address) : undefined;
};
