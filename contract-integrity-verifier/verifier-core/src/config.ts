/**
 * Contract Integrity Verifier - Configuration Loading
 *
 * Handles loading and validating the verifier configuration file.
 * Supports both JSON and Markdown config formats.
 * Supports environment variable interpolation in config values.
 */

import { readFileSync, existsSync } from "fs";
import { resolve, dirname, extname } from "path";

import { VerifierConfig, ChainConfig, ContractConfig } from "./types";
import { parseMarkdownConfig } from "./utils/markdown-config";

/**
 * Interpolates environment variables in a string.
 * Supports ${VAR_NAME} syntax.
 * @param value - String containing ${VAR_NAME} placeholders
 * @param required - If true, throws when env var is missing. Default: true
 */
function interpolateEnvVars(value: string, required = true): string {
  return value.replace(/\$\{([^}]+)\}/g, (match, varName: string) => {
    const envValue = process.env[varName];
    if (envValue === undefined) {
      if (required) {
        throw new Error(`Environment variable '${varName}' is not set`);
      }
      return ""; // Return empty string for optional env vars
    }
    return envValue;
  });
}

/**
 * Interpolates environment variables in chain configs.
 * Used after markdown parsing to handle default chains.
 * Missing env vars result in empty rpcUrl (validated later when chain is used).
 */
function interpolateChainConfigs(chains: Record<string, ChainConfig>): void {
  for (const chain of Object.values(chains)) {
    if (chain.rpcUrl) {
      // Allow missing env vars - will be validated when chain is actually used
      chain.rpcUrl = interpolateEnvVars(chain.rpcUrl, false);
    }
    if (chain.explorerUrl) {
      chain.explorerUrl = interpolateEnvVars(chain.explorerUrl, false);
    }
  }
}

/**
 * Validates a chain configuration.
 * Note: rpcUrl can be empty for unused chains (validated when chain is referenced).
 */
function validateChainConfig(name: string, config: ChainConfig): void {
  if (!config.chainId || typeof config.chainId !== "number") {
    throw new Error(`Chain '${name}' must have a valid numeric chainId`);
  }
  // rpcUrl is validated when the chain is actually used by a contract
}

/**
 * Validates a contract configuration.
 * Also validates that the referenced chain has a valid rpcUrl.
 */
function validateContractConfig(config: ContractConfig, chains: Record<string, ChainConfig>): void {
  if (!config.name) {
    throw new Error("Contract must have a name");
  }
  if (!config.chain || !chains[config.chain]) {
    throw new Error(`Contract '${config.name}' references unknown chain '${config.chain}'`);
  }
  // Validate that the referenced chain has an rpcUrl
  const chain = chains[config.chain];
  if (!chain.rpcUrl) {
    throw new Error(
      `Contract '${config.name}' uses chain '${config.chain}' which has no rpcUrl. ` +
        `Set the environment variable or define the chain explicitly.`,
    );
  }
  if (!config.address || !/^0x[a-fA-F0-9]{40}$/.test(config.address)) {
    throw new Error(`Contract '${config.name}' must have a valid address`);
  }
  if (!config.artifactFile) {
    throw new Error(`Contract '${config.name}' must have an artifactFile`);
  }
}

/**
 * Resolves file paths relative to the config file location.
 */
function resolveFilePaths(config: VerifierConfig, configDir: string): void {
  for (const contract of config.contracts) {
    contract.artifactFile = resolve(configDir, contract.artifactFile);
  }
}

/**
 * Loads a JSON configuration file.
 */
function loadJsonConfig(configPath: string): VerifierConfig {
  const absolutePath = resolve(configPath);

  let rawConfig: string;
  try {
    rawConfig = readFileSync(absolutePath, "utf-8");
  } catch (error) {
    throw new Error(`Failed to read configuration file: ${error}`);
  }

  // Interpolate environment variables before parsing
  const interpolatedConfig = interpolateEnvVars(rawConfig);

  let config: VerifierConfig;
  try {
    config = JSON.parse(interpolatedConfig) as VerifierConfig;
  } catch (error) {
    throw new Error(`Failed to parse configuration file as JSON: ${error}`);
  }

  // Resolve relative file paths
  resolveFilePaths(config, dirname(absolutePath));

  return config;
}

/**
 * Loads and validates a configuration file.
 * Supports both JSON (.json) and Markdown (.md) formats.
 */
export function loadConfig(configPath: string): VerifierConfig {
  const absolutePath = resolve(configPath);

  if (!existsSync(absolutePath)) {
    throw new Error(`Configuration file not found: ${absolutePath}`);
  }

  const ext = extname(absolutePath).toLowerCase();
  let config: VerifierConfig;

  if (ext === ".md") {
    // Load markdown config
    const rawContent = readFileSync(absolutePath, "utf-8");
    config = parseMarkdownConfig(rawContent, dirname(absolutePath));
    // Interpolate env vars in chain configs (default chains have placeholders)
    interpolateChainConfigs(config.chains);
  } else {
    // Default to JSON
    config = loadJsonConfig(absolutePath);
  }

  // Validate chains
  if (!config.chains || typeof config.chains !== "object") {
    throw new Error("Configuration must have a 'chains' object");
  }
  for (const [name, chainConfig] of Object.entries(config.chains)) {
    validateChainConfig(name, chainConfig);
  }

  // Validate contracts
  if (!config.contracts || !Array.isArray(config.contracts)) {
    throw new Error("Configuration must have a 'contracts' array");
  }
  for (const contract of config.contracts) {
    validateContractConfig(contract, config.chains);
  }

  return config;
}

/**
 * Checks if the artifact file exists for a contract configuration.
 */
export function checkArtifactExists(contract: ContractConfig): boolean {
  return existsSync(contract.artifactFile);
}
