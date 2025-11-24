/* eslint-disable @typescript-eslint/no-explicit-any */
import { EthersError } from "ethers";
import { WinstonLogger } from "@consensys/linea-shared-utils";
import { IPostmanLogger } from "./IPostmanLogger";

export class PostmanWinstonLogger extends WinstonLogger implements IPostmanLogger {
  /**
   * Decides whether to log a message as a `warning` or an `error` based on its content and severity.
   *
   * This method is particularly useful for handling errors that may not always require immediate attention or could be retried successfully.
   *
   * @param {any} message - The primary log message or error object.
   * @param {...any[]} params - Additional parameters or metadata to log alongside the message.
   */
  public warnOrError(message: any, ...params: any[]): void {
    if (this.shouldLogErrorAsWarning(message)) {
      this.warn(message, ...params);
    } else {
      this.error(message, ...params);
    }
  }

  /**
   * Determines whether a given error should be logged as a `warning` instead of an `error`.
   *
   * This captures the original Postman-specific heuristics for common RPC responses.
   */
  protected shouldLogErrorAsWarning(error: unknown): boolean {
    const isEthersError = (value: unknown): value is EthersError => {
      return (value as EthersError).shortMessage !== undefined || (value as EthersError).code !== undefined;
    };

    if (!isEthersError(error)) {
      return false;
    }

    return (
      (error.shortMessage?.includes("processing response error") ||
        error.info?.error?.message?.includes("processing response error")) &&
      error.code === "SERVER_ERROR" &&
      error.info?.error?.code === -32603
    );
  }
}
