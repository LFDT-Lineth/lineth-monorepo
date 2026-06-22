import { ethers } from "ethers";

import { formatEnvVarForLog, formatEnvVarValueForMessage } from "./envVarLogging";

export {
  envVarNameFromContext,
  formatEnvVarForLog,
  formatEnvVarValueForMessage,
  isEnvVarContext,
  isSensitiveEnvVar,
} from "./envVarLogging";

export function getRequiredEnvVar(name: string): string {
  const envValue = process.env[name];
  if (!envValue) {
    throw new Error(`Required environment variable "${name}" is missing or empty.`);
  }
  console.log(`Using environment variable ${formatEnvVarForLog(name, envValue)}`);
  return envValue;
}

export function getEnvVarOrDefault(envVar: string, defaultValue: unknown) {
  const envValue = process.env[envVar];

  if (!envValue) {
    console.log(`Using default ${envVar}`);
    return defaultValue;
  }

  console.log(`Using provided ${formatEnvVarForLog(envVar, envValue)}`);
  try {
    const parsedValue = JSON.parse(envValue);
    if (typeof parsedValue === "object" && !Array.isArray(parsedValue)) {
      return parsedValue;
    }

    if (Array.isArray(parsedValue) && parsedValue.every((item) => typeof item === "object")) {
      return parsedValue;
    }
  } catch {
    console.log(`Unable to parse ${envVar}, returning as string.`);
  }
  return envValue;
}

export function getOptionalEnvVar(name: string): string | undefined {
  const envValue = process.env[name];
  if (envValue === undefined) {
    return undefined;
  }
  return envValue;
}

/**
 * Reads an environment variable that must be a valid EVM address.
 * Throws immediately with a clear message if the value is missing or not a valid address.
 * Returns the EIP-55 checksummed form of the address.
 *
 * Use this for addresses that are not tracked in the deployed address registry
 * (e.g. ephemeral operator addresses, newly configured roles).
 * For registry-tracked addresses prefer `requireAddressFromRegistryOrEnv` from readAddress.ts.
 */
export function validateAddressEnvVar(name: string): string {
  const raw = getRequiredEnvVar(name);
  if (!ethers.isAddress(raw)) {
    throw new Error(
      `Environment variable "${name}" is not a valid EVM address. Got: "${formatEnvVarValueForMessage(name, raw)}". ` +
        `Expected a 0x-prefixed 40-hex-character address.`,
    );
  }
  return ethers.getAddress(raw);
}
