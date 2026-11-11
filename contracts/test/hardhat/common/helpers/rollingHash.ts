import { expect } from "chai";
import { type Abi, type Address, type Hash, type Hex, zeroHash } from "viem";

import { type AssertionTarget, expectEvent, expectRevertWithCustomError } from "./expectations.js";
import { calculateRollingHash, generateKeccak256 } from "./hashing.js";

/**
 * Initial rolling hash value (zero hash)
 */
export const INITIAL_ROLLING_HASH: Hex = zeroHash;

/**
 * Common error names for rolling hash validation
 */
export const ROLLING_HASH_ERRORS = {
  ROLLING_HASH_MISMATCH: "RollingHashMismatch",
  ZERO_HASH_NOT_ALLOWED: "ZeroHashNotAllowed",
  INVALID_MESSAGE_NUMBER: "InvalidMessageNumber",
  FINAL_ROLLING_HASH_MISMATCH: "FinalRollingHashMismatch",
} as const;

/**
 * Common event names for rolling hash updates
 */
export const ROLLING_HASH_EVENTS = {
  ROLLING_HASH_UPDATED: "RollingHashUpdated",
} as const;

/** Any deployed viem contract instance (address + ABI). */
type DeployedContract = { abi: Abi; address: Address };

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
export interface RollingHashEventConfig<TContract extends DeployedContract> {
  /** The contract instance */
  contract: TContract;
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
 * Configuration for testing rolling hash mismatch errors
 */
export interface RollingHashMismatchConfig<TContract extends DeployedContract> {
  /** The contract instance */
  contract: TContract;
  /** The async call that should revert */
  call: Promise<unknown>;
  /** Optional: The expected error name (defaults to RollingHashMismatch) */
  errorName?: string;
  /** Optional: Expected error arguments */
  errorArgs?: unknown[];
}

/**
 * Result from computing a rolling hash chain
 */
export interface RollingHashChainResult {
  /** The final rolling hash after processing all messages */
  finalHash: Hex;
  /** Intermediate rolling hashes at each step */
  intermediateHashes: Hex[];
  /** The message hashes that were processed */
  messageHashes: Hex[];
}

/**
 * Computes a rolling hash chain from a starting hash and multiple message hashes.
 * Returns detailed information about the computation.
 *
 * @param startingHash - The initial rolling hash (usually zero hash)
 * @param messageHashes - Array of message hashes to process
 * @returns Object containing final hash, intermediate hashes, and input hashes
 *
 * @example
 * ```typescript
 * const result = computeRollingHashChain(zeroHash, [hash1, hash2, hash3]);
 * console.log(result.finalHash); // Final rolling hash
 * console.log(result.intermediateHashes); // [hash after 1, hash after 2, hash after 3]
 * ```
 */
export function computeRollingHashChain(startingHash: Hex, messageHashes: Hex[]): RollingHashChainResult {
  const intermediateHashes: Hex[] = [];
  let currentHash = startingHash;

  for (const messageHash of messageHashes) {
    currentHash = calculateRollingHash(currentHash, messageHash);
    intermediateHashes.push(currentHash);
  }

  return {
    finalHash: currentHash,
    intermediateHashes,
    messageHashes,
  };
}

/**
 * Generates a message hash for testing purposes.
 * Uses keccak256 of the provided data.
 *
 * @param sender - The sender address
 * @param receiver - The receiver address
 * @param fee - The message fee
 * @param value - The message value
 * @param nonce - The message nonce
 * @param calldata - The message calldata
 * @returns The computed message hash
 *
 * @example
 * ```typescript
 * const messageHash = generateMessageHash(
 *   sender.address,
 *   receiver.address,
 *   parseEther("0.05"),
 *   parseEther("1"),
 *   1n,
 *   "0x"
 * );
 * ```
 */
export function generateMessageHash(
  sender: Address,
  receiver: Address,
  fee: bigint,
  value: bigint,
  nonce: bigint,
  calldata: Hex,
): Hex {
  return generateKeccak256(
    ["address", "address", "uint256", "uint256", "uint256", "bytes"],
    [sender, receiver, fee, value, nonce, calldata],
  );
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
export async function expectRollingHashUpdatedEvent<TContract extends DeployedContract>(
  config: RollingHashEventConfig<TContract>,
): Promise<void> {
  const { contract, updateCall, messageNumber, expectedRollingHash, messageHash } = config;

  const target: AssertionTarget = contract;
  await expectEvent(target, updateCall, ROLLING_HASH_EVENTS.ROLLING_HASH_UPDATED, [
    messageNumber,
    expectedRollingHash,
    messageHash,
  ]);
}

/**
 * Tests that an operation reverts due to a rolling hash mismatch.
 *
 * @param config - Configuration for the mismatch test
 *
 * @example
 * ```typescript
 * await expectRollingHashMismatchRevert({
 *   contract: messageManager,
 *   call: messageManager.anchorL1L2MessageHashes(messageHashes, 100n, badRollingHash),
 *   errorArgs: [expectedHash, actualHash],
 * });
 * ```
 */
export async function expectRollingHashMismatchRevert<TContract extends DeployedContract>(
  config: RollingHashMismatchConfig<TContract>,
): Promise<void> {
  const errorName = config.errorName ?? ROLLING_HASH_ERRORS.ROLLING_HASH_MISMATCH;
  const target: AssertionTarget = config.contract;
  await expectRevertWithCustomError(target, config.call, errorName, config.errorArgs);
}

/**
 * Computes rolling hash and validates storage in one operation.
 * Useful for verifying state after a message operation.
 *
 * @param contract - The contract to query
 * @param startingHash - The rolling hash before the operation
 * @param messageHash - The message hash that was added
 * @param messageNumber - The message number to check
 * @param isL1RollingHash - Whether to use l1RollingHashes method
 *
 * @example
 * ```typescript
 * const expectedHash = await computeAndValidateRollingHash(
 *   messageService,
 *   zeroHash,
 *   messageHash,
 *   1n,
 * );
 * ```
 */
export async function computeAndValidateRollingHash(
  contract: RollingHashStorageConfig["contract"],
  startingHash: Hex,
  messageHash: Hex,
  messageNumber: bigint,
  isL1RollingHash?: boolean,
): Promise<Hex> {
  const expectedHash = calculateRollingHash(startingHash, messageHash);

  await validateRollingHashStorage({
    contract,
    messageNumber,
    expectedHash,
    isL1RollingHash,
  });

  return expectedHash;
}

/**
 * Creates a sequence of message hashes for testing.
 * Each hash is derived from a base string and index.
 *
 * @param baseString - Base string for generating hashes
 * @param count - Number of hashes to generate
 * @returns Array of message hashes
 *
 * @example
 * ```typescript
 * const hashes = createMessageHashSequence("test-message", 10);
 * ```
 */
export function createMessageHashSequence(baseString: string, count: number): Hex[] {
  const hashes: Hex[] = [];
  for (let i = 1; i <= count; i++) {
    hashes.push(generateKeccak256(["string"], [`${baseString}${i}`], true));
  }
  return hashes;
}

/**
 * Validates an entire rolling hash chain at multiple message numbers.
 *
 * @param contract - The contract to query
 * @param chainResult - The computed chain result
 * @param startingMessageNumber - The first message number in the sequence
 * @param isL1RollingHash - Whether to use l1RollingHashes method
 *
 * @example
 * ```typescript
 * const chain = computeRollingHashChain(zeroHash, messageHashes);
 * await validateRollingHashChain(messageService, chain, 1n);
 * ```
 */
export async function validateRollingHashChain(
  contract: RollingHashStorageConfig["contract"],
  chainResult: RollingHashChainResult,
  startingMessageNumber: bigint,
  isL1RollingHash?: boolean,
): Promise<void> {
  for (let i = 0; i < chainResult.intermediateHashes.length; i++) {
    await validateRollingHashStorage({
      contract,
      messageNumber: startingMessageNumber + BigInt(i),
      expectedHash: chainResult.intermediateHashes[i],
      isL1RollingHash,
    });
  }
}
