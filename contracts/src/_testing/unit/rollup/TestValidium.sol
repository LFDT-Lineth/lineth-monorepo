// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LinethRollupBase } from "../../../rollup/LinethRollupBase.sol";
import { Validium } from "../../../rollup/Validium.sol";
import { FinalizedStateHashing } from "../../../libraries/FinalizedStateHashing.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract TestValidium is Validium {
  function setLivenessRecoveryOperatorAddress(address _livenessRecoveryOperator) external {
    livenessRecoveryOperator = _livenessRecoveryOperator;
  }

  function addRollingHash(uint256 _messageNumber, bytes32 _messageHash) external {
    _addRollingHash(_messageNumber, _messageHash);
  }

  function setRollingHash(uint256 _messageNumber, bytes32 _rollingHash) external {
    rollingHashes[_messageNumber] = _rollingHash;
  }

  function validateL2ComputedRollingHash(uint256 _rollingHashMessageNumber, bytes32 _rollingHash) external view {
    _validateL2ComputedRollingHash(_rollingHashMessageNumber, _rollingHash);
  }

  function setupParentShnarf(bytes32 _dataRollingHash) external {
    _blobShnarfExists[_dataRollingHash] = 1;
  }

  function setLastFinalizedBlock(uint256 _blockNumber) external {
    currentL2BlockNumber = _blockNumber;
  }

  function setLastFinalizedShnarf(bytes32 _lastFinalizedPositionCommitment) external {
    currentFinalizedShnarf = _lastFinalizedPositionCommitment;
  }

  function setShnarfFinalBlockNumber(bytes32 _dataRollingHash, uint256 _value) external {
    _blobShnarfExists[_dataRollingHash] = _value;
  }

  function setLastFinalizedState(uint256 _messageNumber, bytes32 _rollingHash, uint256 _timestamp) external {
    currentFinalizedState = FinalizedStateHashing._computeLastFinalizedState(_messageNumber, _rollingHash, _timestamp);
  }
}
