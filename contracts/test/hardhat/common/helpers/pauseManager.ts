import { expect } from "chai";

import { expectEvent, expectRevertWithCustomError } from "./expectations.js";

import type { Abi, Account, Address, Hash } from "viem";

/**
 * Interface for contracts that implement PauseManager
 */
export interface PauseManagerContract {
  abi: Abi;
  address: Address;
  read: {
    isPaused(args: readonly [number]): Promise<boolean>;
    pauseTypeExpiryTimestamps(args: readonly [number]): Promise<bigint>;
    PAUSE_DURATION(): Promise<bigint>;
    COOLDOWN_DURATION(): Promise<bigint>;
  };
  write: {
    pauseByType(args: readonly [number], options?: { account?: Account }): Promise<Hash>;
    unPauseByType(args: readonly [number], options?: { account?: Account }): Promise<Hash>;
  };
}

/**
 * Verifies that a specific pause type is paused
 * @param contract - Contract with PauseManager
 * @param pauseType - Pause type to check
 */
export async function expectPaused(contract: PauseManagerContract, pauseType: number): Promise<void> {
  const isPaused = await contract.read.isPaused([pauseType]);
  expect(isPaused).to.be.true;
}

/**
 * Verifies that a specific pause type is not paused
 * @param contract - Contract with PauseManager
 * @param pauseType - Pause type to check
 */
export async function expectNotPaused(contract: PauseManagerContract, pauseType: number): Promise<void> {
  const isPaused = await contract.read.isPaused([pauseType]);
  expect(isPaused).to.be.false;
}

/**
 * Expects a pause event to be emitted
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause transaction
 * @param pauserAddress - Address of the account performing the pause
 * @param pauseType - Pause type being activated
 */
export async function expectPauseEvent(
  contract: PauseManagerContract,
  asyncCall: Promise<Hash>,
  pauserAddress: Address,
  pauseType: number,
): Promise<void> {
  await expectEvent(contract, asyncCall, "Paused", [pauserAddress, pauseType]);
}

/**
 * Expects an unpause event to be emitted
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the unpause transaction
 * @param unpauser - Address of the account performing the unpause
 * @param pauseType - Pause type being deactivated
 */
export async function expectUnpauseEvent(
  contract: PauseManagerContract,
  asyncCall: Promise<Hash>,
  unpauser: Address,
  pauseType: number,
): Promise<void> {
  await expectEvent(contract, asyncCall, "UnPaused", [unpauser, pauseType]);
}

/**
 * Expects an unpause due to expiry event to be emitted
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the unpause transaction
 * @param pauseType - Pause type being deactivated
 */
export async function expectUnpauseExpiryEvent(
  contract: PauseManagerContract,
  asyncCall: Promise<Hash>,
  pauseType: number,
): Promise<void> {
  await expectEvent(contract, asyncCall, "UnPausedDueToExpiry", [pauseType]);
}

/**
 * Expects a transaction to revert because the contract is paused
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the transaction that should revert
 * @param pauseType - Pause type that is active (for error message)
 */
export async function expectRevertWhenPaused(
  contract: PauseManagerContract,
  asyncCall: Promise<unknown>,
  pauseType: number,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "IsPaused", [pauseType]);
}

/**
 * Expects a transaction to revert because the contract is not paused
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the transaction that should revert
 * @param pauseType - Pause type that should be active
 */
export async function expectRevertWhenNotPaused(
  contract: PauseManagerContract,
  asyncCall: Promise<unknown>,
  pauseType: number,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "IsNotPaused", [pauseType]);
}

/**
 * Expects a pause attempt to revert because pause type is not used
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause/unpause transaction
 */
export async function expectPauseTypeNotUsedRevert(
  contract: PauseManagerContract,
  asyncCall: Promise<unknown>,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "PauseTypeNotUsed");
}

/**
 * Expects a pause attempt to revert due to cooldown
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause transaction
 * @param expectedCooldownEnd - Expected cooldown end timestamp
 */
export async function expectCooldownRevert(
  contract: PauseManagerContract,
  asyncCall: Promise<unknown>,
  expectedCooldownEnd: bigint,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "PauseUnavailableDueToCooldown", [expectedCooldownEnd]);
}

/**
 * Expects an unpause by expiry to revert because pause has not expired
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the unpause transaction
 * @param pauseExpiryTimestamp - Expected pause expiry timestamp
 */
export async function expectPauseNotExpiredRevert(
  contract: PauseManagerContract,
  asyncCall: Promise<unknown>,
  pauseExpiryTimestamp: bigint,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "PauseNotExpired", [pauseExpiryTimestamp]);
}
