const VERIFY_BIGINT_SENTINEL = "__linea_verify_bigint__";

export function stringifyVerifyTaskArgs(args: Record<string, unknown>): string {
  return JSON.stringify(args, (_key, value: unknown) => {
    if (typeof value === "bigint") {
      return {
        [VERIFY_BIGINT_SENTINEL]: value.toString(),
      };
    }
    return value;
  });
}

export function parseVerifyTaskArgs(rawArgs: string): Record<string, unknown> {
  return JSON.parse(rawArgs, (_key, value: unknown) => {
    const maybeSerializedBigInt =
      value !== null && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : null;

    if (
      maybeSerializedBigInt !== null &&
      Object.keys(maybeSerializedBigInt).length === 1 &&
      typeof maybeSerializedBigInt[VERIFY_BIGINT_SENTINEL] === "string"
    ) {
      return BigInt(maybeSerializedBigInt[VERIFY_BIGINT_SENTINEL] as string);
    }
    return value;
  }) as Record<string, unknown>;
}
