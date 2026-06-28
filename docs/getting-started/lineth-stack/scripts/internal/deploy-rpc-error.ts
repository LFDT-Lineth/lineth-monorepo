const PUBLIC_RPC_SUBMISSION_MARKERS = [
  "already known",
  "nonce too low",
  "replacement transaction underpriced",
  "transaction underpriced",
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function collectErrorStrings(value: unknown, seen = new Set<unknown>()): string[] {
  if (value === undefined || value === null || seen.has(value)) {
    return [];
  }
  seen.add(value);

  if (typeof value === "string") {
    return [value];
  }

  if (typeof value === "number" || typeof value === "bigint") {
    return [String(value)];
  }

  if (value instanceof Error) {
    return [value.name, value.message, ...collectErrorStrings((value as Error & { cause?: unknown }).cause, seen)];
  }

  if (Array.isArray(value)) {
    return value.flatMap((item) => collectErrorStrings(item, seen));
  }

  if (!isRecord(value)) {
    return [];
  }

  return ["code", "message", "shortMessage", "reason", "method", "error", "payload"].flatMap((key) =>
    collectErrorStrings(value[key], seen),
  );
}

export function formatQuickstartDeployRpcError(error: unknown): Error | undefined {
  const errorText = collectErrorStrings(error).join(" ").toLowerCase();
  const isRawTransactionSubmission = errorText.includes("eth_sendrawtransaction");
  const marker = PUBLIC_RPC_SUBMISSION_MARKERS.find((candidate) => errorText.includes(candidate));

  if (!isRawTransactionSubmission || marker === undefined) {
    return undefined;
  }

  return new Error(
    [
      `[deploy-contracts] L1 contract deployment hit an RPC submission error: ${marker}.`,
      "This usually means the free or overloaded Sepolia RPC endpoint accepted the raw transaction on one backend,",
      "then another backend returned a stale or conflicting mempool response.",
      "Use a dedicated paid Sepolia RPC endpoint for L1_RPC_URL, then run ./scripts/reset.sh before retrying the quickstart.",
      "This is not fixed by changing gas or funding amounts.",
    ].join(" "),
  );
}
