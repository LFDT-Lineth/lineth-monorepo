// Linea SDK entry point
export { LineaSDK } from "./LineaSDK";

// Core types
export {
  LineaSDKOptions,
  Network,
  NetworkInfo,
  L1FeeEstimatorOptions,
  L2FeeEstimatorOptions,
  SDKMode,
  Message,
  MessageSent,
  MessageClaimed,
  L2MessagingBlockAnchored,
  ServiceVersionMigrated,
} from "./core/types";

// Core enums and constants
export { OnChainMessageStatus, Direction } from "./core/enums";
export * from "./core/constants";

// Providers to interact with Ethereum and Linea
export { Provider, BrowserProvider, LineaProvider, LineaBrowserProvider } from "./clients/providers";
export { DefaultGasProvider, GasProvider, LineaGasProvider } from "./clients/gas";

// Core errors
export { GasEstimationError, FeeEstimationError } from "./core/errors";

// Contracts types and factories (generated from typechain)
export { LineaRollup, LineaRollup__factory, L2MessageService, L2MessageService__factory } from "./contracts/typechain";

// Utils functions
export { formatMessageStatus, serialize, isEmptyBytes, isString, isNull, isUndefined, wait } from "./core/utils";

// Testing helpers
import {
  generateLineaRollupClient,
  generateL2MessageServiceClient,
  generateTransactionReceipt,
  generateTransactionResponse,
} from "./utils/testing/helpers";

const testingHelpers = {
  generateLineaRollupClient,
  generateL2MessageServiceClient,
  generateTransactionReceipt,
  generateTransactionResponse,
};

export { testingHelpers };
