/** Returns true when `context` looks like an environment-variable name (optional [index] suffix). */
export function isEnvVarContext(context: string): boolean {
  return /^[A-Z][A-Z0-9_]*(\[\d+\])?$/.test(context);
}

export function envVarNameFromContext(context: string): string {
  return context.replace(/\[\d+\]$/, "");
}

/** Env var names whose values must never appear in deploy logs or error messages. */
const SENSITIVE_ENV_VAR_PATTERNS: RegExp[] = [
  /PRIVATE_KEY/,
  /SECRET/,
  /PASSWORD/,
  /MNEMONIC/,
  /SEED_PHRASE/,
  /CREDENTIAL/,
  /API_KEY/,
  /_RPC$/,
  /^RPC$/,
  /WEBSOCKET/,
  /_URL$/,
  /ETHERSCAN/,
  /INFURA/,
  /AUTH_TOKEN/,
  /BEARER/,
  /JWT/,
];

/**
 * Returns true when an env var value must be redacted from deploy logs and error messages.
 * Keys that do not match the denylist are treated as safe to log.
 */
export function isSensitiveEnvVar(name: string): boolean {
  const base = envVarNameFromContext(name).toUpperCase();
  return SENSITIVE_ENV_VAR_PATTERNS.some((pattern) => pattern.test(base));
}

export function formatEnvVarForLog(name: string, value: string): string {
  return isSensitiveEnvVar(name) ? name : `${name}=${value}`;
}

export function formatEnvVarValueForMessage(name: string, value: string): string {
  return isSensitiveEnvVar(name) ? "[REDACTED]" : value;
}
