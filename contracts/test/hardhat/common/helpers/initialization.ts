import { BaseContract, ContractFactory } from "ethers";

import { expectRevertWithCustomError, expectRevertWithReason } from "./expectations";

/**
 * Standard error messages for initialization failures
 */
export const INITIALIZATION_ERROR_MESSAGES = {
  ALREADY_INITIALIZED: "Initializable: contract is already initialized",
  NOT_INITIALIZING: "Initializable: contract is not initializing",
} as const;

/**
 * Standard custom error names for zero value validation
 */
export const ZERO_VALUE_ERRORS = {
  ZERO_ADDRESS_NOT_ALLOWED: "ZeroAddressNotAllowed",
  ZERO_VALUE_NOT_ALLOWED: "ZeroValueNotAllowed",
  ZERO_LENGTH_NOT_ALLOWED: "ZeroLengthNotAllowed",
  ZERO_HASH_NOT_ALLOWED: "ZeroHashNotAllowed",
} as const;

/**
 * Configuration for testing initialization with zero values
 */
export interface ZeroValueTestConfig<TContract extends BaseContract | ContractFactory> {
  /** The contract instance used for error matching */
  contract: TContract;
  /** The async call that should revert */
  deployOrInitCall: Promise<unknown>;
  /** The expected custom error name */
  expectedError: string;
  /** Optional: Additional error arguments to validate */
  errorArgs?: unknown[];
}

/**
 * Configuration for testing double initialization
 */
export interface DoubleInitTestConfig<TContract extends BaseContract> {
  /** The contract instance that has already been initialized */
  contract: TContract;
  /** The async call that attempts to initialize again */
  initCall: Promise<unknown>;
  /** Whether the contract uses custom errors (true) or reason strings (false) */
  usesCustomError?: boolean;
  /** Optional: Custom error name if using custom errors */
  customErrorName?: string;
}

/**
 * Configuration for batch testing multiple zero value scenarios
 */
export interface ZeroValueBatchConfig<TContract extends BaseContract | ContractFactory> {
  /** The contract instance used for error matching */
  contract: TContract;
  /** Array of test scenarios */
  scenarios: Array<{
    /** Description of what's being tested */
    description: string;
    /** The async call that should revert */
    deployOrInitCall: Promise<unknown>;
    /** The expected custom error name */
    expectedError: string;
    /** Optional: Additional error arguments */
    errorArgs?: unknown[];
  }>;
}

/**
 * Tests that a deployment or initialization reverts when a zero address is provided.
 *
 * @param config - Configuration for the zero address test
 *
 * @example
 * ```typescript
 * await expectZeroAddressRevert({
 *   contract: myContract,
 *   deployOrInitCall: deployMyContract(ZERO_ADDRESS, validParam),
 *   expectedError: "ZeroAddressNotAllowed",
 * });
 * ```
 */
export async function expectZeroAddressRevert<TContract extends BaseContract | ContractFactory>(
  config: Omit<ZeroValueTestConfig<TContract>, "expectedError"> & { expectedError?: string },
): Promise<void> {
  const errorName = config.expectedError ?? ZERO_VALUE_ERRORS.ZERO_ADDRESS_NOT_ALLOWED;
  await expectRevertWithCustomError(config.contract, config.deployOrInitCall, errorName, config.errorArgs ?? []);
}

/**
 * Tests that a deployment or initialization reverts when a zero value is provided.
 *
 * @param config - Configuration for the zero value test
 *
 * @example
 * ```typescript
 * await expectZeroValueRevert({
 *   contract: myContract,
 *   deployOrInitCall: myContract.initialize(0n),
 *   expectedError: "ZeroValueNotAllowed",
 * });
 * ```
 */
export async function expectZeroValueRevert<TContract extends BaseContract | ContractFactory>(
  config: Omit<ZeroValueTestConfig<TContract>, "expectedError"> & { expectedError?: string },
): Promise<void> {
  const errorName = config.expectedError ?? ZERO_VALUE_ERRORS.ZERO_VALUE_NOT_ALLOWED;
  await expectRevertWithCustomError(config.contract, config.deployOrInitCall, errorName, config.errorArgs ?? []);
}

/**
 * Tests that a deployment or initialization reverts when a zero hash is provided.
 *
 * @param config - Configuration for the zero hash test
 *
 * @example
 * ```typescript
 * await expectZeroHashRevert({
 *   contract: myContract,
 *   deployOrInitCall: myContract.initialize(ZERO_HASH),
 *   expectedError: "ZeroHashNotAllowed",
 * });
 * ```
 */
export async function expectZeroHashRevert<TContract extends BaseContract | ContractFactory>(
  config: Omit<ZeroValueTestConfig<TContract>, "expectedError"> & { expectedError?: string },
): Promise<void> {
  const errorName = config.expectedError ?? ZERO_VALUE_ERRORS.ZERO_HASH_NOT_ALLOWED;
  await expectRevertWithCustomError(config.contract, config.deployOrInitCall, errorName, config.errorArgs ?? []);
}

/**
 * Tests that a deployment or initialization reverts when zero length data is provided.
 *
 * @param config - Configuration for the zero length test
 *
 * @example
 * ```typescript
 * await expectZeroLengthRevert({
 *   contract: myContract,
 *   deployOrInitCall: myContract.storeData("0x"),
 *   expectedError: "ZeroLengthNotAllowed",
 * });
 * ```
 */
export async function expectZeroLengthRevert<TContract extends BaseContract | ContractFactory>(
  config: Omit<ZeroValueTestConfig<TContract>, "expectedError"> & { expectedError?: string },
): Promise<void> {
  const errorName = config.expectedError ?? ZERO_VALUE_ERRORS.ZERO_LENGTH_NOT_ALLOWED;
  await expectRevertWithCustomError(config.contract, config.deployOrInitCall, errorName, config.errorArgs ?? []);
}

/**
 * Tests that attempting to initialize an already initialized contract reverts.
 * Supports both OpenZeppelin's reason strings and custom errors.
 *
 * @param config - Configuration for the double initialization test
 *
 * @example
 * ```typescript
 * // Using default reason string (OpenZeppelin Initializable)
 * await expectDoubleInitRevert({
 *   contract: myContract,
 *   initCall: myContract.initialize(params),
 * });
 *
 * // Using custom error
 * await expectDoubleInitRevert({
 *   contract: myContract,
 *   initCall: myContract.initialize(params),
 *   usesCustomError: true,
 *   customErrorName: "InvalidInitialization",
 * });
 * ```
 */
export async function expectDoubleInitRevert<TContract extends BaseContract>(
  config: DoubleInitTestConfig<TContract>,
): Promise<void> {
  if (config.usesCustomError && config.customErrorName) {
    await expectRevertWithCustomError(config.contract, config.initCall, config.customErrorName);
  } else {
    await expectRevertWithReason(config.initCall, INITIALIZATION_ERROR_MESSAGES.ALREADY_INITIALIZED);
  }
}

/**
 * Tests multiple zero value scenarios in batch.
 * Useful for testing all zero value validation in a contract's initialization.
 *
 * @param config - Configuration containing all test scenarios
 *
 * @example
 * ```typescript
 * await expectZeroValueReverts({
 *   contract: myContract,
 *   scenarios: [
 *     {
 *       description: "admin address",
 *       deployOrInitCall: deploy(ZERO_ADDRESS, validVerifier),
 *       expectedError: "ZeroAddressNotAllowed",
 *     },
 *     {
 *       description: "fee amount",
 *       deployOrInitCall: deploy(validAdmin, 0n),
 *       expectedError: "ZeroValueNotAllowed",
 *     },
 *   ],
 * });
 * ```
 */
export async function expectZeroValueReverts<TContract extends BaseContract | ContractFactory>(
  config: ZeroValueBatchConfig<TContract>,
): Promise<void> {
  for (const scenario of config.scenarios) {
    await expectRevertWithCustomError(
      config.contract,
      scenario.deployOrInitCall,
      scenario.expectedError,
      scenario.errorArgs ?? [],
    );
  }
}
