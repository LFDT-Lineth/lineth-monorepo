import { getAddress, isAddress, ZeroAddress } from "ethers";

const UINT256_MAX = (1n << 256n) - 1n;
const SECONDS_PER_DAY = 86_400;
const DEFAULT_RATE_LIMIT_PERIOD_SECONDS = SECONDS_PER_DAY.toString();
// 1000 tokens at 18 decimals.
const DEFAULT_RATE_LIMIT_AMOUNT_WEI = (1000n * 10n ** 18n).toString();

/**
 * Env vars whose values must never appear in logs. Shared with `cli.ts` so
 * the top-level error handler can redact them even when `loadConfig()`
 * itself throws before producing a `DeployerConfig.sensitiveValues`.
 */
export const SENSITIVE_ENV_VAR_NAMES = [
  "L1_DEPLOYER_PRIVATE_KEY",
  "L2_DEPLOYER_PRIVATE_KEY",
  "L1_RPC_URL",
  "L2_RPC_URL",
] as const;

export interface DeployerConfig {
  l1RpcUrl: string;
  l2RpcUrl: string;
  l1PrivateKey: string;
  l2PrivateKey: string;
  l1StartingNonce: number;
  l2StartingNonce: number;
  artifactDigest: string;
  initialL2StateRootHash: string;
  rateLimitPeriod: string;
  rateLimitAmount: string;
  sensitiveValues: string[];
  expectedL1DeployerAddress?: string;
  expectedL2DeployerAddress?: string;
  expectedL1ChainId?: string;
  expectedL2ChainId?: string;
  l1SecurityCouncilOverride?: string;
  l1RollupOperatorsOverride?: string[];
  l2SecurityCouncilOverride?: string;
  l1L2MessageSetterOverride?: string;
  l2GenesisTimestampOverride?: string;
  l1DeployGasPriceWei?: string;
  l2DeployGasPriceWei?: string;
  checkpointFile?: string;
  bootstrapManifestFile?: string;
  bootstrapScriptsDir?: string;
}

export interface RoleConfig {
  l1SecurityCouncil: string;
  l1RollupOperators: string[];
  l2SecurityCouncil: string;
  l1L2MessageSetter: string;
}

function requireValue(env: NodeJS.ProcessEnv, name: string): string {
  const value = env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function optionalValue(env: NodeJS.ProcessEnv, name: string): string | undefined {
  const value = env[name]?.trim();
  return value ? value : undefined;
}

function assertUint256(value: string, name: string, allowZero = true): string {
  if (!/^[0-9]+$/.test(value)) throw new Error(`${name} must be a non-negative integer`);
  const parsed = BigInt(value);
  if (!allowZero && parsed === 0n) throw new Error(`${name} must be greater than zero`);
  if (parsed > UINT256_MAX) throw new Error(`${name} must fit uint256`);
  return value;
}

function requireStartingNonce(env: NodeJS.ProcessEnv, name: string): number {
  const value = assertUint256(requireValue(env, name), name);
  const parsed = BigInt(value);
  if (parsed > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error(`${name} must fit a safe integer`);
  return Number(parsed);
}

function requireSha256Digest(env: NodeJS.ProcessEnv, name: string): string {
  const value = requireValue(env, name);
  if (!/^sha256:[0-9a-f]{64}$/.test(value)) throw new Error(`${name} must be a sha256 digest`);
  return value;
}

function assertRpcUrl(value: string, name: string): string {
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("unsupported protocol");
  } catch {
    throw new Error(`${name} must be an HTTP(S) URL`);
  }
  return value;
}

function optionalAddress(env: NodeJS.ProcessEnv, name: string, allowZero = true): string | undefined {
  const value = optionalValue(env, name);
  if (value === undefined) return undefined;
  if (!isAddress(value)) throw new Error(`${name} must be an Ethereum address`);
  const address = getAddress(value);
  if (!allowZero && address === ZeroAddress) throw new Error(`${name} must not be the zero address`);
  return address;
}

function optionalAddressList(env: NodeJS.ProcessEnv, name: string, allowZero = true): string[] | undefined {
  const value = optionalValue(env, name);
  if (value === undefined) return undefined;
  const addresses = value.split(",").map((address) => address.trim());
  if (addresses.some((address) => !isAddress(address))) {
    throw new Error(`${name} must be a comma-separated list of Ethereum addresses`);
  }
  return addresses.map((address) => {
    const normalized = getAddress(address);
    if (!allowZero && normalized === ZeroAddress) throw new Error(`${name} must not contain the zero address`);
    return normalized;
  });
}

export function loadConfig(env: NodeJS.ProcessEnv = process.env): DeployerConfig {
  const profile = optionalValue(env, "DEPLOYMENT_PROFILE");
  if (profile !== undefined && profile !== "forge-dev-v1") {
    throw new Error(`DEPLOYMENT_PROFILE must be forge-dev-v1`);
  }

  const initialL2StateRootHash = requireValue(env, "INITIAL_L2_STATE_ROOT_HASH");
  if (!/^0x[0-9a-fA-F]{64}$/.test(initialL2StateRootHash)) {
    throw new Error("INITIAL_L2_STATE_ROOT_HASH must be a 32-byte hex value");
  }

  const l1RpcUrl = assertRpcUrl(requireValue(env, "L1_RPC_URL"), "L1_RPC_URL");
  const l2RpcUrl = assertRpcUrl(requireValue(env, "L2_RPC_URL"), "L2_RPC_URL");
  const l1PrivateKey = requireValue(env, "L1_DEPLOYER_PRIVATE_KEY");
  const l2PrivateKey = requireValue(env, "L2_DEPLOYER_PRIVATE_KEY");
  const l1StartingNonce = requireStartingNonce(env, "L1_STARTING_NONCE");
  const l2StartingNonce = requireStartingNonce(env, "L2_STARTING_NONCE");
  const artifactDigest = requireSha256Digest(env, "DEPLOYER_IMAGE_DIGEST");
  const rateLimitPeriod = assertUint256(
    optionalValue(env, "CONTRACT_RATE_LIMIT_PERIOD") ?? DEFAULT_RATE_LIMIT_PERIOD_SECONDS,
    "CONTRACT_RATE_LIMIT_PERIOD",
    false,
  );
  const rateLimitAmount = assertUint256(
    optionalValue(env, "CONTRACT_RATE_LIMIT_AMOUNT") ?? DEFAULT_RATE_LIMIT_AMOUNT_WEI,
    "CONTRACT_RATE_LIMIT_AMOUNT",
    false,
  );

  const config: DeployerConfig = {
    l1RpcUrl,
    l2RpcUrl,
    l1PrivateKey,
    l2PrivateKey,
    l1StartingNonce,
    l2StartingNonce,
    artifactDigest,
    initialL2StateRootHash,
    rateLimitPeriod,
    rateLimitAmount,
    sensitiveValues: [l1PrivateKey, l2PrivateKey, l1RpcUrl, l2RpcUrl],
  };

  const optionalEntries: Array<[keyof DeployerConfig, string | string[] | undefined]> = [
    ["expectedL1DeployerAddress", optionalAddress(env, "L1_DEPLOYER_ADDRESS")],
    ["expectedL2DeployerAddress", optionalAddress(env, "L2_DEPLOYER_ADDRESS")],
    ["expectedL1ChainId", optionalValue(env, "EXPECTED_L1_CHAIN_ID")],
    ["expectedL2ChainId", optionalValue(env, "EXPECTED_L2_CHAIN_ID")],
    ["l1SecurityCouncilOverride", optionalAddress(env, "L1_SECURITY_COUNCIL", false)],
    ["l1RollupOperatorsOverride", optionalAddressList(env, "LINETH_ROLLUP_OPERATORS", false)],
    ["l2SecurityCouncilOverride", optionalAddress(env, "L2_SECURITY_COUNCIL", false)],
    ["l1L2MessageSetterOverride", optionalAddress(env, "L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER", false)],
    ["l2GenesisTimestampOverride", optionalValue(env, "L2_GENESIS_TIMESTAMP")],
    ["l1DeployGasPriceWei", optionalValue(env, "L1_DEPLOY_GAS_PRICE_WEI")],
    ["l2DeployGasPriceWei", optionalValue(env, "L2_DEPLOY_GAS_PRICE_WEI")],
    ["checkpointFile", optionalValue(env, "CHECKPOINT_FILE")],
    ["bootstrapManifestFile", optionalValue(env, "BOOTSTRAP_MANIFEST_FILE")],
    ["bootstrapScriptsDir", optionalValue(env, "BOOTSTRAP_SCRIPTS_DIR")],
  ];
  for (const [key, value] of optionalEntries) {
    if (value !== undefined) Object.assign(config, { [key]: value });
  }

  if (config.l1DeployGasPriceWei !== undefined) {
    assertUint256(config.l1DeployGasPriceWei, "L1_DEPLOY_GAS_PRICE_WEI");
  }
  if (config.l2DeployGasPriceWei !== undefined) {
    assertUint256(config.l2DeployGasPriceWei, "L2_DEPLOY_GAS_PRICE_WEI");
  }
  if (config.expectedL1ChainId !== undefined) {
    assertUint256(config.expectedL1ChainId, "EXPECTED_L1_CHAIN_ID", false);
  }
  if (config.expectedL2ChainId !== undefined) {
    assertUint256(config.expectedL2ChainId, "EXPECTED_L2_CHAIN_ID", false);
  }

  return config;
}

export function resolveRoleConfig(config: DeployerConfig, l1Deployer: string, l2Deployer: string): RoleConfig {
  if (!isAddress(l1Deployer) || !isAddress(l2Deployer)) {
    throw new Error("deployer addresses must be valid Ethereum addresses");
  }
  const normalizedL1 = getAddress(l1Deployer);
  const normalizedL2 = getAddress(l2Deployer);
  return {
    l1SecurityCouncil: config.l1SecurityCouncilOverride ?? normalizedL1,
    l1RollupOperators: config.l1RollupOperatorsOverride ?? [normalizedL1],
    l2SecurityCouncil: config.l2SecurityCouncilOverride ?? normalizedL2,
    l1L2MessageSetter: config.l1L2MessageSetterOverride ?? normalizedL2,
  };
}

export function sanitizeText(text: string, sensitiveValues: readonly string[]): string {
  const redacted = [...new Set(sensitiveValues)]
    .filter(Boolean)
    .sort((left, right) => right.length - left.length)
    .reduce((sanitized, value) => sanitized.split(value).join("[REDACTED]"), text);
  return redacted.replace(/[\r\n\u2028\u2029]+/g, " ");
}
