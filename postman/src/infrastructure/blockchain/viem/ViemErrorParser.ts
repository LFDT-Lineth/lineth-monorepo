import {
  BaseError as ViemBaseError,
  ContractFunctionRevertedError,
  RpcRequestError,
  UserRejectedRequestError,
  TransactionRejectedRpcError,
  ExecutionRevertedError,
  toFunctionSelector,
  slice,
  type Hex,
} from "viem";
import { formatAbiItem } from "viem/utils";

import { DatabaseAccessError } from "../../../core/errors/DatabaseErrors";
import { IErrorParser, ParsedError } from "../../../core/errors/IErrorParser";

export class ViemErrorParser implements IErrorParser {
  parse(error: unknown): ParsedError {
    if (error instanceof DatabaseAccessError) {
      return { retryable: true, message: error.message };
    }

    if (!(error instanceof ViemBaseError)) {
      if (error instanceof Error) {
        return { retryable: true, message: error.message };
      }
      return { retryable: true, message: String(error ?? "") };
    }

    const message = error.shortMessage || error.message;

    const rpcError = error.walk((e) => e instanceof RpcRequestError) as RpcRequestError | null;
    if (rpcError !== null) {
      const details = rpcError.details?.toLowerCase();
      if (ExecutionRevertedError.nodeMessage.test(details)) {
        if (rpcError.data && typeof rpcError.data === "string") {
          const signature = slice(rpcError.data as Hex, 0, 4);
          if (
            signature ===
            toFunctionSelector(
              formatAbiItem({
                inputs: [],
                name: "RateLimitExceeded",
                type: "error",
              }),
            )
          ) {
            return { retryable: true, message };
          }
        }
        return { retryable: false, message };
      }

      return { retryable: true, message };
    }

    const contractRevertError = error.walk(
      (e) => e instanceof ContractFunctionRevertedError,
    ) as ContractFunctionRevertedError | null;
    if (contractRevertError) {
      if (contractRevertError.raw !== undefined) {
        const signature = slice(contractRevertError.raw, 0, 4);
        if (
          signature ===
          toFunctionSelector(
            formatAbiItem({
              inputs: [],
              name: "RateLimitExceeded",
              type: "error",
            }),
          )
        ) {
          return { retryable: true, message };
        }
      }
      return { retryable: false, message };
    }

    if (error.walk((e) => e instanceof ExecutionRevertedError)) {
      if (error.message.includes("RateLimitExceeded")) {
        return { retryable: true, message };
      }
      return { retryable: false, message };
    }

    if (error.walk((e) => e instanceof UserRejectedRequestError)) {
      return { retryable: false, message };
    }

    if (error.walk((e) => e instanceof TransactionRejectedRpcError)) {
      return { retryable: false, message };
    }

    return { retryable: true, message };
  }
}
