import { expectRevertWithCustomError } from "./expectations.js";

import type { Abi } from "viem";

/** Arguments of the zero-value revert helpers; `expectedError` defaults to the helper's standard error. */
type ZeroValueTestConfig = {
  /** Any contract or artifact whose ABI declares the error */
  contract: { abi: Abi };
  /** The async call that should revert */
  deployOrInitCall: Promise<unknown>;
  expectedError?: string;
  errorArgs?: unknown[];
};

/** Asserts that a deployment or initialization reverts with `ZeroAddressNotAllowed` (or `expectedError`). */
export async function expectZeroAddressRevert(config: ZeroValueTestConfig): Promise<void> {
  await expectRevertWithCustomError(
    config.contract,
    config.deployOrInitCall,
    config.expectedError ?? "ZeroAddressNotAllowed",
    config.errorArgs,
  );
}

/** Asserts that a deployment or initialization reverts with `ZeroHashNotAllowed` (or `expectedError`). */
export async function expectZeroHashRevert(config: ZeroValueTestConfig): Promise<void> {
  await expectRevertWithCustomError(
    config.contract,
    config.deployOrInitCall,
    config.expectedError ?? "ZeroHashNotAllowed",
    config.errorArgs,
  );
}
