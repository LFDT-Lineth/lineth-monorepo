// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { LineaRollupBase } from "./LineaRollupBase.sol";
import { ILivenessRecovery } from "./interfaces/ILivenessRecovery.sol";
/**
 * @title Contract to manage liveness recovery.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LivenessRecovery is LineaRollupBase, ILivenessRecovery {
  /// @dev In practice, when used, this is expected to be a close approximation to 6 months, and is intentional.
  uint256 internal constant SIX_MONTHS_IN_SECONDS = (365 / 2) * 24 * 60 * 60;

  /// @dev Keep 50 free storage slots for inheriting contracts.
  uint256[50] private __gap_LivenessRecoveryOperator;

  /**
   * @notice Initializes _LivenessRecovery - used for new networks only.
   * @param _livenessRecoveryOperator The initial computed genesis shnarf.
   */
  function __LivenessRecovery_init(address _livenessRecoveryOperator) internal virtual {
    require(_livenessRecoveryOperator != address(0), ZeroAddressNotAllowed());
    livenessRecoveryOperator = _livenessRecoveryOperator;
    emit LivenessRecoveryOperatorAddressSet(msg.sender, _livenessRecoveryOperator);
  }

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
  ) external {
    if (block.timestamp < _lastFinalizedTimestamp + SIX_MONTHS_IN_SECONDS) {
      revert LastFinalizationTimeNotLapsed();
    }
    if (currentFinalizedState != _computeLastFinalizedState(_messageNumber, _rollingHash, _lastFinalizedTimestamp)) {
      revert FinalizationStateIncorrect(
        currentFinalizedState,
        _computeLastFinalizedState(_messageNumber, _rollingHash, _lastFinalizedTimestamp)
      );
    }

    address livenessRecoveryOperatorAddress = livenessRecoveryOperator;

    _grantRole(OPERATOR_ROLE, livenessRecoveryOperatorAddress);
    emit LivenessRecoveryOperatorRoleGranted(msg.sender, livenessRecoveryOperatorAddress);
  }

  /**
   * @notice Revokes `role` from the calling account.
   * @dev Liveness recovery operator cannot renounce role. Reverts with OnlyNonLivenessRecoveryOperator.
   * @param _role The role to renounce.
   * @param _account The account to renounce - can only be the _msgSender().
   */
  function renounceRole(bytes32 _role, address _account) public virtual override {
    if (_account == livenessRecoveryOperator) {
      revert OnlyNonLivenessRecoveryOperator();
    }

    super.renounceRole(_role, _account);
  }
}
