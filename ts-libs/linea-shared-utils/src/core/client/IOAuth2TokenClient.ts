export interface IOAuth2TokenClient {
  getBearerToken(): Promise<string | undefined>;
}

export interface OAuth2TokenResponse {
  access_token?: string;
  token_type?: string;
  // Expected in seconds
  expires_in?: number;
  // Expected as Unix timestamp in seconds
  expires_at?: number;
}
