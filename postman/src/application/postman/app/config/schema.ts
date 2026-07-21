import { isHex, size } from "viem/utils";
import { z } from "zod";

import type { Address } from "../../../../core/types/primitives";

const hexString = z
  .string()
  .regex(/^0x[0-9a-fA-F]+$/, "Must be a hex string starting with 0x")
  .describe("Hex string starting with 0x") as z.ZodType<`0x${string}`>;

const privateKeySchema = z
  .custom<`0x${string}`>((val) => {
    return isHex(val) && size(val) === 32;
  }, "Invalid private key")
  .describe("Hex-encoded 32-byte private key used to sign transactions");

const ethAddress = z
  .string()
  .regex(/^0x[0-9a-fA-F]{40}$/, "Must be a valid Ethereum address")
  .describe("Ethereum address (0x-prefixed, 20 bytes)") as z.ZodType<Address>;

const web3SignerTlsConfigSchema = z.object({
  keyStorePath: z.string().min(1).describe("Path to the PKCS12 client keystore used for mTLS with Web3Signer"),
  keyStorePassword: z.string().min(1).describe("Password for the Web3Signer client keystore"),
  trustStorePath: z.string().min(1).describe("Path to the PKCS12 truststore used to verify the Web3Signer server"),
  trustStorePassword: z.string().min(1).describe("Password for the Web3Signer truststore"),
});

const signerConfigSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("private-key").describe('Signer type: local private key ("private-key")'),
    privateKey: privateKeySchema,
  }),
  z.object({
    type: z.literal("web3signer").describe('Signer type: remote Web3Signer ("web3signer")'),
    endpoint: z.string().min(1).describe("Web3Signer HTTP(S) endpoint URL"),
    publicKey: hexString.describe("Public key of the Web3Signer signing account"),
    tls: web3SignerTlsConfigSchema.optional().describe("Optional mTLS settings for connecting to Web3Signer"),
  }),
  z.object({
    type: z.literal("aws-kms").describe('Signer type: AWS KMS ("aws-kms")'),
    kmsKeyId: z.string().min(1).describe("AWS KMS key ID or ARN used for signing"),
    region: z.string().min(1).optional().describe("AWS region of the KMS key (falls back to default SDK region)"),
  }),
]);

const calldataFilterSchema = z.object({
  criteriaExpression: z
    .string()
    .min(1)
    .describe("Filtrex expression used to filter MessageSent events by decoded calldata"),
  calldataFunctionInterface: z
    .string()
    .min(1)
    .describe(
      'Solidity function interface used to decode calldata (e.g. "function transfer(address to, uint256 amount)")',
    ),
});

const eventFiltersSchema = z.object({
  fromAddressFilter: ethAddress.optional().describe("Only process MessageSent events with this from address"),
  toAddressFilter: ethAddress.optional().describe("Only process MessageSent events with this to address"),
  calldataFilter: calldataFilterSchema.optional().describe("Optional calldata-based MessageSent event filter"),
});

export const listenerOptionsSchema = z.object({
  pollingInterval: z.number().positive().optional().describe("Interval in ms between block/event polling cycles"),
  receiptPollingInterval: z
    .number()
    .positive()
    .optional()
    .describe("Interval in ms between transaction receipt polling attempts"),
  initialFromBlock: z
    .number()
    .optional()
    .describe(
      "Starting block for event listening (-1 = latest/resume from DB, 0 = genesis, or a specific block number)",
    ),
  blockConfirmation: z
    .number()
    .nonnegative()
    .optional()
    .describe("Number of block confirmations required before processing an event"),
  maxFetchMessagesFromDb: z
    .number()
    .positive()
    .optional()
    .describe("Maximum number of messages to fetch from the database per batch"),
  maxBlocksToFetchLogs: z
    .number()
    .positive()
    .optional()
    .describe("Maximum number of blocks to request logs for in a single eth_getLogs call"),
  eventFilters: eventFiltersSchema.optional().describe("Optional filters applied to MessageSent events"),
});

export const claimingOptionsSchema = z.object({
  signer: signerConfigSchema.describe("Signer configuration used to submit claim transactions"),
  messageSubmissionTimeout: z
    .number()
    .positive()
    .optional()
    .describe("Timeout in ms waiting for a submitted claim transaction to be mined"),
  feeRecipientAddress: ethAddress
    .optional()
    .describe("Address that receives claim fees; defaults to the signer address when omitted"),
  maxNonceDiff: z
    .number()
    .positive()
    .optional()
    .describe("Maximum allowed difference between the database nonce and the on-chain nonce"),
  maxFeePerGasCap: z
    .bigint()
    .positive()
    .optional()
    .describe("Maximum maxFeePerGas (wei) allowed for claim transactions"),
  gasEstimationPercentile: z
    .number()
    .min(0)
    .max(100)
    .optional()
    .describe("Percentile (0–100) used when estimating gas fees from recent blocks"),
  isMaxGasFeeEnforced: z
    .boolean()
    .optional()
    .describe("When true, reject claims whose estimated fee exceeds maxFeePerGasCap"),
  profitMargin: z
    .number()
    .nonnegative()
    .optional()
    .describe("Minimum profit margin required before claiming a fee-paying message"),
  maxNumberOfRetries: z
    .number()
    .nonnegative()
    .optional()
    .describe("Maximum number of claim submission retries before giving up a cycle"),
  retryDelayInSeconds: z.number().positive().optional().describe("Delay in seconds between claim retry attempts"),
  maxClaimGasLimit: z.bigint().positive().optional().describe("Maximum gas limit allowed for a claim transaction"),
  maxBumpsPerCycle: z
    .number()
    .nonnegative()
    .optional()
    .describe("Maximum number of gas-fee bumps allowed within a single retry cycle"),
  maxRetryCycles: z
    .number()
    .nonnegative()
    .optional()
    .describe("Maximum number of full claim retry cycles before marking the message as failed"),
  isPostmanSponsorshipEnabled: z
    .boolean()
    .optional()
    .describe("When true, the Postman sponsors gas for eligible zero-fee claim transactions"),
  maxPostmanSponsorGasLimit: z
    .bigint()
    .positive()
    .optional()
    .describe("Maximum gas limit for Postman-sponsored claim transactions"),
  claimViaAddress: ethAddress
    .optional()
    .describe("Optional proxy/router address to call when submitting claim transactions"),
});

export const networkOptionsSchema = z.object({
  claiming: claimingOptionsSchema.describe("Claiming and signer options for this network"),
  listener: listenerOptionsSchema.describe("Event listener and polling options for this network"),
  rpcUrl: z.string().url().describe("JSON-RPC endpoint URL for the network node"),
  messageServiceContractAddress: ethAddress.describe("Address of the message service contract on this network"),
  isEOAEnabled: z.boolean().optional().describe("When true, process and claim EOA (empty calldata) messages"),
  isCalldataEnabled: z.boolean().optional().describe("When true, process and claim messages that carry calldata"),
});

export const l2NetworkOptionsSchema = networkOptionsSchema.extend({
  l2MessageTreeDepth: z.number().positive().optional().describe("Depth of the L2 message Merkle tree"),
  enableLineaEstimateGas: z
    .boolean()
    .optional()
    .describe("When true, use Linea's linea_estimateGas endpoint for L2 gas fee estimation"),
});

const dbOptionsSchema = z
  .object({
    type: z.literal("postgres").describe('Database type (currently only "postgres" is supported)'),
  })
  .passthrough()
  .describe("PostgreSQL connection options (host, port, credentials, SSL, etc.)");

export const dbCleanerOptionsSchema = z.object({
  enabled: z.boolean().describe("When true, periodically delete old claimed/finalized messages from the database"),
  cleaningInterval: z.number().positive().optional().describe("Interval in ms between database cleaning runs"),
  daysBeforeNowToDelete: z
    .number()
    .positive()
    .optional()
    .describe("Retain finalized messages for this many days before deleting them"),
});

export const apiOptionsSchema = z.object({
  port: z
    .number()
    .int()
    .positive()
    .max(65535)
    .optional()
    .describe("TCP port for the Express API exposing metrics and health endpoints"),
});

export const postmanOptionsSchema = z.object({
  l1Options: networkOptionsSchema.describe("L1 (Ethereum) network, listener, and claiming configuration"),
  l2Options: l2NetworkOptionsSchema.describe("L2 (Linea) network, listener, and claiming configuration"),
  l1L2AutoClaimEnabled: z.boolean().describe("When true, automatically claim L1→L2 messages"),
  l2L1AutoClaimEnabled: z.boolean().describe("When true, automatically claim L2→L1 messages"),
  databaseOptions: dbOptionsSchema.describe("PostgreSQL persistence configuration"),
  databaseCleanerOptions: dbCleanerOptionsSchema
    .optional()
    .describe("Optional settings for cleaning old finalized messages from the database"),
  loggerOptions: z.any().optional().describe("Winston logger options (e.g. log level)"),
  apiOptions: apiOptionsSchema.optional().describe("Optional HTTP API (metrics/health) settings"),
});
