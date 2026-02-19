/**
 * Executes a fetch request with a configurable timeout.
 *
 * Uses AbortController to cancel the request if it exceeds the specified timeout period.
 * This prevents indefinite hangs from slow or unresponsive servers.
 *
 * @param url - The URL to fetch
 * @param options - Standard fetch options (headers, method, body, etc.)
 * @param timeoutMs - Timeout duration in milliseconds
 * @returns Promise resolving to the fetch Response
 * @throws {Error} If the request times out or fetch fails
 *
 * @example
 * ```typescript
 * const response = await fetchWithTimeout(
 *   'https://api.example.com/data',
 *   { method: 'POST', body: JSON.stringify(data) },
 *   15000
 * );
 * ```
 */
export async function fetchWithTimeout(
  url: string,
  options: RequestInit = {},
  timeoutMs: number
): Promise<Response> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
    });
    return response;
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") {
      throw new Error(`Request timeout after ${timeoutMs}ms`);
    }
    throw error;
  } finally {
    clearTimeout(timeoutId);
  }
}
