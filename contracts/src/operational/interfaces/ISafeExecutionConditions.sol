// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity ^0.8.33;

/**
 * @title Interface for the stateless Safe execution conditions helper.
 * @notice Declares the precondition checks intended to be invoked as the first call of a multisig Safe transaction.
 * @dev The functions are intentionally non-`view` even though they do not mutate state. Multisig UIs (e.g.
 * Safe{Wallet} Transaction Builder) classify `view`/`pure` ABI entries as read-only calls and exclude them when
 * staging batched transactions, so a non-`view` mutability is required for these guards to be usable as a batch call.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ISafeExecutionConditions {
  /**
   * @notice Thrown when the call is executed before the required timestamp.
   * @param timestamp The earliest timestamp at which execution is allowed.
   */
  error OnlyOnOrAfter(uint256 timestamp);

  /**
   * @notice Thrown when the transaction origin is not part of the allowed executors.
   */
  error OnlyAllowedExecutor();

  /**
   * @notice Thrown when the transaction origin is not an owner of the provided Safe.
   */
  error OnlySafeOwner();

  /**
   * @notice Thrown when the provided Safe is not the Safe executing the call.
   */
  error OnlyExecutingSafe();

  /**
   * @notice Reverts when the current block timestamp is earlier than `_timestamp`.
   * @param _timestamp The earliest timestamp at which execution is allowed.
   */
  function onlyAfterTimestamp(uint256 _timestamp) external;

  /**
   * @notice Reverts when the transaction origin is not contained in `_executors`.
   * @param _executors The list of addresses allowed to originate the transaction.
   */
  function onlyExecutedBy(address[] calldata _executors) external;

  /**
   * @notice Reverts when the transaction origin is not an owner of `_safe`.
   * @dev `_safe` must be the Safe executing the call (`msg.sender`); supplying any other address reverts. This
   * prevents a submitter from satisfying the check against a Safe they control instead of the executing Safe.
   * @param _safe The Safe whose ownership is checked against the transaction origin. Must equal `msg.sender`.
   */
  function onlyExecutedBySafeOwner(address _safe) external;
}
