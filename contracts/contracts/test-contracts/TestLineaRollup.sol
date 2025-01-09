// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.26;

import { LineaRollup } from "../LineaRollup.sol";

contract TestLineaRollup is LineaRollup {
  function addRollingHash(uint256 _messageNumber, bytes32 _messageHash) external {
    _addRollingHash(_messageNumber, _messageHash);
  }

  function setRollingHash(uint256 _messageNumber, bytes32 _rollingHash) external {
    rollingHashes[_messageNumber] = _rollingHash;
  }

  function setLastTimeStamp(uint256 _timestamp) external {
    currentTimestamp = _timestamp;
  }

  function validateL2ComputedRollingHash(uint256 _rollingHashMessageNumber, bytes32 _rollingHash) external view {
    _validateL2ComputedRollingHash(_rollingHashMessageNumber, _rollingHash);
  }

  function calculateY(bytes calldata _data, bytes32 _x) external pure returns (bytes32 y) {
    return _calculateY(_data, _x);
  }

  function setupParentShnarf(bytes32 _shnarf) external {
    blobShnarfExists[_shnarf] = 1;
  }

  function setupParentDataShnarf(bytes32 _parentDataHash, bytes32 _shnarf) external {
    dataShnarfHashes[_parentDataHash] = _shnarf;
  }

  function setLastFinalizedBlock(uint256 _blockNumber) external {
    currentL2BlockNumber = _blockNumber;
  }

  function setupParentFinalizedStateRoot(bytes32 _parentDataHash, bytes32 _blockStateRoot) external {
    dataFinalStateRootHashes[_parentDataHash] = _blockStateRoot;
  }

  function setupStartingBlockForDataHash(bytes32 _dataHash, uint256 _blockNumber) external {
    dataStartingBlock[_dataHash] = _blockNumber;
  }

  function setLastFinalizedShnarf(bytes32 _lastFinalizedShnarf) external {
    currentFinalizedShnarf = _lastFinalizedShnarf;
  }

  function setShnarfFinalBlockNumber(bytes32 _shnarf, uint256 _finalBlockNumber) external {
    blobShnarfExists[_shnarf] = _finalBlockNumber;
  }

  function setLastFinalizedState(uint256 _messageNumber, bytes32 _rollingHash, uint256 _timestamp) external {
    currentFinalizedState = _computeLastFinalizedState(_messageNumber, _rollingHash, _timestamp);
  }
}
