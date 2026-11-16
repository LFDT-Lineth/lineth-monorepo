import { type Hex, keccak256, stringToHex } from "viem";

import { generateKeccak256 } from "../../../../common/helpers/hashing.js";

export const generateKeccak256ForString = (value: string): Hex => keccak256(stringToHex(value));

export const generateNKeccak256Hashes = (str: string, numberOfHashToGenerate: number): Hex[] => {
  let arr: Hex[] = [];
  for (let i = 1; i < numberOfHashToGenerate + 1; i++) {
    arr = [...arr, generateKeccak256ForString(`${str}${i}`)];
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
    { encodePacked: false },
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
    { encodePacked: false },
  );
}

export function calculateRollingHash(existingRollingHash: Hex, messageHash: Hex): Hex {
  return generateKeccak256(["bytes32", "bytes32"], [existingRollingHash, messageHash], { encodePacked: false });
}

export function calculateRollingHashFromCollection(existingRollingHash: Hex, messageHashes: Hex[]): Hex {
  return messageHashes.reduce((rollingHash, hash) => calculateRollingHash(rollingHash, hash), existingRollingHash);
}
