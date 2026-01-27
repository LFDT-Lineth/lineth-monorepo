import axios from "axios";

import { IOAuth2TokenClient, OAuth2TokenResponse } from "../core/client/IOAuth2TokenClient";
import { IRetryService } from "../core/services/IRetryService";
import { ILogger } from "../logging/ILogger";
import { getCurrentUnixTimestampSeconds } from "../utils/time";

/**
 * Client for obtaining and managing OAuth2 bearer tokens.
 * Implements token caching with automatic refresh when tokens are near expiration.
 * Uses the client credentials grant type to obtain access tokens.
 */
export class OAuth2TokenClient implements IOAuth2TokenClient {
  private bearerToken?: string;
  private tokenExpiresAtSeconds?: number;

  /**
   * Creates a new OAuth2TokenClient instance.
   *
   * @param {ILogger} logger - The logger instance for logging token operations.
   * @param {IRetryService} retryService - The retry service for handling failed token requests.
   * @param {string} tokenUrl - The OAuth2 token endpoint URL.
   * @param {string} clientId - The OAuth2 client ID.
   * @param {string} clientSecret - The OAuth2 client secret.
   * @param {string} audience - The OAuth2 audience identifier.
   * @param {string} [grantType="client_credentials"] - The OAuth2 grant type to use.
   * @param {number} [expiryBufferSeconds=60] - Buffer time in seconds before token expiration to trigger refresh.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly retryService: IRetryService,
    private readonly tokenUrl: string,
    private readonly clientId: string,
    private readonly clientSecret: string,
    private readonly audience: string,
    private readonly grantType: string = "client_credentials",
    private readonly expiryBufferSeconds: number = 60,
  ) {}

  /**
   * Retrieves a valid bearer token, using cached token if available and not expired.
   * Automatically requests a new token if the cached token is expired or missing.
   * The token is refreshed when it's within the expiry buffer time of expiration.
   *
   * @returns {Promise<string | undefined>} The bearer token string (e.g., "Bearer <token>") if successful, undefined if the request fails or the token is invalid.
   */
  async getBearerToken(): Promise<string | undefined> {
    // Serve cached token while it remains valid.
    if (
      this.bearerToken &&
      this.tokenExpiresAtSeconds &&
      getCurrentUnixTimestampSeconds() < this.tokenExpiresAtSeconds - this.expiryBufferSeconds
    ) {
      this.logger.info("getBearerToken cache-hit");
      return this.bearerToken;
    }

    this.logger.info("getBearerToken requesting new token");
    const { data } = await this.retryService.retry(() =>
      axios.post<OAuth2TokenResponse>(
        this.tokenUrl,
        {
          client_id: this.clientId,
          client_secret: this.clientSecret,
          audience: this.audience,
          grant_type: this.grantType,
        },
        {
          headers: {
            "content-type": "application/json",
          },
        },
      ),
    );

    if (!data?.access_token) {
      this.logger.error("Failed to retrieve OAuth2 access token");
      return undefined;
    }

    const tokenType = data.token_type ?? "Bearer";
    this.bearerToken = `${tokenType} ${data.access_token}`.trim();

    if (data?.expires_at) {
      const tokenExpiresAtSecondsCandidate = data?.expires_at;
      if (tokenExpiresAtSecondsCandidate < getCurrentUnixTimestampSeconds()) {
        this.logger.error(`OAuth2 access token already expired at ${tokenExpiresAtSecondsCandidate}`);
        return undefined;
      }
      this.tokenExpiresAtSeconds = tokenExpiresAtSecondsCandidate;
    } else if (data?.expires_in) {
      this.tokenExpiresAtSeconds = getCurrentUnixTimestampSeconds() + data?.expires_in;
    } else {
      this.logger.error(`OAuth2 access token did not provide expiry data`);
      return undefined;
    }

    this.logger.info(
      `getBearerToken successfully retrived new OAuth2 Bearer token tokenExpiresAtSeconds=${this.tokenExpiresAtSeconds}`,
    );

    return this.bearerToken;
  }
}
