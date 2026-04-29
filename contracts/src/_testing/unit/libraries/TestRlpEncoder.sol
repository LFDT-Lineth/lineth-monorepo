// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { RlpEncoder } from "./RlpEncoder.sol";

contract TestRlpEncoder {
  function encodeBool(bool _boolIn) external pure returns (bytes memory encodedBytes) {
    return RlpEncoder._encodeBool(_boolIn);
  }

  function encodeString(string memory _stringIn) external pure returns (bytes memory encodedBytes) {
    return RlpEncoder._encodeString(_stringIn);
  }

  function encodeInt(int _intIn) external pure returns (bytes memory encodedBytes) {
    return RlpEncoder._encodeInt(_intIn);
  }
}
