export class BaseError extends Error {
  reason?: BaseError | Error | string;

  override name = "PostmanCoreError";

  constructor(message?: string) {
    super();
    this.message = message || "An error occurred.";
    Error.captureStackTrace(this, this.constructor);
  }
}
