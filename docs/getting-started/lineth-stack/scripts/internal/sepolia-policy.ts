import {
  type EnvMap,
  type L1Mode,
  LOCAL_L1_CHAIN_ID,
  envValue,
  l1Mode,
} from "./deployer-wallet";

export {
  type EnvMap,
  type L1Config,
  type L1Context,
  type L1Mode,
  LOCAL_L1_CHAIN_ID,
  LOCAL_L1_CONTAINER_RPC_URL,
  LOCAL_L1_DEPLOYER_PRIVATE_KEY,
  LOCAL_L1_HOST_RPC_URL,
  L1_MODE_VALUES,
  envValue,
  l1Mode,
  readDotEnvContents,
  readDotEnvFile,
  requiredEnvValue,
  resolveL1Config,
} from "./deployer-wallet";

export type SepoliaPolicyConfig = {
  dataAvailabilityMode: string;
  l1DeployerMinBalanceWei: bigint;
  l1DeployGasPriceWei: bigint;
  l1DynamicGasPriceCapDisabled: boolean;
  l1BlobMaxFeePerGasCapWei: bigint;
  l1BlobMaxFeePerBlobGasCapWei: bigint;
  l1BlobMaxPriorityFeePerGasCapWei: bigint;
  l1FinalizationMaxFeePerGasCapWei: bigint;
  l1FinalizationMaxPriorityFeePerGasCapWei: bigint;
  l1RoleMinBalanceWei: bigint;
  l1RoleTopUpWei: bigint;
  l1PostmanMinBalanceWei: bigint;
  l1PostmanTopUpWei: bigint;
  l2RuntimeMinBalanceWei: bigint;
  l2RuntimeTopUpWei: bigint;
  l2GasPriceWei: bigint;
};

export type SepoliaPolicyProvider = {
  getNetwork(): Promise<{ chainId: bigint }>;
  getTransactionCount(address: string, blockTag: "latest" | "pending"): Promise<number>;
  getBalance(address: string): Promise<bigint>;
  getFeeData(): Promise<{ maxFeePerGas?: bigint | null; gasPrice?: bigint | null }>;
  getBlockNumber(): Promise<number>;
  send(method: string, params: unknown[]): Promise<unknown>;
};

export type SepoliaPolicyReport = {
  mode: L1Mode;
  config: SepoliaPolicyConfig;
  deployerAddress: string;
  chainId: bigint;
  latestNonce: number;
  pendingNonce: number;
  balanceWei: bigint;
  minimumBalanceWei: bigint;
  currentExecutionFeeWei?: bigint;
  blobBaseFeeWei?: bigint;
  l1AccountSetupBlockNumber: number;
  l1PostmanListenerStartBlock: number;
  warnings: string[];
};

export const SEPOLIA_CHAIN_ID = 11155111n;

export const SEPOLIA_POLICY_DEFAULTS = {
  L1_DEPLOYER_MIN_BALANCE_WEI: "2000000000000000000",
  L1_DEPLOY_GAS_PRICE_WEI: "5000000000",
  L1_ROLE_MIN_BALANCE_WEI: "400000000000000000",
  L1_ROLE_TOP_UP_WEI: "500000000000000000",
  L1_POSTMAN_MIN_BALANCE_WEI: "50000000000000000",
  L1_POSTMAN_TOP_UP_WEI: "100000000000000000",
  L1_DYNAMIC_GAS_PRICE_CAP_DISABLED: "true",
  L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI: "100000000000",
  L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI: "100000000000",
  L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI: "20000000000",
  L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI: "200000000000",
  L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI: "40000000000",
} as const;

export const LOCAL_L2_POLICY_DEFAULTS = {
  L2_RUNTIME_MIN_BALANCE_WEI: "1000000000000000000",
  L2_RUNTIME_TOP_UP_WEI: "10000000000000000000",
  L2_GAS_PRICE_WEI: "100000000",
} as const;

export function parseDecimalWei(name: string, raw: string): bigint {
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer wei value`);
  }
  return BigInt(raw);
}

export function parseBoolean(name: string, raw: string): boolean {
  if (raw === "true") {
    return true;
  }
  if (raw === "false") {
    return false;
  }
  throw new Error(`${name} must be true or false (got '${raw}')`);
}

export function buildSepoliaPolicyConfig(env: EnvMap): SepoliaPolicyConfig {
  const dataAvailabilityMode = envValue("LINEA_COORDINATOR_DATA_AVAILABILITY", env, "ROLLUP");
  if (dataAvailabilityMode !== "ROLLUP") {
    throw new Error(
      `LINEA_COORDINATOR_DATA_AVAILABILITY=${dataAvailabilityMode} is not supported by this quickstart; use ROLLUP`,
    );
  }

  // Each wei field reads env[name] and falls back to the matching default key;
  // the keyed helpers keep that (name, default) pairing impossible to mismatch.
  const sepoliaWei = (name: keyof typeof SEPOLIA_POLICY_DEFAULTS): bigint =>
    parseDecimalWei(name, envValue(name, env, SEPOLIA_POLICY_DEFAULTS[name]));
  const localL2Wei = (name: keyof typeof LOCAL_L2_POLICY_DEFAULTS): bigint =>
    parseDecimalWei(name, envValue(name, env, LOCAL_L2_POLICY_DEFAULTS[name]));

  return {
    dataAvailabilityMode,
    l1DeployerMinBalanceWei: sepoliaWei("L1_DEPLOYER_MIN_BALANCE_WEI"),
    l1DeployGasPriceWei: sepoliaWei("L1_DEPLOY_GAS_PRICE_WEI"),
    l1DynamicGasPriceCapDisabled: parseBoolean(
      "L1_DYNAMIC_GAS_PRICE_CAP_DISABLED",
      envValue("L1_DYNAMIC_GAS_PRICE_CAP_DISABLED", env, SEPOLIA_POLICY_DEFAULTS.L1_DYNAMIC_GAS_PRICE_CAP_DISABLED),
    ),
    l1BlobMaxFeePerGasCapWei: sepoliaWei("L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI"),
    l1BlobMaxFeePerBlobGasCapWei: sepoliaWei("L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI"),
    l1BlobMaxPriorityFeePerGasCapWei: sepoliaWei("L1_BLOB_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI"),
    l1FinalizationMaxFeePerGasCapWei: sepoliaWei("L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI"),
    l1FinalizationMaxPriorityFeePerGasCapWei: sepoliaWei("L1_FINALIZATION_MAX_PRIORITY_FEE_PER_GAS_CAP_WEI"),
    l1RoleMinBalanceWei: sepoliaWei("L1_ROLE_MIN_BALANCE_WEI"),
    l1RoleTopUpWei: sepoliaWei("L1_ROLE_TOP_UP_WEI"),
    l1PostmanMinBalanceWei: sepoliaWei("L1_POSTMAN_MIN_BALANCE_WEI"),
    l1PostmanTopUpWei: sepoliaWei("L1_POSTMAN_TOP_UP_WEI"),
    l2RuntimeMinBalanceWei: localL2Wei("L2_RUNTIME_MIN_BALANCE_WEI"),
    l2RuntimeTopUpWei: localL2Wei("L2_RUNTIME_TOP_UP_WEI"),
    l2GasPriceWei: localL2Wei("L2_GAS_PRICE_WEI"),
  };
}

export function sanitizeExternalError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return message
    .replace(/https?:\/\/[^\s)"']+/g, "<redacted-url>")
    .replace(/0x[a-fA-F0-9]{64}/g, "<redacted-hex>");
}

function requireCapAbove(name: string, cap: bigint, observed: bigint | undefined, observedLabel: string) {
  if (observed === undefined) {
    return;
  }
  if (observed > cap) {
    throw new Error(
      `${name}=${cap} is below current Sepolia ${observedLabel} ${observed}; increase ${name} before boot.`,
    );
  }
}

function decodeQuantity(value: unknown, name: string): bigint | undefined {
  if (typeof value !== "string" || value === "") {
    return undefined;
  }
  try {
    return BigInt(value);
  } catch {
    throw new Error(`${name} returned a non-quantity value`);
  }
}

export async function runSepoliaPolicyCheck(params: {
  provider: SepoliaPolicyProvider;
  deployerAddress: string;
  env: EnvMap;
}): Promise<SepoliaPolicyReport> {
  return runL1PolicyCheck({ ...params, env: { ...params.env, L1_MODE: "sepolia" } });
}

export async function runL1PolicyCheck(params: {
  provider: SepoliaPolicyProvider;
  deployerAddress: string;
  env: EnvMap;
}): Promise<SepoliaPolicyReport> {
  const mode = l1Mode(params.env);
  const config = buildSepoliaPolicyConfig(params.env);
  const warnings: string[] = [];

  const network = await params.provider.getNetwork();
  if (mode === "sepolia" && network.chainId !== SEPOLIA_CHAIN_ID) {
    throw new Error(`L1_RPC_URL must point to Sepolia chainId ${SEPOLIA_CHAIN_ID}; got ${network.chainId}`);
  }
  if (mode === "local" && network.chainId !== LOCAL_L1_CHAIN_ID) {
    throw new Error(`L1_RPC_URL must point to local L1 chainId ${LOCAL_L1_CHAIN_ID}; got ${network.chainId}`);
  }

  const latestNonce = await params.provider.getTransactionCount(params.deployerAddress, "latest");
  const pendingNonce = await params.provider.getTransactionCount(params.deployerAddress, "pending");
  if (mode === "sepolia" && pendingNonce !== latestNonce) {
    throw new Error(
      `L1 deployer has pending transactions (latest nonce ${latestNonce}, pending nonce ${pendingNonce}). ` +
        "Use a clean deployer account or wait for pending transactions before boot.",
    );
  }

  const balanceWei = await params.provider.getBalance(params.deployerAddress);
  const minimumBalanceWei = mode === "local" ? 1n : config.l1DeployerMinBalanceWei;
  if (mode === "local" && balanceWei === 0n) {
    throw new Error(`L1 deployer ${params.deployerAddress} has zero wei on local L1.`);
  }
  if (mode === "sepolia" && balanceWei < minimumBalanceWei) {
    throw new Error(
      `L1 deployer ${params.deployerAddress} has ${balanceWei} wei; ` +
        `fund it to at least ${minimumBalanceWei} wei.`,
    );
  }

  let currentExecutionFeeWei: bigint | undefined;
  let blobBaseFeeWei: bigint | undefined;
  if (mode === "sepolia") {
    const feeData = await params.provider.getFeeData();
    currentExecutionFeeWei = feeData.maxFeePerGas ?? feeData.gasPrice ?? undefined;
    requireCapAbove("L1_DEPLOY_GAS_PRICE_WEI", config.l1DeployGasPriceWei, currentExecutionFeeWei, "execution fee");
    requireCapAbove(
      "L1_BLOB_MAX_FEE_PER_GAS_CAP_WEI",
      config.l1BlobMaxFeePerGasCapWei,
      currentExecutionFeeWei,
      "execution fee",
    );
    requireCapAbove(
      "L1_FINALIZATION_MAX_FEE_PER_GAS_CAP_WEI",
      config.l1FinalizationMaxFeePerGasCapWei,
      currentExecutionFeeWei,
      "execution fee",
    );

    try {
      blobBaseFeeWei = decodeQuantity(await params.provider.send("eth_blobBaseFee", []), "eth_blobBaseFee");
    } catch (error) {
      warnings.push(`eth_blobBaseFee unavailable; blob-fee preflight skipped (${sanitizeExternalError(error)})`);
    }
    requireCapAbove(
      "L1_BLOB_MAX_FEE_PER_BLOB_GAS_CAP_WEI",
      config.l1BlobMaxFeePerBlobGasCapWei,
      blobBaseFeeWei,
      "blob base fee",
    );
  }

  const l1AccountSetupBlockNumber = await params.provider.getBlockNumber();
  const l1PostmanListenerStartBlock = Math.max(0, l1AccountSetupBlockNumber - 5);

  return {
    mode,
    config,
    deployerAddress: params.deployerAddress,
    chainId: network.chainId,
    latestNonce,
    pendingNonce,
    balanceWei,
    minimumBalanceWei,
    currentExecutionFeeWei,
    blobBaseFeeWei,
    l1AccountSetupBlockNumber,
    l1PostmanListenerStartBlock,
    warnings,
  };
}
