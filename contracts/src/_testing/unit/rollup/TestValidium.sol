// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { LineaRollupBase } from "../../../rollup/LineaRollupBase.sol";
import { Validium } from "../../../rollup/Validium.sol";
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

  function setupParentShnarf(bytes32 _shnarf) external {
    _blobShnarfExists[_shnarf] = 1;
  }

  function setLastFinalizedBlock(uint256 _blockNumber) external {
    currentL2BlockNumber = _blockNumber;
  }

  function setLastFinalizedShnarf(bytes32 _lastFinalizedShnarf) external {
    currentFinalizedShnarf = _lastFinalizedShnarf;
  }

  function setShnarfFinalBlockNumber(bytes32 _shnarf, uint256 _finalBlockNumber) external {
    _blobShnarfExists[_shnarf] = _finalBlockNumber;
  }

  function setLastFinalizedState(uint256 _messageNumber, bytes32 _rollingHash, uint256 _timestamp) external {
    currentFinalizedState = _computeLastFinalizedState(_messageNumber, _rollingHash, _timestamp);
  }
}
