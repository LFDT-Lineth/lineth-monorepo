import { Interface, isAddress } from "ethers";
import { compileExpression, useDotAccessOperator } from "filtrex";

import { ListenerConfig, PostmanConfig, PostmanOptions } from "./config";
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
  DEFAULT_MAX_FEE_PER_GAS_CAP,
  DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
  DEFAULT_MAX_NONCE_DIFF,
  DEFAULT_MAX_NUMBER_OF_RETRIES,
  DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
  DEFAULT_MAX_TX_RETRIES,
  DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
  DEFAULT_PROFIT_MARGIN,
  DEFAULT_RETRY_DELAY_IN_SECONDS,
} from "../../../../core/constants";

/**
 * @notice Generates the configuration for the Postman service based on provided options.
 * @dev This function merges the provided options with default values where necessary.
 * @param postmanOptions The options provided to configure the Postman service.
 * @return postmanConfig The complete configuration for the Postman service.
 */
export function getConfig(postmanOptions: PostmanOptions): PostmanConfig {
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
      listener: {
        pollingInterval: l1Options.listener.pollingInterval ?? DEFAULT_LISTENER_INTERVAL,
        receiptPollingInterval: l1Options.listener.receiptPollingInterval ?? DEFAULT_LISTENER_INTERVAL,
        maxFetchMessagesFromDb: l1Options.listener.maxFetchMessagesFromDb ?? DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
        maxBlocksToFetchLogs: l1Options.listener.maxBlocksToFetchLogs ?? DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
        initialFromBlock: l1Options.listener.initialFromBlock ?? DEFAULT_INITIAL_FROM_BLOCK,
        blockConfirmation: l1Options.listener.blockConfirmation ?? DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
        ...(l1Options.listener.eventFilters ? { eventFilters: l1Options.listener.eventFilters } : {}),
      },
      claiming: {
        signerPrivateKey: l1Options.claiming.signerPrivateKey,
        messageSubmissionTimeout: l1Options.claiming.messageSubmissionTimeout ?? DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
        feeRecipientAddress: l1Options.claiming.feeRecipientAddress,
        maxNonceDiff: l1Options.claiming.maxNonceDiff ?? DEFAULT_MAX_NONCE_DIFF,
        maxFeePerGasCap: l1Options.claiming.maxFeePerGasCap ?? DEFAULT_MAX_FEE_PER_GAS_CAP,
        gasEstimationPercentile: l1Options.claiming.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE,
        isMaxGasFeeEnforced: l1Options.claiming.isMaxGasFeeEnforced ?? false,
        profitMargin: l1Options.claiming.profitMargin ?? DEFAULT_PROFIT_MARGIN,
        maxNumberOfRetries: l1Options.claiming.maxNumberOfRetries ?? DEFAULT_MAX_NUMBER_OF_RETRIES,
        retryDelayInSeconds: l1Options.claiming.retryDelayInSeconds ?? DEFAULT_RETRY_DELAY_IN_SECONDS,
        maxClaimGasLimit: l1Options.claiming.maxClaimGasLimit ?? DEFAULT_MAX_CLAIM_GAS_LIMIT,
        maxTxRetries: l1Options.claiming.maxTxRetries ?? DEFAULT_MAX_TX_RETRIES,
        isPostmanSponsorshipEnabled:
          l1Options.claiming.isPostmanSponsorshipEnabled ?? DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit:
          l1Options.claiming.maxPostmanSponsorGasLimit ?? DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
        claimViaAddress: l1Options.claiming.claimViaAddress,
      },
    },
    l2Config: {
      rpcUrl: l2Options.rpcUrl,
      messageServiceContractAddress: l2Options.messageServiceContractAddress,
      isEOAEnabled: l2Options.isEOAEnabled ?? DEFAULT_EOA_ENABLED,
      isCalldataEnabled: l2Options.isCalldataEnabled ?? DEFAULT_CALLDATA_ENABLED,
      l2MessageTreeDepth: l2Options.l2MessageTreeDepth ?? DEFAULT_L2_MESSAGE_TREE_DEPTH,
      enableLineaEstimateGas: l2Options.enableLineaEstimateGas ?? false,
      listener: {
        pollingInterval: l2Options.listener.pollingInterval ?? DEFAULT_LISTENER_INTERVAL,
        receiptPollingInterval: l2Options.listener.receiptPollingInterval ?? DEFAULT_LISTENER_INTERVAL,
        maxFetchMessagesFromDb: l2Options.listener.maxFetchMessagesFromDb ?? DEFAULT_MAX_FETCH_MESSAGES_FROM_DB,
        maxBlocksToFetchLogs: l2Options.listener.maxBlocksToFetchLogs ?? DEFAULT_MAX_BLOCKS_TO_FETCH_LOGS,
        initialFromBlock: l2Options.listener.initialFromBlock ?? DEFAULT_INITIAL_FROM_BLOCK,
        blockConfirmation: l2Options.listener.blockConfirmation ?? DEFAULT_LISTENER_BLOCK_CONFIRMATIONS,
        ...(l2Options.listener.eventFilters ? { eventFilters: l2Options.listener.eventFilters } : {}),
      },
      claiming: {
        signerPrivateKey: l2Options.claiming.signerPrivateKey,
        messageSubmissionTimeout: l2Options.claiming.messageSubmissionTimeout ?? DEFAULT_MESSAGE_SUBMISSION_TIMEOUT,
        feeRecipientAddress: l2Options.claiming.feeRecipientAddress,
        maxNonceDiff: l2Options.claiming.maxNonceDiff ?? DEFAULT_MAX_NONCE_DIFF,
        maxFeePerGasCap: l2Options.claiming.maxFeePerGasCap ?? DEFAULT_MAX_FEE_PER_GAS_CAP,
        gasEstimationPercentile: l2Options.claiming.gasEstimationPercentile ?? DEFAULT_GAS_ESTIMATION_PERCENTILE,
        isMaxGasFeeEnforced: l2Options.claiming.isMaxGasFeeEnforced ?? false,
        profitMargin: l2Options.claiming.profitMargin ?? DEFAULT_PROFIT_MARGIN,
        maxNumberOfRetries: l2Options.claiming.maxNumberOfRetries ?? DEFAULT_MAX_NUMBER_OF_RETRIES,
        retryDelayInSeconds: l2Options.claiming.retryDelayInSeconds ?? DEFAULT_RETRY_DELAY_IN_SECONDS,
        maxClaimGasLimit: l2Options.claiming.maxClaimGasLimit ?? DEFAULT_MAX_CLAIM_GAS_LIMIT,
        maxTxRetries: l2Options.claiming.maxTxRetries ?? DEFAULT_MAX_TX_RETRIES,
        isPostmanSponsorshipEnabled:
          l2Options.claiming.isPostmanSponsorshipEnabled ?? DEFAULT_ENABLE_POSTMAN_SPONSORING,
        maxPostmanSponsorGasLimit:
          l2Options.claiming.maxPostmanSponsorGasLimit ?? DEFAULT_MAX_POSTMAN_SPONSOR_GAS_LIMIT,
        claimViaAddress: l2Options.claiming.claimViaAddress,
      },
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
    const i = new Interface([functionInterface]);

    return i.fragments.length !== 0;
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
