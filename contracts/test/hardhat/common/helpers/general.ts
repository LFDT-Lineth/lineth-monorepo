import { randomBytes } from "node:crypto";
import { type Address, type Hex, bytesToHex, isHex } from "viem";
import { generatePrivateKey, privateKeyToAddress } from "viem/accounts";

export const range = (start: number, end: number) => Array.from(Array(end - start + 1).keys()).map((x) => x + start);

export const generateRandomBytes = (length: number): Hex => bytesToHex(randomBytes(length));

/** Random address, replaces ethers' `Wallet.createRandom().address`. */
export const randomAddress = (): Address => privateKeyToAddress(generatePrivateKey());

export function buildAccessErrorMessage(account: { address: string }, role: string): string {
  return `AccessControl: account ${account.address.toLowerCase()} is missing role ${role}`;
}

/** Validates a `0x`-prefixed hex string coming from untyped data (JSON fixtures, env) and types it as `Hex`. */
export function hex(value: string): Hex {
  if (!isHex(value)) {
    throw new Error(`Expected a 0x-prefixed hex string, got: ${value}`);
  }
  return value;
}

/** Runs `fn` while capturing `console.log` / `console.warn` lines. */
export async function captureLogs<T>(fn: () => Promise<T>): Promise<{ result: T; lines: string[] }> {
  const { log, warn } = console;
  const lines: string[] = [];
  const capture = (...args: unknown[]) => {
    lines.push(args.map((arg) => String(arg)).join(" "));
  };
  console.log = capture;
  console.warn = capture;
  try {
    const result = await fn();
    return { result, lines };
  } finally {
    console.log = log;
    console.warn = warn;
  }
}
