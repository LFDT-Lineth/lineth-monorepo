export type InferErrorType<T> = T extends Error
  ? Error
  : T extends string
    ? string
    : T extends number
      ? number
      : T extends { message: string }
        ? { message: string }
        : unknown;

export class BaseError<T = unknown, M = unknown> extends Error {
  stack: string = "";
  metadata?: M;
  error: InferErrorType<T>;

  constructor(error: T, metadata?: M) {
    const message = BaseError.getMessage(error);
    super(message);
    this.stack = error instanceof Error && error.stack ? error.stack : message;
    this.metadata = metadata;
    this.error = error as InferErrorType<T>;

    Object.setPrototypeOf(this, BaseError.prototype);
  }

  private static getMessage(error: unknown): string {
    if (typeof error === "string") return error;
    if (typeof error === "number") return `Error Code: ${error}`;
    if (error instanceof Error) return error.message;
    if (typeof error === "object" && error !== null && "message" in error) {
      return String(error.message);
    }
    return "Unknown error";
  }
}
