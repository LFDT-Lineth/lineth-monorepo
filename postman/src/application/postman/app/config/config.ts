import { LoggerOptions } from "winston";
import { z } from "zod";

import {
  claimingOptionsSchema,
  listenerOptionsSchema,
  networkOptionsSchema,
  l2NetworkOptionsSchema,
  dbCleanerOptionsSchema,
  apiOptionsSchema,
} from "./schema";
import { DBOptions, DBCleanerConfig } from "../../../../infrastructure/persistence/config/types";

import type { Address } from "../../../../core/types/primitives";
import type {
  SignerConfig,
  Web3SignerTlsConfig,
} from "../../../../infrastructure/blockchain/viem/signers/SignerConfig";

export type { SignerConfig, Web3SignerTlsConfig };

// Options types inferred from zod schemas
export type ClaimingOptions = z.infer<typeof claimingOptionsSchema>;
export type ListenerOptions = z.infer<typeof listenerOptionsSchema>;
type NetworkOptions = z.infer<typeof networkOptionsSchema>;
export type L1NetworkOptions = NetworkOptions;
export type L2NetworkOptions = z.infer<typeof l2NetworkOptionsSchema>;
export type DBCleanerOptions = z.infer<typeof dbCleanerOptionsSchema>;
export type ApiOptions = z.infer<typeof apiOptionsSchema>;

export type PostmanOptions = {
  l1Options: L1NetworkOptions;
  l2Options: L2NetworkOptions;
  l1L2AutoClaimEnabled: boolean;
  l2L1AutoClaimEnabled: boolean;
  databaseOptions: DBOptions;
  databaseCleanerOptions?: DBCleanerOptions;
  loggerOptions?: LoggerOptions;
  apiOptions?: ApiOptions;
};

// Config types (resolved, with defaults applied)
export type ClaimingConfig = Omit<Required<ClaimingOptions>, "feeRecipientAddress" | "claimViaAddress" | "signer"> & {
  signer: SignerConfig;
  feeRecipientAddress?: Address;
  claimViaAddress?: Address;
};

export type ListenerConfig = Required<Omit<ListenerOptions, "eventFilters">> &
  Partial<Pick<ListenerOptions, "eventFilters">>;

type NetworkConfig = {
  rpcUrl: string;
  messageServiceContractAddress: Address;
  isEOAEnabled: boolean;
  isCalldataEnabled: boolean;
  claiming: ClaimingConfig;
  listener: ListenerConfig;
};

export type L1NetworkConfig = NetworkConfig;

export type L2NetworkConfig = NetworkConfig & {
  l2MessageTreeDepth: number;
  enableLineaEstimateGas: boolean;
};

export type ApiConfig = Required<ApiOptions>;

export type PostmanConfig = {
  l1Config: L1NetworkConfig;
  l2Config: L2NetworkConfig;
  l1L2AutoClaimEnabled: boolean;
  l2L1AutoClaimEnabled: boolean;
  databaseOptions: DBOptions;
  databaseCleanerConfig: DBCleanerConfig;
  loggerOptions?: LoggerOptions;
  apiConfig: ApiConfig;
};
