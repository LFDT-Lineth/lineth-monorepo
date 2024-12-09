import { EthersError, ErrorCode } from "ethers";
import { GasEstimationError } from "@consensys/linea-sdk";
import { DatabaseAccessError } from "../core/errors/DatabaseErrors";
import { MessageProps } from "../core/entities/Message";

export type Mitigation = {
  shouldRetry: boolean;
  retryWithBlocking?: boolean;
  retryPeriodInMs?: number;
  retryNumOfTime?: number;
};

export type ParsedErrorResult = {
  errorCode: ErrorCode;
  context?: string;
  reason?: string;
  mitigation: Mitigation;
};

export class ErrorParser {
  /**
   * Parses an `EthersError` to identify its type and suggests mitigation strategies.
   *
   * This method categorizes various Ethereum-related errors into more understandable contexts and suggests actionable mitigation strategies, such as whether to retry the operation.
   *
   * @param {EthersError} error - The error encountered during Ethereum operations.
   * @returns {ParsedErrorResult | null} An object containing the parsed error result and mitigation strategies, or `null` if the error is `undefined`.
   */
  public static parseErrorWithMitigation(error: EthersError): ParsedErrorResult | null {
    if (!error) {
      return null;
    }

    const parsedErrResult: ParsedErrorResult = {
      errorCode: "UNKNOWN_ERROR",
      mitigation: { shouldRetry: false },
    };

    switch (error.code) {
      case "NETWORK_ERROR":
      case "SERVER_ERROR":
      case "TIMEOUT":
      case "INSUFFICIENT_FUNDS":
      case "REPLACEMENT_UNDERPRICED":
      case "NONCE_EXPIRED":
        parsedErrResult.context = error.shortMessage;
        parsedErrResult.mitigation = {
          shouldRetry: true,
        };
        break;
      case "CALL_EXCEPTION":
        if (
          error.shortMessage.includes("execution reverted") ||
          error.info?.error?.code === 4001 || //The user rejected the request (EIP-1193)
          error.info?.error?.code === -32603 //Internal JSON-RPC error (EIP-1474)
        ) {
          parsedErrResult.context = error.info?.error?.message ?? error.shortMessage;
          break;
        }

        if (
          error.info?.error?.code === -32000 && //Missing or invalid parameters (EIP-1474)
          (error.info?.error?.message.includes("gas required exceeds allowance (0)") ||
            error.info?.error?.message.includes("max priority fee per gas higher than max fee per gas") ||
            error.info?.error?.message.includes("max fee per gas less than block base fee"))
        ) {
          parsedErrResult.context = error.info?.error?.message;
          parsedErrResult.mitigation = {
            shouldRetry: true,
          };
          break;
        }

        parsedErrResult.context = error.shortMessage;
        parsedErrResult.mitigation = {
          shouldRetry: true,
        };
        break;
      case "ACTION_REJECTED":
      case "UNKNOWN_ERROR":
        parsedErrResult.context = error.shortMessage;
        break;
      default:
        if (error instanceof GasEstimationError) {
          parsedErrResult.context = (error as GasEstimationError<MessageProps>).message;
          break;
        }

        if (error instanceof DatabaseAccessError) {
          parsedErrResult.context = (error as DatabaseAccessError<MessageProps>).message;
        } else {
          parsedErrResult.context = error.message;
        }

        parsedErrResult.context = error.shortMessage ?? error.message;
        parsedErrResult.mitigation = {
          shouldRetry: true,
        };
        break;
    }

    return {
      ...parsedErrResult,
      errorCode: error.code || parsedErrResult.errorCode,
    };
  }
}
