import { BaseError } from "./BaseError";

export class UnsupportedOperationError extends BaseError {
  override name = "UnsupportedOperationError";

  constructor(operation: string, context?: string) {
    super(`${operation} is not supported${context ? ` (${context})` : ""}`);
  }
}
