// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { EfficientLeftRightKeccak } from "../../../libraries/EfficientLeftRightKeccak.sol";

contract TestEfficientLeftRightKeccak {
  function efficientKeccak(bytes32 _left, bytes32 _right) external pure returns (bytes32 value) {
    return EfficientLeftRightKeccak._efficientKeccak(_left, _right);
  }
}
