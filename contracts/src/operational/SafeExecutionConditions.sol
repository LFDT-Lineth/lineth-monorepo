// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { ISafe } from "./interfaces/ISafe.sol";
import { ISafeExecutionConditions } from "./interfaces/ISafeExecutionConditions.sol";

/**
 * @title Stateless precondition checks for multisig Safe transaction execution.
 * @notice Intended to be invoked as the first call of a batched Safe transaction to gate execution on a condition.
 * @dev The contract holds no state. Each function reverts with a dedicated error when its condition is not met,
 * causing the enclosing Safe transaction to revert atomically.
 * @dev The functions are intentionally non-`view`: multisig UIs exclude `view`/`pure` ABI entries when staging
 * batched transactions, so non-`view` mutability is required for these guards to be selectable as a batch call.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract SafeExecutionConditions is ISafeExecutionConditions {
  /**
   * @notice Reverts when the current block timestamp is earlier than `_timestamp`.
   * @param _timestamp The earliest timestamp at which execution is allowed.
   */
  function onlyAfterTimestamp(uint256 _timestamp) external {
    require(block.timestamp >= _timestamp, OnlyOnOrAfter(_timestamp));
  }

  /**
   * @notice Reverts when the transaction origin is not contained in `_executors`.
   * @dev `tx.origin` is used deliberately to authorise the externally owned account that submitted the Safe
   * transaction, rather than the Safe contract itself which would be `msg.sender`.
   * @param _executors The list of addresses allowed to originate the transaction.
   */
  function onlyExecutedBy(address[] calldata _executors) external {
    // solhint-disable-next-line avoid-tx-origin
    require(_contains(_executors, tx.origin), OnlyAllowedExecutor());
  }

  /**
   * @notice Reverts when the transaction origin is not an owner of `_safe`.
   * @dev `_safe` must equal `msg.sender`, i.e. the Safe executing the call. Binding to the caller prevents a
   * submitter from passing the check against a Safe they control rather than the executing Safe.
   * @dev `tx.origin` is used deliberately to authorise the externally owned account that submitted the Safe
   * transaction, rather than the Safe contract itself which would be `msg.sender`.
   * @param _safe The Safe whose ownership is checked against the transaction origin. Must equal `msg.sender`.
   */
  function onlyExecutedBySafeOwner(address _safe) external {
    require(_safe == msg.sender, OnlyExecutingSafe());
    // solhint-disable-next-line avoid-tx-origin
    require(ISafe(_safe).isOwner(tx.origin), OnlySafeOwner());
  }

  /**
   * @notice Returns whether `_target` is present in `_addresses`.
   * @param _addresses The list of addresses to search.
   * @param _target The address to look for.
   * @return found True when `_target` is contained in `_addresses`.
   */
  function _contains(address[] calldata _addresses, address _target) private pure returns (bool found) {
    for (uint256 i; i < _addresses.length; i++) {
      if (_addresses[i] == _target) {
        return true;
      }
    }
    return false;
  }
}
