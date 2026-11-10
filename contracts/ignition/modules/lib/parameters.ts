import { toHex } from "viem";

import { formatEnvVarValueForMessage } from "../../../common/helpers/envVarLogging.js";

import type { ModuleParameters, SolidityParameterType } from "@nomicfoundation/ignition-core";
import type { Hex } from "viem";

/** Value parsers and helpers shared by the `<Module>.deploy.ts` parameter builders. */

/** Validates untyped values (env JSON, constants) as Ignition parameter values. */
export function toSolidityParameter(value: unknown, context: string): SolidityParameterType {
  if (
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "bigint" ||
    typeof value === "boolean"
  ) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item, index) => toSolidityParameter(item, `${context}[${index}]`));
  }
  if (value !== null && typeof value === "object") {
    const result: { [field: string]: SolidityParameterType } = {};
    for (const [key, item] of Object.entries(value)) {
      result[key] = toSolidityParameter(item, `${context}.${key}`);
    }
    return result;
  }
  throw new Error(`${context}: unsupported parameter value ${String(value)}`);
}
/** Parses an unsigned integer env value (decimal or 0x hex, as ethers accepted) to a bigint. */
export function toUint(name: string, raw: string): bigint {
  const trimmed = raw.trim();
  if (!/^(0x[0-9a-fA-F]+|\d+)$/.test(trimmed)) {
    throw new Error(`${name} must be an unsigned integer, got "${formatEnvVarValueForMessage(name, raw)}"`);
  }
  return BigInt(trimmed);
}
export function toInteger(name: string, raw: string): number {
  const value = Number(raw.trim());
  if (!Number.isSafeInteger(value)) {
    throw new Error(`${name} must be an integer, got "${formatEnvVarValueForMessage(name, raw)}"`);
  }
  return value;
}
/** `ethers.toBeHex(value, 32)`. */
export function toBytes32(name: string, raw: string): Hex {
  return toHex(toUint(name, raw), { size: 32 });
}
/**
 * Strict bool for a constructor `bool` argument. ethers (and Ignition, which encodes with ethers) treats any non-empty
 * string as `true`, so `"false"`/`"0"` used to deploy `true`; accept only true/false/1/0.
 */
export function toBool(name: string, raw: string): boolean {
  const value = raw.trim().toLowerCase();
  if (value === "true" || value === "1") return true;
  if (value === "false" || value === "0") return false;
  throw new Error(`${name} must be one of true, false, 1, 0; got "${formatEnvVarValueForMessage(name, raw)}"`);
}
/** Same address list ordering and contents the scripts passed (`split(",")`). */
export function splitList(raw: string): string[] {
  return raw.split(",");
}
export function withoutKey(parameters: ModuleParameters, key: string): ModuleParameters {
  const result: ModuleParameters = {};
  for (const [name, value] of Object.entries(parameters)) {
    if (name !== key) {
      result[name] = value;
    }
  }
  return result;
}
