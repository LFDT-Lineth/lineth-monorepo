import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { expect } from "chai";
import { TestPauseManager } from "contracts/typechain-types";
import { BaseContract } from "ethers";

import { expectEvent, expectRevertWithCustomError, expectRevertWithReason } from "./expectations";
import { buildAccessErrorMessage } from "./general";

/**
 * Interface for contracts that implement PauseManager
 */
export interface PauseManagerContract extends BaseContract {
  isPaused(pauseType: number): Promise<boolean>;
  pauseByType(pauseType: number): Promise<unknown>;
  unPauseByType(pauseType: number): Promise<unknown>;
  pauseTypeExpiryTimestamps(pauseType: number): Promise<bigint>;
  PAUSE_DURATION(): Promise<bigint>;
  COOLDOWN_DURATION(): Promise<bigint>;
}

/**
 * Interface for contracts that support expired pause unpause
 */
export interface PauseManagerWithExpiryContract extends PauseManagerContract {
  unPauseByExpiredType(pauseType: number): Promise<unknown>;
}

/**
 * Configuration for pause type role mapping
 */
export interface PauseTypeRoleConfig {
  pauseType: number;
  pauseRole: string;
  unpauseRole: string;
}

/**
 * Verifies that a specific pause type is paused
 * @param contract - Contract with PauseManager
 * @param pauseType - Pause type to check
 */
export async function expectPaused<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
): Promise<void> {
  const isPaused = await contract.isPaused(pauseType);
  expect(isPaused).to.be.true;
}

/**
 * Verifies that a specific pause type is not paused
 * @param contract - Contract with PauseManager
 * @param pauseType - Pause type to check
 */
export async function expectNotPaused<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
): Promise<void> {
  const isPaused = await contract.isPaused(pauseType);
  expect(isPaused).to.be.false;
}

/**
 * Pauses a contract and verifies the pause was successful
 * @param contract - Contract with PauseManager (connected to authorized account)
 * @param pauseType - Pause type to activate
 */
export async function pauseAndVerify<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
): Promise<void> {
  await contract.pauseByType(pauseType);
  await expectPaused(contract, pauseType);
}

/**
 * Unpauses a contract and verifies the unpause was successful
 * @param contract - Contract with PauseManager (connected to authorized account)
 * @param pauseType - Pause type to deactivate
 */
export async function unpauseAndVerify<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
): Promise<void> {
  await contract.unPauseByType(pauseType);
  await expectNotPaused(contract, pauseType);
}

/**
 * Expects a pause event to be emitted
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause transaction
 * @param pauserAddress - Address of the account performing the pause
 * @param pauseType - Pause type being activated
 */
export async function expectPauseEvent<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  asyncCall: Promise<unknown>,
  pauserAddress: string,
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
export async function expectUnpauseEvent<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  asyncCall: Promise<unknown>,
  unpauser: string,
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
export async function expectUnpauseExpiryEvent<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  asyncCall: Promise<unknown>,
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
export async function expectRevertWhenPaused<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
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
export async function expectRevertWhenNotPaused<T extends PauseManagerContract>(
  contract: T,
  asyncCall: Promise<unknown>,
  pauseType: number,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "IsNotPaused", [pauseType]);
}

/**
 * Expects a pause attempt to revert due to missing pause role
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause transaction
 * @param account - Account attempting the pause
 * @param requiredRole - Required pause role
 */
export async function expectPauseRoleRevert<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  asyncCall: Promise<unknown>,
  account: SignerWithAddress,
  requiredRole: string,
): Promise<void> {
  await expectRevertWithReason(asyncCall, buildAccessErrorMessage(account, requiredRole));
}

/**
 * Expects an unpause attempt to revert due to missing unpause role
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the unpause transaction
 * @param account - Account attempting the unpause
 * @param requiredRole - Required unpause role
 */
export async function expectUnpauseRoleRevert<T extends PauseManagerContract>(
  contract: T,
  asyncCall: Promise<unknown>,
  account: SignerWithAddress,
  requiredRole: string,
): Promise<void> {
  await expectRevertWithReason(asyncCall, buildAccessErrorMessage(account, requiredRole));
}

/**
 * Expects a pause attempt to revert because pause type is not used
 * @param contract - Contract with PauseManager
 * @param asyncCall - Promise of the pause/unpause transaction
 */
export async function expectPauseTypeNotUsedRevert<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
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
export async function expectCooldownRevert<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
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
export async function expectPauseNotExpiredRevert<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  asyncCall: Promise<unknown>,
  pauseExpiryTimestamp: bigint,
): Promise<void> {
  await expectRevertWithCustomError(contract, asyncCall, "PauseNotExpired", [pauseExpiryTimestamp]);
}

/**
 * Tests that an action fails when a specific pause type is active
 * @param contract - Contract with PauseManager
 * @param pauser - Account with pause permissions (connected contract)
 * @param pauseType - Pause type to test
 * @param actionFn - Function that performs the paused action
 */
export async function testActionFailsWhenPaused<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauser: T,
  pauseType: number,
  actionFn: () => Promise<unknown>,
): Promise<void> {
  // Pause the contract
  await pauser.pauseByType(pauseType);
  await expectPaused(contract, pauseType);

  // Verify action fails
  await expectRevertWhenPaused(contract, actionFn(), pauseType);
}

/**
 * Tests the full pause/unpause cycle for a pause type
 * @param contract - Contract with PauseManager (connected to authorized account)
 * @param pauseType - Pause type to test
 * @param pauser - Address performing the pause/unpause
 */
export async function testPauseUnpauseCycle<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
  pauser: string,
): Promise<void> {
  // Verify not paused initially
  await expectNotPaused(contract, pauseType);

  // Pause and verify event
  await expectPauseEvent(contract, contract.pauseByType(pauseType), pauser, pauseType);
  await expectPaused(contract, pauseType);

  // Unpause and verify event
  await expectUnpauseEvent(contract, contract.unPauseByType(pauseType), pauser, pauseType);
  await expectNotPaused(contract, pauseType);
}

/**
 * Calculates the expected cooldown end timestamp for a given pause type
 * @param contract - Contract with PauseManager
 * @param pauseType - Pause type to check
 * @returns Expected cooldown end timestamp
 */
export async function getExpectedCooldownEnd<T extends PauseManagerContract | TestPauseManager>(
  contract: T,
  pauseType: number,
): Promise<bigint> {
  const expiryTimestamp = await contract.pauseTypeExpiryTimestamps(pauseType);
  const cooldownDuration = await contract.COOLDOWN_DURATION();
  return expiryTimestamp + cooldownDuration;
}

/**
 * Configuration for pause type testing
 */
export interface PauseTypeTestConfig {
  pauseType: number;
  name: string;
}

/**
 * Tests that an action reverts for multiple pause types.
 * Creates parameterized test cases for each pause type.
 * Use within a describe block to generate individual tests.
 *
 * @example
 * ```typescript
 * describePauseTypeTests(
 *   [
 *     { pauseType: GENERAL_PAUSE_TYPE, name: "GENERAL_PAUSE_TYPE" },
 *     { pauseType: FINALIZATION_PAUSE_TYPE, name: "FINALIZATION_PAUSE_TYPE" },
 *   ],
 *   () => lineaRollup,
 *   () => lineaRollup.connect(securityCouncil),
 *   async () => lineaRollup.connect(operator).finalizeBlocks(proof, 0, finalizationData),
 * );
 * ```
 *
 * @param pauseTypes - Array of pause types with their names
 * @param getContract - Function returning the contract to test
 * @param getPauser - Function returning the contract connected to a pausing account
 * @param getActionFn - Function returning the action to test
 */
export function describePauseTypeTests<T extends PauseManagerContract | TestPauseManager>(
  pauseTypes: PauseTypeTestConfig[],
  getContract: () => T | Promise<T>,
  getPauser: () => T | Promise<T>,
  getActionFn: () => Promise<unknown>,
): void {
  pauseTypes.forEach(({ pauseType, name }) => {
    it(`Should revert if ${name} is enabled`, async () => {
      const contract = await getContract();
      const pauser = await getPauser();

      await pauser.pauseByType(pauseType);
      await expectRevertWhenPaused(contract, getActionFn(), pauseType);
    });
  });
}

/**
 * Generates pause type test configurations for common pause types.
 * @param types - Object mapping pause type names to their values
 */
export function createPauseTypeConfigs(types: Record<string, number>): PauseTypeTestConfig[] {
  return Object.entries(types).map(([name, pauseType]) => ({ pauseType, name }));
}
