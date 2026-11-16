import { type Hex, isHex, size } from "viem";

/**
 * Parses a 32-byte hex private key, with or without the `0x` prefix (ethers accepted both).
 * The error names `label` but never echoes the key.
 */
export function parsePrivateKey(value: string, label: string): Hex {
  const key = value.startsWith("0x") ? value : `0x${value}`;
  if (!isHex(key, { strict: true }) || size(key) !== 32) {
    throw new Error(`${label} must be a 32-byte hex private key (64 hex characters, optional 0x prefix).`);
  }
  return key;
}
