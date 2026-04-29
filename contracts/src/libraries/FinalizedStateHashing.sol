// SPDX-License-Identifier: Apache-2.0

/**
 * @title Library for computing the last finalized state on L1.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
pragma solidity ^0.8.33;

library FinalizedStateHashing {
  /**
   * @notice Internal function to compute and save the finalization state.
   * @dev Using assembly this way is cheaper gas wise.
   * @param _messageNumber Is the last L2 computed L1 message number in the finalization.
   * @param _messageRollingHash Is the last L2 computed L1 rolling hash in the finalization.
   * @param _forcedTransactionNumber Is the last processed forced transaction on L2's number.
   * @param _forcedTransactionRollingHash Is the last processed forced transaction on L2's rolling hash.
   * @param _timestamp The final timestamp in the finalization.
   * @return hashedFinalizationState The hashed finalization state.
   */
  function _computeLastFinalizedState(
    uint256 _messageNumber,
    bytes32 _messageRollingHash,
    uint256 _forcedTransactionNumber,
    bytes32 _forcedTransactionRollingHash,
    uint256 _timestamp
  ) internal pure returns (bytes32 hashedFinalizationState) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _messageNumber)
      mstore(add(mPtr, 0x20), _messageRollingHash)
      mstore(add(mPtr, 0x40), _forcedTransactionNumber)
      mstore(add(mPtr, 0x60), _forcedTransactionRollingHash)
      mstore(add(mPtr, 0x80), _timestamp)
      hashedFinalizationState := keccak256(mPtr, 0xa0)
    }
  }

  /**
   * @notice Internal function to compute and save the finalization state.
   * @dev Using assembly this way is cheaper gas wise.
   * @param _messageNumber Is the last L2 computed L1 message number in the finalization.
   * @param _rollingHash Is the last L2 computed L1 rolling hash in the finalization.
   * @param _timestamp The final timestamp in the finalization.
   * @return hashedFinalizationState The hashed finalization state.
   */
  function _computeLastFinalizedState(
    uint256 _messageNumber,
    bytes32 _rollingHash,
    uint256 _timestamp
  ) internal pure returns (bytes32 hashedFinalizationState) {
    assembly {
      let mPtr := mload(0x40)
      mstore(mPtr, _messageNumber)
      mstore(add(mPtr, 0x20), _rollingHash)
      mstore(add(mPtr, 0x40), _timestamp)
      hashedFinalizationState := keccak256(mPtr, 0x60)
    }
  }
}
