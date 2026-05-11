/**
 * Must match `HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER` in
 * `contracts/scripts/hardhat/signer-ui-bridge.ts`
 * (HTTP headers are case-insensitive; Node normalizes to lowercase).
 */
export const HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER = "x-hardhat-signer-ui-session-token";

function parseErrorMessageFromBody(text: string): string | undefined {
  const trimmed = text.trim();
  if (trimmed.length === 0) {
    return undefined;
  }

  try {
    const parsed = JSON.parse(trimmed) as { error?: unknown };
    if (typeof parsed.error === "string") {
      const normalized = parsed.error.trim();
      if (normalized.length > 0) {
        return normalized;
      }
    }
  } catch {
    // Non-JSON body; use plain text below.
  }

  return trimmed;
}

export async function bootstrapSession(apiBaseUrl: string, fetchImpl: typeof fetch = fetch): Promise<string> {
  const response = await fetchImpl(`${apiBaseUrl}/api/bootstrap`, {
    method: "POST",
  });

  if (!response.ok) {
    const bodyText = await response.text();
    const errorMessage = parseErrorMessageFromBody(bodyText);
    throw new Error(errorMessage ?? `Session bootstrap failed with ${response.status}`);
  }

  const parsed = (await response.json()) as { sessionToken?: unknown };
  if (typeof parsed.sessionToken !== "string" || parsed.sessionToken.length === 0) {
    throw new Error("Session bootstrap response did not include a token.");
  }

  return parsed.sessionToken;
}

export async function postJson(
  url: string,
  payload: unknown,
  sessionSecret: string,
  fetchImpl: typeof fetch = fetch,
): Promise<void> {
  const response = await fetchImpl(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      [HARDHAT_SIGNER_UI_SESSION_TOKEN_HEADER]: sessionSecret,
    },
    body: JSON.stringify(payload),
  });

  if (response.ok) {
    return;
  }

  const bodyText = await response.text();
  const errorMessage = parseErrorMessageFromBody(bodyText);
  throw new Error(errorMessage ?? `Request to ${url} failed with ${response.status}`);
}

export const __testOnly = {
  parseErrorMessageFromBody,
};
