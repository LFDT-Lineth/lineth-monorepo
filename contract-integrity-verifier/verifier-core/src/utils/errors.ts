/**
 * Contract Integrity Verifier - Error Handling Utilities
 *
 * Centralized error formatting and handling for consistent
 * error messages across the codebase.
 */

/**
 * Formats an error into a consistent string message.
 * Handles Error instances, strings, and other types.
 *
 * @param error - The error to format
 * @returns Formatted error message string
 */
export function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return String(error);
}

/**
 * Creates a standardized error result object.
 * Used for returning error information in result objects.
 *
 * @param message - Error message
 * @param details - Optional additional details
 */
export interface ErrorResult {
  status: "fail";
  message: string;
  error?: string;
}

/**
 * Creates an error result with consistent formatting.
 *
 * @param prefix - Prefix for the error message (e.g., "Call failed")
 * @param error - The original error
 * @returns ErrorResult object
 */
export function createErrorResult(prefix: string, error: unknown): ErrorResult {
  const message = formatError(error);
  return {
    status: "fail",
    message: `${prefix}: ${message}`,
    error: message,
  };
}

/**
 * Wraps an async function with consistent error handling.
 * Returns the result or an error result on failure.
 *
 * @param fn - Async function to wrap
 * @param errorPrefix - Prefix for error messages
 * @returns Result of fn or error result
 */
export async function withErrorHandling<T extends { status: string; message: string }>(
  fn: () => Promise<T>,
  errorPrefix: string,
): Promise<T | ErrorResult> {
  try {
    return await fn();
  } catch (error) {
    return createErrorResult(errorPrefix, error);
  }
}
