import { isNativeError } from "util/types";
import { BaseError, InferErrorType } from "./BaseError";

/**
 * Converts an `unknown` value that was thrown into a `BaseError` object.
 *
 * @param value - An `unknown` value.
 *
 * @returns A `BaseError` object.
 */
export const makeBaseError = <E, M>(value: E extends BaseError<E, M> ? never : E, metadata?: M): BaseError<E, M> => {
  if (isNativeError(value)) {
    return new BaseError(value, metadata);
  } else {
    try {
      return new BaseError(
        new Error(`${typeof value === "object" ? JSON.stringify(value) : String(value)}`),
        metadata,
      ) as BaseError<E, M>;
    } catch {
      return new BaseError(new Error(`Unexpected value thrown: non-stringifiable object`), metadata) as BaseError<E, M>;
    }
  }
};

/**
 * Type guard to check if an `unknown` value is a `BaseError` object.
 *
 * @param value - The value to check.
 *
 * @returns `true` if the value is a `BaseError` object, otherwise `false`.
 */
export const isBaseError = <E, M>(value: unknown): value is BaseError<InferErrorType<E>, M> => {
  return value instanceof BaseError && typeof value.stack === "string" && "metadata" in value && "error" in value;
};
