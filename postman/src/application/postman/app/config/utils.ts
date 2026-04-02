import { compileExpression, useDotAccessOperator } from "filtrex";
import { isAddress, parseAbi } from "viem";
import { ZodError } from "zod";

import {
  ClaimingConfig,
  ClaimingOptions,
  ListenerConfig,
  ListenerOptions,
  PostmanConfig,
  PostmanOptions,
} from "./config";
import { postmanOptionsSchema } from "./schema";
import {
  DEFAULT_CALLDATA_ENABLED,
  DEFAULT_ENABLE_POSTMAN_SPONSORING,
  DEFAULT_EOA_ENABLED,
  DEFAULT_GAS_ESTIMATION_PERCENTILE,
  DEFAULT_INITIAL_FROM_BLOCK,
  DEFAULT_L2_MESSAGE_TREE_DEPTH,
  DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
  DEFAULT_LISTENER_INTERVAL,
  DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
  DEFAULT_MAX_CLAIM_GAS_LIMIT,
  DEFAULT_MAX_BUMPS_PER_CYCLE,
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
  DEFAULT_MAX_NONCE_DIFF,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_MAX_RETRY_CYCLES,
  DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../../../core/constants";

function resolveListenerConfig(opts: ListenerOptions): ListenerConfig {
  return {
    pollingInterval: opts.pollingInterval ?? DEFAULT_LISTENER_INTERVAL,
    receiptPollingInterval: opts.receiptPollingInterval ?? DEFAULT_LISTENER_INTERVAL,
    maxFetchMessagesFromDb: opts.maxFetchMessagesFromDb ?? DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
    maxBlocksToFetchLogs: opts.maxBlocksToFetchLogs ?? DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
    initialFromBlock: opts.initialFromBlock ?? DEFAULT_INITIAL_FROM_BLOCK,
    blockConfirmation: opts.blockConfirmation ?? DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
    ...(opts.eventFilters ? { eventFilters: opts.eventFilters } : {}),
  };
}

function resolveClaimingConfig(opts: ClaimingOptions): ClaimingConfig {
  return {
    signer: opts.signer,
    messageSubmissionTimeout: opts.messageSubmissionTimeout ?? DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
    feeRecipientAddress: opts.feeRecipientAddress,
    maxNonceDiff: opts.maxNonceDiff ?? DEFAULT_MAX_NONCE_DIFF,
    maxFeePerGasCap: opts.maxFeePerGasCap ?? DEFAULT_MAX_FEE_PER_GAS_CAP,
    gasEstimationPercentile: opts.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE,
    isMaxGasFeeEnforced: opts.isMaxGasFeeEnforced ?? false,
    profitMargin: opts.profitMargin ?? DEFAULT_PROFIT_MARGIN,
    maxNumberOfRetries: opts.maxNumberOfRetries ?? DEFAULT_MAX_NUMBER_OF_RETRIES,
    retryDelayInSeconds: opts.retryDelayInSeconds ?? DEFAULT_RETRY_DELAY_IN_SECONDS,
    maxClaimGasLimit: opts.maxClaimGasLimit ?? DEFAULT_MAX_CLAIM_GAS_LIMIT,
    maxBumpsPerCycle: opts.maxBumpsPerCycle ?? DEFAULT_MAX_BUMPS_PER_CYCLE,
    maxRetryCycles: opts.maxRetryCycles ?? DEFAULT_MAX_RETRY_CYCLES,
    isPostmanSponsorshipEnabled: opts.isPostmanSponsorshipEnabled ?? DEFAULT_ENABLE_POSTMAN_SPONSORING,
    maxPostmanSponsorGasLimit: opts.maxPostmanSponsorGasLimit ?? DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
    claimViaAddress: opts.claimViaAddress,
  };
}

/**
 * @notice Generates the configuration for the Postman service based on provided options.
 * @dev This function merges the provided options with default values where necessary.
 * @param postmanOptions The options provided to configure the Postman service.
 * @return postmanConfig The complete configuration for the Postman service.
 */
export function getConfig(postmanOptions: PostmanOptions): PostmanConfig {
  try {
    postmanOptionsSchema.parse(postmanOptions);
  } catch (error) {
    if (error instanceof ZodError) {
      const issues = error.issues.map((issue) => `  - ${issue.path.join(".")}: ${issue.message}`).join("\n");
      throw new Error(`Invalid postman configuration:\n${issues}`);
    }
    throw error;
  }

  const {
    l1Options,
    l2Options,
    l1L2AutoClaimEnabled,
    l2L1AutoClaimEnabled,
    databaseOptions,
    databaseCleanerOptions,
    loggerOptions,
    apiOptions,
  } = postmanOptions;

  if (l1Options.listener.eventFilters) {
    validateEventsFiltersConfig(l1Options.listener.eventFilters);
  }

  if (l2Options.listener.eventFilters) {
    validateEventsFiltersConfig(l2Options.listener.eventFilters);
  }

  return {
    l1Config: {
      rpcUrl: l1Options.rpcUrl,
      messageServiceContractAddress: l1Options.messageServiceContractAddress,
      isEOAEnabled: l1Options.isEOAEnabled ?? DEFAULT_EOA_ENABLED,
      isCalldataEnabled: l1Options.isCalldataEnabled ?? DEFAULT_CALLDATA_ENABLED,
      listener: resolveListenerConfig(l1Options.listener),
      claiming: resolveClaimingConfig(l1Options.claiming),
    },
    l2Config: {
      rpcUrl: l2Options.rpcUrl,
      messageServiceContractAddress: l2Options.messageServiceContractAddress,
      isEOAEnabled: l2Options.isEOAEnabled ?? DEFAULT_EOA_ENABLED,
      isCalldataEnabled: l2Options.isCalldataEnabled ?? DEFAULT_CALLDATA_ENABLED,
      l2MessageTreeDepth: l2Options.l2MessageTreeDepth ?? DEFAULT_L2_MESSAGE_TREE_DEPTH,
      enableLineaEstimateGas: l2Options.enableLineaEstimateGas ?? true,
      listener: resolveListenerConfig(l2Options.listener),
      claiming: resolveClaimingConfig(l2Options.claiming),
    },
    l1L2AutoClaimEnabled,
    l2L1AutoClaimEnabled,
    databaseOptions,
    databaseCleanerConfig: {
      enabled: databaseCleanerOptions?.enabled ?? false,
      cleaningInterval: databaseCleanerOptions?.cleaningInterval ?? 43200000,
      daysBeforeNowToDelete: databaseCleanerOptions?.daysBeforeNowToDelete ?? 14,
    },
    loggerOptions,
    apiConfig: {
      port: apiOptions?.port ?? 3000,
    },
  };
}

export function validateEventsFiltersConfig(eventFilters: ListenerConfig["eventFilters"]): void {
  if (eventFilters?.fromAddressFilter && !isAddress(eventFilters.fromAddressFilter)) {
    throw new Error(`Invalid fromAddressFilter: ${eventFilters.fromAddressFilter}`);
  }

  if (eventFilters?.toAddressFilter && !isAddress(eventFilters.toAddressFilter)) {
    throw new Error(`Invalid toAddressFilter: ${eventFilters.toAddressFilter}`);
  }

  if (
    eventFilters?.calldataFilter?.criteriaExpression &&
    !isValidFiltrexExpression(eventFilters?.calldataFilter?.criteriaExpression)
  ) {
    throw new Error(`Invalid calldataFilter expression: ${eventFilters.calldataFilter.criteriaExpression}`);
  }

  if (
    eventFilters?.calldataFilter?.calldataFunctionInterface &&
    !isFunctionInterfaceValid(eventFilters?.calldataFilter?.calldataFunctionInterface)
  ) {
    throw new Error(`Invalid calldataFunctionInterface: ${eventFilters?.calldataFilter?.calldataFunctionInterface}`);
  }
}

export function isFunctionInterfaceValid(functionInterface: string): boolean {
  try {
    const abi = parseAbi([functionInterface] as readonly string[]);
    return (abi as unknown[]).length !== 0;
  } catch {
    return false;
  }
}

export function isValidFiltrexExpression(expression: string): boolean {
  try {
    compileExpression(expression, { customProp: useDotAccessOperator });
    return true;
  } catch {
    return false;
  }
}
