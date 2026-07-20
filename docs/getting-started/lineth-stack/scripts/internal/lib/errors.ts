// Shared error/log redaction helpers for the quickstart internal TypeScript.
//
// Keep a single redaction source so the security-sensitive redactors cannot
// silently drift between callers.

/** Redacts URLs and 32-byte hex (0x + 64 hex) from arbitrary error/log text. */
export function sanitizeExternalError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return message
    .replace(/https?:\/\/[^\s)"']+/g, "<redacted-url>")
    .replace(/0x[a-fA-F0-9]{64}/g, "<redacted-hex>");
}

/** Redacts an explicit list of secrets (and case variants) from error text. */
export function sanitizeSecrets(error: unknown, secrets: Array<string | undefined> = []): string {
  let message = error instanceof Error ? error.message : String(error);
  for (const secret of secrets) {
    if (!secret) {
      continue;
    }
    message = message.split(secret).join("[REDACTED]");
    message = message.split(secret.toLowerCase()).join("[REDACTED]");
    message = message.split(secret.toUpperCase()).join("[REDACTED]");
  }
  return message;
}
