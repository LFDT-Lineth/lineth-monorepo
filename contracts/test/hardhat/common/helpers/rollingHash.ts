import { expect } from "chai";
import { type Hash, type Hex, zeroHash } from "viem";

import { type AssertionTarget, expectEvent } from "./expectations.js";

/**
 * Initial rolling hash value (zero hash)
 */
export const INITIAL_ROLLING_HASH: Hex = zeroHash;

/**
 * Configuration for validating rolling hash storage
 */
export interface RollingHashStorageConfig {
  /** The contract instance to query */
  contract: {
    read: {
      rollingHashes?: (args: readonly [bigint]) => Promise<Hex>;
      l1RollingHashes?: (args: readonly [bigint]) => Promise<Hex>;
    };
  };
  /** The message number to check */
  messageNumber: bigint;
  /** The expected rolling hash value */
  expectedHash: Hex;
  /** Whether this is an L1 rolling hash (uses l1RollingHashes) or standard (uses rollingHashes) */
  isL1RollingHash?: boolean | undefined;
}

/**
 * Configuration for testing rolling hash update events
 */
export interface RollingHashEventConfig {
  /** The contract instance */
  contract: AssertionTarget;
  /** The async call that should emit the event */
  updateCall: Promise<Hash>;
  /** The message number for the event */
  messageNumber: bigint;
  /** The expected rolling hash in the event */
  expectedRollingHash: Hex;
  /** The message hash that was added */
  messageHash: Hex;
}

/**
 * Validates that a rolling hash is stored correctly in the contract.
 *
 * @param config - Configuration for the storage validation
 *
 * @example
 * ```typescript
 * await validateRollingHashStorage({
 *   contract: messageService,
 *   messageNumber: 1n,
 *   expectedHash: computedRollingHash,
 * });
 * ```
 */
export async function validateRollingHashStorage(config: RollingHashStorageConfig): Promise<void> {
  const { contract, messageNumber, expectedHash, isL1RollingHash } = config;

  let storedHash: Hex;
  if (isL1RollingHash && contract.read.l1RollingHashes) {
    storedHash = await contract.read.l1RollingHashes([messageNumber]);
  } else if (contract.read.rollingHashes) {
    storedHash = await contract.read.rollingHashes([messageNumber]);
  } else {
    throw new Error("Contract does not have rollingHashes or l1RollingHashes method");
  }

  expect(storedHash, `Rolling hash at message number ${messageNumber} should match expected`).to.equal(expectedHash);
}

/**
 * Validates that a rolling hash is not the zero hash.
 *
 * @param config - Configuration for the storage validation (without expectedHash)
 *
 * @example
 * ```typescript
 * await validateRollingHashNotZero({
 *   contract: messageService,
 *   messageNumber: 1n,
 * });
 * ```
 */
export async function validateRollingHashNotZero(
  config: Omit<RollingHashStorageConfig, "expectedHash">,
): Promise<void> {
  const { contract, messageNumber, isL1RollingHash } = config;

  let storedHash: Hex;
  if (isL1RollingHash && contract.read.l1RollingHashes) {
    storedHash = await contract.read.l1RollingHashes([messageNumber]);
  } else if (contract.read.rollingHashes) {
    storedHash = await contract.read.rollingHashes([messageNumber]);
  } else {
    throw new Error("Contract does not have rollingHashes or l1RollingHashes method");
  }

  expect(storedHash, `Rolling hash at message number ${messageNumber} should not be zero`).to.not.equal(
    INITIAL_ROLLING_HASH,
  );
}

/**
 * Validates that a rolling hash is exactly the zero hash.
 * Useful for testing that a hash was not set or was cleared.
 *
 * @param config - Configuration for the storage validation (without expectedHash)
 *
 * @example
 * ```typescript
 * await validateRollingHashIsZero({
 *   contract: messageService,
 *   messageNumber: 100n,
 * });
 * ```
 */
export async function validateRollingHashIsZero(config: Omit<RollingHashStorageConfig, "expectedHash">): Promise<void> {
  const { contract, messageNumber, isL1RollingHash } = config;

  let storedHash: Hex;
  if (isL1RollingHash && contract.read.l1RollingHashes) {
    storedHash = await contract.read.l1RollingHashes([messageNumber]);
  } else if (contract.read.rollingHashes) {
    storedHash = await contract.read.rollingHashes([messageNumber]);
  } else {
    throw new Error("Contract does not have rollingHashes or l1RollingHashes method");
  }

  expect(storedHash, `Rolling hash at message number ${messageNumber} should be zero`).to.equal(INITIAL_ROLLING_HASH);
}

/**
 * Tests that a RollingHashUpdated event is emitted with expected values.
 *
 * @param config - Configuration for the event test
 *
 * @example
 * ```typescript
 * await expectRollingHashUpdatedEvent({
 *   contract: messageService,
 *   updateCall: messageService.sendMessage(recipient, fee, calldata, { value }),
 *   messageNumber: 1n,
 *   expectedRollingHash: computedHash,
 *   messageHash: computedMessageHash,
 * });
 * ```
 */
export async function expectRollingHashUpdatedEvent(config: RollingHashEventConfig): Promise<void> {
  const { contract, updateCall, messageNumber, expectedRollingHash, messageHash } = config;

  await expectEvent(contract, updateCall, "RollingHashUpdated", [messageNumber, expectedRollingHash, messageHash]);
}
