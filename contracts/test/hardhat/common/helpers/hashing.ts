import { type Hex, keccak256 } from "viem";

import { encodeData } from "./encoding.js";

export const generateKeccak256BytesDirectly = (data: Hex) => keccak256(data);

export const generateKeccak256Hash = (str: string) => generateKeccak256(["string"], [str], true);

export const generateKeccak256 = (types: string[], values: unknown[], packed?: boolean): Hex =>
  keccak256(encodeData(types, values, packed));

export const generateNKeccak256Hashes = (str: string, numberOfHashToGenerate: number): Hex[] => {
  let arr: Hex[] = [];
  for (let i = 1; i < numberOfHashToGenerate + 1; i++) {
    arr = [...arr, generateKeccak256(["string"], [`${str}${i}`], true)];
  }
  return arr;
};

export function calculateLastFinalizedState(
  l1RollingHashMessageNumber: bigint,
  l1RollingHash: Hex,
  forcedTransactionNumber: bigint,
  forcedTransactionRollingHash: Hex,
  finalTimestamp: bigint,
): Hex {
  return generateKeccak256(
    ["uint256", "bytes32", "uint256", "bytes32", "uint256"],
    [l1RollingHashMessageNumber, l1RollingHash, forcedTransactionNumber, forcedTransactionRollingHash, finalTimestamp],
  );
}

export function calculateLastFinalizedStateV6(
  l1RollingHashMessageNumber: bigint,
  l1RollingHash: Hex,
  finalTimestamp: bigint,
): Hex {
  return generateKeccak256(
    ["uint256", "bytes32", "uint256"],
    [l1RollingHashMessageNumber, l1RollingHash, finalTimestamp],
  );
}

export function calculateRollingHash(existingRollingHash: Hex, messageHash: Hex): Hex {
  return generateKeccak256(["bytes32", "bytes32"], [existingRollingHash, messageHash]);
}

export function calculateRollingHashFromCollection(existingRollingHash: Hex, messageHashes: Hex[]): Hex {
  return messageHashes.reduce((rollingHash, hash) => calculateRollingHash(rollingHash, hash), existingRollingHash);
}
