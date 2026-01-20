// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title LivenessRecovery interface for current functions, structs, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ILivenessRecovery {
  /**
   * @notice Emitted when the liveness recovery operator role is granted.
   * @param caller The address that called the function granting the role.
   * @param livenessRecoveryOperator The liveness recovery operator address that received the operator role.
   */
  event LivenessRecoveryOperatorRoleGranted(address indexed caller, address indexed livenessRecoveryOperator);

  /**
   * @notice Emitted when the liveness recovery operator role is set on the contract.
   * @param caller The address that set the liveness recovery operator address.
   * @param livenessRecoveryOperator The liveness recovery operator address.
   */
  event LivenessRecoveryOperatorAddressSet(address indexed caller, address indexed livenessRecoveryOperator);

  /**
   * @dev Thrown when the liveness recovery operator tries to renounce their operator role.
   */
  error OnlyNonLivenessRecoveryOperator();

  /**
   * @dev Thrown when the last finalization time has not lapsed when trying to grant the OPERATOR_ROLE to the liveness recovery operator address.
   */
  error LastFinalizationTimeNotLapsed();

  /**
   * @notice Sets the liveness recovery operator role to the specified address if six months have passed since the last finalization.
   * @dev Reverts if six months have not passed since the last finalization.
   * @param _messageNumber Last finalized L1 message number as part of the feedback loop.
   * @param _rollingHash Last finalized L1 rolling hash as part of the feedback loop.
   * @param _lastFinalizedTimestamp Last finalized L2 block timestamp.
   */
  function setLivenessRecoveryOperator(
    uint256 _messageNumber,
    bytes32 _rollingHash,
    uint256 _lastFinalizedTimestamp
  ) external;
}
