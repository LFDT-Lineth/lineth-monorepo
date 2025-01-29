import { ethers } from "ethers";
import { encodeData } from "./encoding";

export const generateKeccak256Hash = (str: string) => generateKeccak256(["string"], [str], true);

export const generateKeccak256 = (types: string[], values: unknown[], packed?: boolean) =>
  ethers.keccak256(encodeData(types, values, packed));

export const generateNKeccak256Hashes = (str: string, numberOfHashToGenerate: number): string[] => {
  let arr: string[] = [];
  for (let i = 1; i < numberOfHashToGenerate + 1; i++) {
    arr = [...arr, generateKeccak256(["string"], [`${str}${i}`], true)];
  }
  return arr;
};

export function calculateLastFinalizedState(
  l1RollingHashMessageNumber: bigint,
  l1RollingHash: string,
  finalTimestamp: bigint,
): string {
  return generateKeccak256(
    ["uint256", "bytes32", "uint256"],
    [l1RollingHashMessageNumber, l1RollingHash, finalTimestamp],
  );
}

export function calculateRollingHash(existingRollingHash: string, messageHash: string) {
  return generateKeccak256(["bytes32", "bytes32"], [existingRollingHash, messageHash]);
}

export function calculateRollingHashFromCollection(existingRollingHash: string, messageHashes: string[]) {
  return messageHashes.reduce((rollingHash, hash) => calculateRollingHash(rollingHash, hash), existingRollingHash);
}
