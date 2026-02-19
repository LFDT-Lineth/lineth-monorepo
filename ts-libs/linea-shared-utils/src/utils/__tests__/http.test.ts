import { fetchWithTimeout } from "../http";

describe("fetchWithTimeout", () => {
  let fetchMock: jest.SpyInstance;
  let originalFetch: typeof global.fetch;

  beforeEach(() => {
    originalFetch = global.fetch;
    fetchMock = jest.fn();
    global.fetch = fetchMock as unknown as typeof fetch;
  });

  afterEach(() => {
    global.fetch = originalFetch;
    jest.restoreAllMocks();
  });

  it("returns response when fetch completes within timeout", async () => {
    // Arrange
    const mockResponse = { ok: true, status: 200 } as Response;
    fetchMock.mockResolvedValue(mockResponse);

    // Act
    const result = await fetchWithTimeout("https://example.com/api", {}, 5000);

    // Assert
    expect(result).toBe(mockResponse);
    expect(fetchMock).toHaveBeenCalledWith("https://example.com/api", {
      signal: expect.any(AbortSignal),
    });
  });

  it("passes request options to fetch", async () => {
    // Arrange
    const mockResponse = { ok: true } as Response;
    fetchMock.mockResolvedValue(mockResponse);
    const options: RequestInit = {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ test: "data" }),
    };

    // Act
    await fetchWithTimeout("https://example.com/api", options, 5000);

    // Assert
    expect(fetchMock).toHaveBeenCalledWith("https://example.com/api", {
      ...options,
      signal: expect.any(AbortSignal),
    });
  });

  it("throws timeout error when request exceeds timeout", async () => {
    // Arrange
    const abortError = new Error("Aborted");
    abortError.name = "AbortError";
    fetchMock.mockRejectedValue(abortError);

    // Act & Assert
    await expect(fetchWithTimeout("https://example.com/api", {}, 1000)).rejects.toThrow(
      "Request timeout after 1000ms"
    );
  });

  it("propagates non-timeout errors from fetch", async () => {
    // Arrange
    const networkError = new Error("Network failure");
    networkError.name = "NetworkError";
    fetchMock.mockRejectedValue(networkError);

    // Act & Assert
    await expect(fetchWithTimeout("https://example.com/api", {}, 5000)).rejects.toThrow("Network failure");
  });

  it("clears timeout after successful fetch", async () => {
    // Arrange
    const mockResponse = { ok: true } as Response;
    fetchMock.mockResolvedValue(mockResponse);
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");

    // Act
    await fetchWithTimeout("https://example.com/api", {}, 5000);

    // Assert
    expect(clearTimeoutSpy).toHaveBeenCalled();
  });

  it("clears timeout after failed fetch", async () => {
    // Arrange
    const networkError = new Error("Network failure");
    fetchMock.mockRejectedValue(networkError);
    const clearTimeoutSpy = jest.spyOn(global, "clearTimeout");

    // Act
    try {
      await fetchWithTimeout("https://example.com/api", {}, 5000);
    } catch {
      // Expected
    }

    // Assert
    expect(clearTimeoutSpy).toHaveBeenCalled();
  });

  it("works with default empty options", async () => {
    // Arrange
    const mockResponse = { ok: true } as Response;
    fetchMock.mockResolvedValue(mockResponse);

    // Act
    const result = await fetchWithTimeout("https://example.com/api", undefined, 5000);

    // Assert
    expect(result).toBe(mockResponse);
    expect(fetchMock).toHaveBeenCalledWith("https://example.com/api", {
      signal: expect.any(AbortSignal),
    });
  });
});
