import { Address, getAddress } from "viem";
import { z } from "zod";

import { AccountManager } from "../accounts/account-manager";

const urlSchema = z.instanceof(URL);
const addressSchema = z.custom<Address>((val) => getAddress(val));

export const BaseConfigSchema = z.object({
  rpcUrl: urlSchema,
  chainId: z.number().int(),
  accountManager: z.instanceof(AccountManager),
  dummyContractAddress: addressSchema,
});

export const L1ConfigSchema = BaseConfigSchema.extend({
  lineaRollupAddress: addressSchema,
  lineaRollupProxyAdminAddress: addressSchema,
  tokenBridgeAddress: addressSchema,
  l1TokenAddress: addressSchema,
});

export const BaseL2ConfigSchema = BaseConfigSchema.extend({
  l2MessageServiceAddress: addressSchema,
  l2TestContractAddress: addressSchema.optional(),
  besuNodeRpcUrl: urlSchema.optional(),
  besuFollowerNodeRpcUrl: urlSchema.optional(),
  tokenBridgeAddress: addressSchema,
  l2TokenAddress: addressSchema,
  l2SparseMerkleProofAddress: addressSchema,
  l2LineaSequencerUptimeFeedAddress: addressSchema,
  opcodeTesterAddress: addressSchema,
  shomeiEndpoint: urlSchema.optional(),
  shomeiFrontendEndpoint: urlSchema.optional(),
  sequencerEndpoint: urlSchema.optional(),
  transactionExclusionEndpoint: urlSchema.optional(),
});

export const LocalL2ConfigSchema = BaseL2ConfigSchema.extend({
  besuNodeRpcUrl: urlSchema,
  shomeiEndpoint: urlSchema,
  shomeiFrontendEndpoint: urlSchema,
  sequencerEndpoint: urlSchema,
  transactionExclusionEndpoint: urlSchema,
});

export const DevL2ConfigSchema = BaseL2ConfigSchema;
export const SepoliaL2ConfigSchema = BaseL2ConfigSchema;

export const L2ConfigSchema = z.union([LocalL2ConfigSchema, DevL2ConfigSchema, SepoliaL2ConfigSchema]);

export const ConfigSchema = z.object({
  L1: L1ConfigSchema,
  L2: L2ConfigSchema,
});

export type Config = z.infer<typeof ConfigSchema>;
export type LocalL2Config = z.infer<typeof LocalL2ConfigSchema>;
export type L1Config = z.infer<typeof L1ConfigSchema>;
export type L2Config = z.infer<typeof L2ConfigSchema>;
