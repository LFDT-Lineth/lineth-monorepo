import { EthersError, ErrorCode, isError } from "ethers";
import { isBaseError } from "@consensys/linea-sdk";
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
  errorMessage?: string;
  data?: string;
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
  public static parseErrorWithMitigation(error: unknown): ParsedErrorResult | null {
    if (!error) {
      return null;
    }

    if (!isBaseError(error)) {
      if (error instanceof DatabaseAccessError) {
        return {
          errorCode: "UNKNOWN_ERROR",
          errorMessage: (error as DatabaseAccessError<MessageProps>).message,
          mitigation: { shouldRetry: true },
        };
      }

      return {
        errorCode: "UNKNOWN_ERROR",
        errorMessage: error instanceof Error ? error.message : String(error),
        mitigation: { shouldRetry: false },
      };
    }

    if (!this.isEthersError(error.error)) {
      return {
        errorCode: "UNKNOWN_ERROR",
        errorMessage: error instanceof Error ? error.message : String(error),
        mitigation: { shouldRetry: false },
      };
    }

    return this.parseEthersError(error.error);
  }

  private static isEthersError(error: unknown): error is EthersError {
    return (error as EthersError).shortMessage !== undefined || (error as EthersError).code !== undefined;
  }

  public static parseEthersError(error: EthersError): ParsedErrorResult {
    if (
      isError(error, "NETWORK_ERROR") ||
      isError(error, "SERVER_ERROR") ||
      isError(error, "TIMEOUT") ||
      isError(error, "INSUFFICIENT_FUNDS") ||
      isError(error, "REPLACEMENT_UNDERPRICED") ||
      isError(error, "NONCE_EXPIRED")
    ) {
      return {
        errorCode: error.code,
        errorMessage: error.message,
        mitigation: {
          shouldRetry: true,
        },
      };
    }

    if (isError(error, "CALL_EXCEPTION")) {
      if (
        error.shortMessage.includes("execution reverted") ||
        error.info?.error?.code === 4001 || //The user rejected the request (EIP-1193)
        error.info?.error?.code === -32603 //Internal JSON-RPC error (EIP-1474)
      ) {
        return {
          errorCode: error.code,
          errorMessage: error.info?.error?.message ?? error.shortMessage,
          mitigation: {
            shouldRetry: false,
          },
        };
      }

      if (
        error.info?.error?.code === -32000 && //Missing or invalid parameters (EIP-1474)
        (error.info?.error?.message.includes("gas required exceeds allowance (0)") ||
          error.info?.error?.message.includes("max priority fee per gas higher than max fee per gas") ||
          error.info?.error?.message.includes("max fee per gas less than block base fee"))
      ) {
        return {
          errorCode: error.code,
          errorMessage: error.info?.error?.message ?? error.shortMessage,
          mitigation: {
            shouldRetry: true,
          },
        };
      }

      return {
        errorCode: error.code,
        errorMessage: error.shortMessage,
        mitigation: {
          shouldRetry: true,
        },
      };
    }

    if (isError(error, "ACTION_REJECTED")) {
      return {
        errorCode: error.code,
        errorMessage: error.info?.error?.message ?? error.shortMessage,
        mitigation: {
          shouldRetry: false,
        },
      };
    }

    if (isError(error, "UNKNOWN_ERROR")) {
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore
      if (error.error?.code === -32000 && error.error?.message?.toLowerCase().includes("execution reverted")) {
        return {
          errorCode: error.code,
          errorMessage: error.error?.message ?? error.shortMessage,
          // eslint-disable-next-line @typescript-eslint/ban-ts-comment
          // @ts-ignore
          data: error.error?.data,
          mitigation: {
            shouldRetry: false,
          },
        };
      }

      return {
        errorCode: error.code,
        errorMessage: error.shortMessage,
        mitigation: {
          shouldRetry: false,
        },
      };
    }

    return {
      errorCode: error.code,
      errorMessage: error.shortMessage ?? error.message,
      mitigation: {
        shouldRetry: true,
      },
    };
  }
}
