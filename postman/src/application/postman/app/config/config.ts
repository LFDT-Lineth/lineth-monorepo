import { LoggerOptions } from "winston";
import { DBOptions, DBCleanerOptions, DBCleanerConfig } from "../../persistence/config/types";

type DeepRequired<T> = {
  [P in keyof T]-?: T[P] extends object ? DeepRequired<T[P]> : T[P];
};

/**
 * Configuration for the Postman service, including network configurations, database options, and logging.
 */
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

/**
 * Configuration for the Postman service, including network configurations, database options, and logging.
 */
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

/**
 * Base configuration for a network, including claiming, listener settings, and contract details.
 */
type NetworkOptions = {
  claiming: ClaimingOptions;
  listener: ListenerOptions;
  rpcUrl: string;
  messageServiceContractAddress: string;
  isEOAEnabled?: boolean;
  isCalldataEnabled?: boolean;
};

type NetworkConfig = Omit<DeepRequired<NetworkOptions>, "claiming" | "listener"> & {
  claiming: ClaimingConfig;
  listener: ListenerConfig;
};

/**
 * Configuration specific to the L1 network, extending the base NetworkConfig.
 */
export type L1NetworkOptions = NetworkOptions;

export type L1NetworkConfig = NetworkConfig;

/**
 * Configuration specific to the L2 network, extending the base NetworkConfig with additional options.
 */
export type L2NetworkOptions = NetworkOptions & {
  l2MessageTreeDepth?: number;
  enableLineaEstimateGas?: boolean;
};

export type L2NetworkConfig = NetworkConfig & {
  l2MessageTreeDepth: number;
  enableLineaEstimateGas: boolean;
};

/**
 * Configuration for claiming operations, including signer details, fee settings, and retry policies.
 */
export type ClaimingOptions = {
  signerPrivateKey: string;
  messageSubmissionTimeout?: number;
  feeRecipientAddress?: string;
  maxNonceDiff?: number;
  maxFeePerGasCap?: bigint;
  gasEstimationPercentile?: number;
  isMaxGasFeeEnforced?: boolean;
  profitMargin?: number;
  maxNumberOfRetries?: number;
  retryDelayInSeconds?: number;
  maxClaimGasLimit?: bigint;
  maxTxRetries?: number;
  isPostmanSponsorshipEnabled?: boolean;
  maxPostmanSponsorGasLimit?: bigint;
};

export type ClaimingConfig = Omit<Required<ClaimingOptions>, "feeRecipientAddress"> & {
  feeRecipientAddress?: string;
};

/**
 * Configuration for the event listener, including polling settings and block fetching limits.
 */
export type ListenerOptions = {
  pollingInterval?: number;
  initialFromBlock?: number;
  blockConfirmation?: number;
  maxFetchMessagesFromDb?: number;
  maxBlocksToFetchLogs?: number;
  eventFilters?: {
    fromAddressFilter?: string;
    toAddressFilter?: string;
    calldataFilter?: {
      criteriaExpression: string;
      calldataFunctionInterface: string;
    };
  };
};

export type ListenerConfig = Required<Omit<ListenerOptions, "eventFilters">> &
  Partial<Pick<ListenerOptions, "eventFilters">>;

export type ApiOptions = {
  port?: number;
};

export type ApiConfig = Required<ApiOptions>;
