// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.24;

import { LineaRollupV5 } from "./LineaRollupV5.sol";

contract TestLineaRollupV5 is LineaRollupV5 {
  function setDefaultShnarfExistValue(bytes32 _shnarf) external {
    shnarfFinalBlockNumbers[_shnarf] = 1;
  }

  function setRollingHash(uint256 _messageNumber, bytes32 _rollingHash) external {
    rollingHashes[_messageNumber] = _rollingHash;
  }
}
