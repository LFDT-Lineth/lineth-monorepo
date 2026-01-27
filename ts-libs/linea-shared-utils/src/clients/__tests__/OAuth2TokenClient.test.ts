import axios from "axios";

import { IRetryService } from "../../core/services/IRetryService";
import { ILogger } from "../../logging/ILogger";
import { getCurrentUnixTimestampSeconds } from "../../utils/time";
import { OAuth2TokenClient } from "../OAuth2TokenClient";

jest.mock("axios");
jest.mock("../../utils/time", () => ({
  getCurrentUnixTimestampSeconds: jest.fn(),
}));

const mockedAxios = axios as jest.Mocked<typeof axios>;
const getCurrentUnixTimestampSecondsMock = getCurrentUnixTimestampSeconds as jest.MockedFunction<
  typeof getCurrentUnixTimestampSeconds
>;

describe("OAuth2TokenClient", () => {
  const tokenUrl = "https://auth.local/token";
  const clientId = "client-id";
  const clientSecret = "client-secret";
  const audience = "api-audience";
  const grantType = "client_credentials";

  let logger: jest.Mocked<ILogger>;
  let retryService: jest.Mocked<IRetryService>;
  let client: OAuth2TokenClient;

  beforeEach(() => {
    logger = {
      name: "test",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    };
    retryService = {
      retry: jest.fn(async (fn) => fn()),
    } as unknown as jest.Mocked<IRetryService>;
    mockedAxios.post.mockReset();
    getCurrentUnixTimestampSecondsMock.mockReset();

    client = new OAuth2TokenClient(logger, retryService, tokenUrl, clientId, clientSecret, audience, grantType, 60);
  });

  it("returns the cached token when it is still valid", async () => {
    (client as any).bearerToken = "Bearer cached-token";
    (client as any).tokenExpiresAtSeconds = 500;
    getCurrentUnixTimestampSecondsMock.mockReturnValue(430);

    const token = await client.getBearerToken();

    expect(token).toBe("Bearer cached-token");
    expect(logger.info).toHaveBeenCalledWith("getBearerToken cache-hit");
    expect(retryService.retry).not.toHaveBeenCalled();
    expect(mockedAxios.post).not.toHaveBeenCalled();
  });

  it("requests a new token and stores expiration based on expires_in", async () => {
    getCurrentUnixTimestampSecondsMock.mockReturnValueOnce(1_000);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: "new-token",
        token_type: "Custom",
        expires_in: 120,
      },
    });

    const token = await client.getBearerToken();

    expect(token).toBe("Custom new-token");

    expect(retryService.retry).toHaveBeenCalledTimes(1);
    const expectedBody = {
      client_id: clientId,
      client_secret: clientSecret,
      audience,
      grant_type: grantType,
    };
    expect(mockedAxios.post).toHaveBeenCalledWith(
      tokenUrl,
      expectedBody,
      expect.objectContaining({
        headers: { "content-type": "application/json" },
      }),
    );

    expect((client as any).bearerToken).toBe("Custom new-token");
    expect((client as any).tokenExpiresAtSeconds).toBe(1_120);
    expect(logger.info).toHaveBeenNthCalledWith(1, "getBearerToken requesting new token");
    expect(logger.info).toHaveBeenNthCalledWith(
      2,
      "getBearerToken successfully retrived new OAuth2 Bearer token tokenExpiresAtSeconds=1120",
    );
    expect(logger.error).not.toHaveBeenCalled();
  });

  it("requests a new token and respects expires_at when provided", async () => {
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: "expires-at-token",
        token_type: undefined,
        expires_at: 9_999,
      },
    });

    const token = await client.getBearerToken();

    expect(token).toBe("Bearer expires-at-token");
    expect((client as any).tokenExpiresAtSeconds).toBe(9_999);
    expect(logger.info).toHaveBeenNthCalledWith(1, "getBearerToken requesting new token");
    expect(logger.info).toHaveBeenNthCalledWith(
      2,
      "getBearerToken successfully retrived new OAuth2 Bearer token tokenExpiresAtSeconds=9999",
    );
  });

  it("logs an error and returns undefined when access_token is missing", async () => {
    mockedAxios.post.mockResolvedValue({
      data: {},
    });

    const token = await client.getBearerToken();

    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("Failed to retrieve OAuth2 access token");
    expect(logger.info).toHaveBeenCalledWith("getBearerToken requesting new token");
    expect(logger.info).not.toHaveBeenCalledWith(expect.stringContaining("successfully retrived"));
  });

  it("returns undefined and logs when expires_at is already elapsed", async () => {
    getCurrentUnixTimestampSecondsMock.mockReturnValue(2_000);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: "expired-token",
        expires_at: 1_000,
      },
    });

    const token = await client.getBearerToken();

    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("OAuth2 access token already expired at 1000");
    expect(logger.info).toHaveBeenCalledWith("getBearerToken requesting new token");
    expect(logger.info).not.toHaveBeenCalledWith(expect.stringContaining("successfully retrived"));
  });

  it("returns undefined and logs when expiry metadata is missing", async () => {
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: "no-expiry-token",
      },
    });

    const token = await client.getBearerToken();

    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("OAuth2 access token did not provide expiry data");
    expect(logger.info).toHaveBeenCalledWith("getBearerToken requesting new token");
    expect(logger.info).not.toHaveBeenCalledWith(expect.stringContaining("successfully retrived"));
  });

  it("uses default grant type and expiry buffer when omitted", async () => {
    client = new OAuth2TokenClient(logger, retryService, tokenUrl, clientId, clientSecret, audience);

    expect((client as any).grantType).toBe("client_credentials");
    expect((client as any).expiryBufferSeconds).toBe(60);

    mockedAxios.post.mockResolvedValueOnce({
      data: {
        access_token: "default-token",
        expires_in: 120,
      },
    });

    getCurrentUnixTimestampSecondsMock.mockReturnValueOnce(1_000).mockReturnValueOnce(1_000);

    const token = await client.getBearerToken();
    expect(token).toBe("Bearer default-token");
    expect(mockedAxios.post).toHaveBeenCalledTimes(1);
    expect(mockedAxios.post).toHaveBeenCalledWith(
      tokenUrl,
      {
        client_id: clientId,
        client_secret: clientSecret,
        audience,
        grant_type: grantType,
      },
      expect.objectContaining({
        headers: { "content-type": "application/json" },
      }),
    );
  });
});
