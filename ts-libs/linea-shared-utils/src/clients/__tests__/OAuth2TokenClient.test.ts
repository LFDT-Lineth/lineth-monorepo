import axios from "axios";

import { IRetryService } from "../../core/services/IRetryService";
import { ILogger } from "../../logging/ILogger";
import { getCurrentUnixTimestampSeconds } from "../../utils/time";
import { OAuth2TokenClient } from "../OAuth2TokenClient";
import { createLoggerMock, createRetryServiceMock } from "../../__tests__/helpers/factories";

jest.mock("axios");
jest.mock("../../utils/time", () => ({
  getCurrentUnixTimestampSeconds: jest.fn(),
}));

const mockedAxios = axios as jest.Mocked<typeof axios>;
const getCurrentUnixTimestampSecondsMock = getCurrentUnixTimestampSeconds as jest.MockedFunction<
  typeof getCurrentUnixTimestampSeconds
>;

describe("OAuth2TokenClient", () => {
  // Test data constants
  const TOKEN_URL = "https://auth.local/token";
  const CLIENT_ID = "client-id";
  const CLIENT_SECRET = "client-secret";
  const AUDIENCE = "api-audience";
  const GRANT_TYPE = "client_credentials";
  const EXPIRY_BUFFER_SECONDS = 60;

  // Time constants
  const CURRENT_TIME_SECONDS = 1_000;
  const TOKEN_EXPIRES_IN_SECONDS = 120;
  const TOKEN_ALREADY_EXPIRED_TIME = 500;
  const TOKEN_VALID_TIME = 430;

  // Token constants
  const NEW_ACCESS_TOKEN = "new-token";
  const CUSTOM_TOKEN_TYPE = "Custom";
  const EXPIRES_AT_TOKEN = "expires-at-token";
  const EXPIRES_AT_VALUE = 9_999;
  const EXPIRED_TOKEN = "expired-token";
  const NO_EXPIRY_TOKEN = "no-expiry-token";
  const DEFAULT_TOKEN = "default-token";

  let logger: jest.Mocked<ILogger>;
  let retryService: jest.Mocked<IRetryService>;
  let client: OAuth2TokenClient;

  beforeEach(() => {
    logger = createLoggerMock();
    retryService = createRetryServiceMock();
    mockedAxios.post.mockReset();
    getCurrentUnixTimestampSecondsMock.mockReset();

    client = new OAuth2TokenClient(
      logger,
      retryService,
      TOKEN_URL,
      CLIENT_ID,
      CLIENT_SECRET,
      AUDIENCE,
      GRANT_TYPE,
      EXPIRY_BUFFER_SECONDS,
    );
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it("return cached token when valid", async () => {
    // Arrange
    getCurrentUnixTimestampSecondsMock.mockReturnValue(TOKEN_VALID_TIME);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: NEW_ACCESS_TOKEN,
        token_type: CUSTOM_TOKEN_TYPE,
        expires_in: TOKEN_EXPIRES_IN_SECONDS,
      },
    });

    // Pre-populate cache by fetching a token first
    await client.getBearerToken();

    // Reset mocks to verify cache hit behavior
    mockedAxios.post.mockClear();
    retryService.retry.mockClear();

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBe(`${CUSTOM_TOKEN_TYPE} ${NEW_ACCESS_TOKEN}`);
    expect(retryService.retry).not.toHaveBeenCalled();
    expect(mockedAxios.post).not.toHaveBeenCalled();
  });

  it("request new token and store expiration based on expires_in", async () => {
    // Arrange
    getCurrentUnixTimestampSecondsMock.mockReturnValue(CURRENT_TIME_SECONDS);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: NEW_ACCESS_TOKEN,
        token_type: CUSTOM_TOKEN_TYPE,
        expires_in: TOKEN_EXPIRES_IN_SECONDS,
      },
    });

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBe(`${CUSTOM_TOKEN_TYPE} ${NEW_ACCESS_TOKEN}`);
    expect(retryService.retry).toHaveBeenCalledTimes(1);

    const expectedBody = {
      client_id: CLIENT_ID,
      client_secret: CLIENT_SECRET,
      audience: AUDIENCE,
      grant_type: GRANT_TYPE,
    };
    expect(mockedAxios.post).toHaveBeenCalledWith(
      TOKEN_URL,
      expectedBody,
      expect.objectContaining({
        headers: { "content-type": "application/json" },
      }),
    );

    // Verify token is cached by making second request
    mockedAxios.post.mockClear();
    retryService.retry.mockClear();
    const cachedToken = await client.getBearerToken();
    expect(cachedToken).toBe(`${CUSTOM_TOKEN_TYPE} ${NEW_ACCESS_TOKEN}`);
    expect(retryService.retry).not.toHaveBeenCalled();
    expect(mockedAxios.post).not.toHaveBeenCalled();
  });

  it("request new token and respect expires_at when provided", async () => {
    // Arrange
    getCurrentUnixTimestampSecondsMock.mockReturnValue(CURRENT_TIME_SECONDS);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: EXPIRES_AT_TOKEN,
        token_type: undefined,
        expires_at: EXPIRES_AT_VALUE,
      },
    });

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBe(`Bearer ${EXPIRES_AT_TOKEN}`);

    // Verify token is cached by making second request
    mockedAxios.post.mockClear();
    retryService.retry.mockClear();
    const cachedToken = await client.getBearerToken();
    expect(cachedToken).toBe(`Bearer ${EXPIRES_AT_TOKEN}`);
    expect(retryService.retry).not.toHaveBeenCalled();
    expect(mockedAxios.post).not.toHaveBeenCalled();
  });

  it("return undefined when access_token is missing", async () => {
    // Arrange
    mockedAxios.post.mockResolvedValue({
      data: {},
    });

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("Failed to retrieve OAuth2 access token");
  });

  it("return undefined when expires_at is already elapsed", async () => {
    // Arrange
    getCurrentUnixTimestampSecondsMock.mockReturnValue(CURRENT_TIME_SECONDS);
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: EXPIRED_TOKEN,
        expires_at: TOKEN_ALREADY_EXPIRED_TIME,
      },
    });

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith(`OAuth2 access token already expired at ${TOKEN_ALREADY_EXPIRED_TIME}`);
  });

  it("return undefined when expiry metadata is missing", async () => {
    // Arrange
    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: NO_EXPIRY_TOKEN,
      },
    });

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("OAuth2 access token did not provide expiry data");
  });

  it("use default grant type and expiry buffer when omitted", async () => {
    // Arrange
    client = new OAuth2TokenClient(logger, retryService, TOKEN_URL, CLIENT_ID, CLIENT_SECRET, AUDIENCE);

    mockedAxios.post.mockResolvedValue({
      data: {
        access_token: DEFAULT_TOKEN,
        expires_in: TOKEN_EXPIRES_IN_SECONDS,
      },
    });

    getCurrentUnixTimestampSecondsMock.mockReturnValue(CURRENT_TIME_SECONDS);

    // Act
    const token = await client.getBearerToken();

    // Assert
    expect(token).toBe(`Bearer ${DEFAULT_TOKEN}`);
    expect(mockedAxios.post).toHaveBeenCalledTimes(1);
    expect(mockedAxios.post).toHaveBeenCalledWith(
      TOKEN_URL,
      {
        client_id: CLIENT_ID,
        client_secret: CLIENT_SECRET,
        audience: AUDIENCE,
        grant_type: GRANT_TYPE,
      },
      expect.objectContaining({
        headers: { "content-type": "application/json" },
      }),
    );
  });
});
