/* eslint-disable @typescript-eslint/no-explicit-any */
import { ILogger } from "@consensys/linea-shared-utils";

/**
 * Extended logger interface for the postman project.
 * Extends the base ILogger interface with the warnOrError method.
 */
export interface IPostmanLogger extends ILogger {
  /**
   * Decides whether to log a message as a `warning` or an `error` based on its content and severity.
   *
   * This method is particularly useful for handling errors that may not always require immediate attention or could be retried successfully.
   *
   * @param {any} message - The primary log message or error object.
   * @param {...any[]} params - Additional parameters or metadata to log alongside the message.
   */
  warnOrError(message: any, ...params: any[]): void;
}
