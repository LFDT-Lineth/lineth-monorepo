export class BaseError extends Error {
  override name = "PostmanCoreError";

  constructor(message?: string, options?: { cause?: unknown }) {
    super(message || "An error occurred.", options);
    Error.captureStackTrace(this, this.constructor);
  }
}
