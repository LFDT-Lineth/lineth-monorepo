import { isHex, size } from "viem/utils";
import { z } from "zod";

import type { Address } from "../../../../core/types/primitives";

const hexString = z
  .string()
  .regex(/^0x[0-9a-fA-F]+$/, "Must be a hex string starting with 0x") as z.ZodType<`0x${string}`>;

const privateKeySchema = z.custom<`0x${string}`>((val) => {
  return isHex(val) && size(val) === 32;
}, "Invalid private key");

const ethAddress = z.string().regex(/^0x[0-9a-fA-F]{40}$/, "Must be a valid Ethereum address") as z.ZodType<Address>;

const web3SignerTlsConfigSchema = z.object({
  keyStorePath: z.string().min(1),
  keyStorePassword: z.string().min(1),
  trustStorePath: z.string().min(1),
  trustStorePassword: z.string().min(1),
});

const signerConfigSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("private-key"),
    privateKey: privateKeySchema,
  }),
  z.object({
    type: z.literal("web3signer"),
    endpoint: z.string().min(1),
    publicKey: hexString,
    tls: web3SignerTlsConfigSchema.optional(),
  }),
]);

const calldataFilterSchema = z.object({
  criteriaExpression: z.string().min(1),
  calldataFunctionInterface: z.string().min(1),
});

const eventFiltersSchema = z.object({
  fromAddressFilter: ethAddress.optional(),
  toAddressFilter: ethAddress.optional(),
  calldataFilter: calldataFilterSchema.optional(),
});

export const listenerOptionsSchema = z.object({
  pollingInterval: z.number().positive().optional(),
  receiptPollingInterval: z.number().positive().optional(),
  initialFromBlock: z.number().optional(),
  blockConfirmation: z.number().nonnegative().optional(),
  maxFetchMessagesFromDb: z.number().positive().optional(),
  maxBlocksToFetchLogs: z.number().positive().optional(),
  eventFilters: eventFiltersSchema.optional(),
});

export const claimingOptionsSchema = z.object({
  signer: signerConfigSchema,
  messageSubmissionTimeout: z.number().positive().optional(),
  feeRecipientAddress: ethAddress.optional(),
  maxNonceDiff: z.number().positive().optional(),
  maxFeePerGasCap: z.bigint().positive().optional(),
  gasEstimationPercentile: z.number().min(0).max(100).optional(),
  isMaxGasFeeEnforced: z.boolean().optional(),
  profitMargin: z.number().nonnegative().optional(),
  maxNumberOfRetries: z.number().nonnegative().optional(),
  retryDelayInSeconds: z.number().positive().optional(),
  maxClaimGasLimit: z.bigint().positive().optional(),
  maxBumpsPerCycle: z.number().nonnegative().optional(),
  maxRetryCycles: z.number().nonnegative().optional(),
  isPostmanSponsorshipEnabled: z.boolean().optional(),
  maxPostmanSponsorGasLimit: z.bigint().positive().optional(),
  claimViaAddress: ethAddress.optional(),
});

export const networkOptionsSchema = z.object({
  claiming: claimingOptionsSchema,
  listener: listenerOptionsSchema,
  rpcUrl: z.string().url(),
  messageServiceContractAddress: ethAddress,
  isEOAEnabled: z.boolean().optional(),
  isCalldataEnabled: z.boolean().optional(),
});

export const l2NetworkOptionsSchema = networkOptionsSchema.extend({
  l2MessageTreeDepth: z.number().positive().optional(),
  enableLineaEstimateGas: z.boolean().optional(),
});

const dbOptionsSchema = z
  .object({
    type: z.literal("postgres"),
  })
  .passthrough();

export const dbCleanerOptionsSchema = z.object({
  enabled: z.boolean(),
  cleaningInterval: z.number().positive().optional(),
  daysBeforeNowToDelete: z.number().positive().optional(),
});

export const apiOptionsSchema = z.object({
  port: z.number().int().positive().max(65535).optional(),
});

export const postmanOptionsSchema = z.object({
  l1Options: networkOptionsSchema,
  l2Options: l2NetworkOptionsSchema,
  l1L2AutoClaimEnabled: z.boolean(),
  l2L1AutoClaimEnabled: z.boolean(),
  databaseOptions: dbOptionsSchema,
  databaseCleanerOptions: dbCleanerOptionsSchema.optional(),
  loggerOptions: z.any().optional(),
  apiOptions: apiOptionsSchema.optional(),
});
