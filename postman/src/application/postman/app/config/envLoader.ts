import { PostmanOptions } from "./config";
import { SignerConfig } from "../../../../infrastructure/blockchain/viem/signers/SignerConfig";

function normalizePrivateKeyHex(raw: string): `0x${string}` {
  return (raw.startsWith("0x") ? raw : `0x${raw}`) as `0x${string}`;
}

function buildSignerConfig(prefix: "L1" | "L2"): SignerConfig {
  const signerType = process.env[`${prefix}_SIGNER_TYPE`] ?? "private-key";

  if (signerType === "web3signer") {
    return {
      type: "web3signer",
      endpoint: process.env[`${prefix}_WEB3_SIGNER_ENDPOINT`] ?? "",
      publicKey: (process.env[`${prefix}_WEB3_SIGNER_PUBLIC_KEY`] ?? "0x") as `0x${string}`,
      ...(process.env[`${prefix}_WEB3_SIGNER_TLS_KEYSTORE_PATH`]
        ? {
            tls: {
              keyStorePath: process.env[`${prefix}_WEB3_SIGNER_TLS_KEYSTORE_PATH`]!,
              keyStorePassword: process.env[`${prefix}_WEB3_SIGNER_TLS_KEYSTORE_PASSWORD`] ?? "",
              trustStorePath: process.env[`${prefix}_WEB3_SIGNER_TLS_TRUSTSTORE_PATH`] ?? "",
              trustStorePassword: process.env[`${prefix}_WEB3_SIGNER_TLS_TRUSTSTORE_PASSWORD`] ?? "",
            },
          }
        : {}),
    };
  }

  if (signerType === "aws-kms") {
    const region = process.env[`${prefix}_AWS_KMS_REGION`];
    return {
      type: "aws-kms",
      kmsKeyId: process.env[`${prefix}_AWS_KMS_KEY_ID`] ?? "",
      ...(region ? { region } : {}),
    };
  }

  return {
    type: "private-key",
    privateKey: normalizePrivateKeyHex(process.env[`${prefix}_SIGNER_PRIVATE_KEY`] ?? "0x"),
  };
}

function optionalInt(envVar: string | undefined): number | undefined {
  return envVar ? parseInt(envVar) : undefined;
}

function optionalBigInt(envVar: string | undefined): bigint | undefined {
  return envVar ? BigInt(envVar) : undefined;
}

function optionalFloat(envVar: string | undefined): number | undefined {
  return envVar ? parseFloat(envVar) : undefined;
}

function buildListenerOptions(prefix: "L1" | "L2") {
  return {
    pollingInterval: optionalInt(process.env[`${prefix}_LISTENER_INTERVAL`]),
    receiptPollingInterval: optionalInt(process.env[`${prefix}_LISTENER_RECEIPT_POLLING_INTERVAL`]),
    maxFetchMessagesFromDb: optionalInt(process.env.MAX_FETCH_MESSAGES_FROM_DB),
    maxBlocksToFetchLogs: optionalInt(process.env[`${prefix}_MAX_BLOCKS_TO_FETCH_LOGS`]),
    ...(parseInt(process.env[`${prefix}_LISTENER_INITIAL_FROM_BLOCK`] ?? "") >= 0
      ? { initialFromBlock: parseInt(process.env[`${prefix}_LISTENER_INITIAL_FROM_BLOCK`]!) }
      : {}),
    ...(parseInt(process.env[`${prefix}_LISTENER_BLOCK_CONFIRMATION`] ?? "") >= 0
      ? { blockConfirmation: parseInt(process.env[`${prefix}_LISTENER_BLOCK_CONFIRMATION`]!) }
      : {}),
    ...buildEventFilters(prefix),
  };
}

function buildEventFilters(prefix: "L1" | "L2") {
  const fromAddr = process.env[`${prefix}_EVENT_FILTER_FROM_ADDRESS`];
  const toAddr = process.env[`${prefix}_EVENT_FILTER_TO_ADDRESS`];
  const calldataExpr = process.env[`${prefix}_EVENT_FILTER_CALLDATA`];
  const calldataIface = process.env[`${prefix}_EVENT_FILTER_CALLDATA_FUNCTION_INTERFACE`];

  if (!fromAddr && !toAddr && !(calldataExpr && calldataIface)) return {};

  return {
    eventFilters: {
      fromAddressFilter: fromAddr as `0x${string}` | undefined,
      toAddressFilter: toAddr as `0x${string}` | undefined,
      ...(calldataExpr && calldataIface
        ? {
            calldataFilter: {
              criteriaExpression: calldataExpr,
              calldataFunctionInterface: calldataIface,
            },
          }
        : {}),
    },
  };
}

function buildClaimingOptions(prefix: "L1" | "L2") {
  const opposite = prefix === "L1" ? "L2" : "L1";
  return {
    signer: buildSignerConfig(prefix),
    messageSubmissionTimeout: optionalInt(process.env.MESSAGE_SUBMISSION_TIMEOUT),
    maxNonceDiff: optionalInt(process.env.MAX_NONCE_DIFF),
    maxFeePerGasCap: optionalBigInt(process.env.MAX_FEE_PER_GAS_CAP),
    gasEstimationPercentile: optionalInt(process.env.GAS_ESTIMATION_PERCENTILE),
    profitMargin: optionalFloat(process.env.PROFIT_MARGIN),
    maxNumberOfRetries: optionalInt(process.env.MAX_NUMBER_OF_RETRIES),
    retryDelayInSeconds: optionalInt(process.env.RETRY_DELAY_IN_SECONDS),
    maxClaimGasLimit: optionalBigInt(process.env.MAX_CLAIM_GAS_LIMIT),
    maxBumpsPerCycle: optionalInt(process.env.MAX_BUMPS_PER_CYCLE),
    maxRetryCycles: optionalInt(process.env.MAX_RETRY_CYCLES),
    isMaxGasFeeEnforced: process.env[`${prefix}_MAX_GAS_FEE_ENFORCED`] === "true",
    isPostmanSponsorshipEnabled: process.env[`${opposite}_${prefix}_ENABLE_POSTMAN_SPONSORING`] === "true",
    maxPostmanSponsorGasLimit: optionalBigInt(process.env.MAX_POSTMAN_SPONSOR_GAS_LIMIT),
    claimViaAddress: process.env[`${prefix}_CLAIM_VIA_ADDRESS`] as `0x${string}` | undefined,
  };
}

export function loadPostmanOptionsFromEnv(): PostmanOptions {
  return {
    l1Options: {
      rpcUrl: process.env.L1_RPC_URL ?? "",
      messageServiceContractAddress: (process.env.L1_CONTRACT_ADDRESS ?? "") as `0x${string}`,
      isEOAEnabled: process.env.L1_L2_EOA_ENABLED === "true",
      isCalldataEnabled: process.env.L1_L2_CALLDATA_ENABLED === "true",
      listener: buildListenerOptions("L1"),
      claiming: buildClaimingOptions("L1"),
    },
    l2Options: {
      rpcUrl: process.env.L2_RPC_URL ?? "",
      messageServiceContractAddress: (process.env.L2_CONTRACT_ADDRESS ?? "") as `0x${string}`,
      isEOAEnabled: process.env.L2_L1_EOA_ENABLED === "true",
      isCalldataEnabled: process.env.L2_L1_CALLDATA_ENABLED === "true",
      listener: buildListenerOptions("L2"),
      claiming: buildClaimingOptions("L2"),
      l2MessageTreeDepth: optionalInt(process.env.L2_MESSAGE_TREE_DEPTH),
      enableLineaEstimateGas: process.env.ENABLE_LINEA_ESTIMATE_GAS === "true",
    },
    l1L2AutoClaimEnabled: process.env.L1_L2_AUTO_CLAIM_ENABLED === "true",
    l2L1AutoClaimEnabled: process.env.L2_L1_AUTO_CLAIM_ENABLED === "true",
    loggerOptions: {
      level: process.env.LOG_LEVEL ?? "info",
    },
    databaseOptions: {
      type: "postgres",
      host: process.env.POSTGRES_HOST ?? "127.0.0.1",
      port: parseInt(process.env.POSTGRES_PORT ?? "5432"),
      username: process.env.POSTGRES_USER ?? "postgres",
      password: process.env.POSTGRES_PASSWORD ?? "postgres",
      database: process.env.POSTGRES_DB ?? "postman_db",
      ...(process.env.POSTGRES_SSL === "true"
        ? {
            ssl: {
              rejectUnauthorized: process.env.POSTGRES_SSL_REJECT_UNAUTHORIZED === "true",
              ca: process.env.POSTGRES_SSL_CA_PATH ?? undefined,
            },
          }
        : {}),
    },
    databaseCleanerOptions: {
      enabled: process.env.DB_CLEANER_ENABLED === "true",
      cleaningInterval: optionalInt(process.env.DB_CLEANING_INTERVAL),
      daysBeforeNowToDelete: optionalInt(process.env.DB_DAYS_BEFORE_NOW_TO_DELETE),
    },
    apiOptions: {
      port: optionalInt(process.env.API_PORT),
    },
  };
}
