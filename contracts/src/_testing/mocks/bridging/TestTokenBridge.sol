// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { TokenBridge } from "../../../bridging/token/TokenBridge.sol";

contract TestTokenBridge is TokenBridge {
  function testReturnDataToString(bytes memory _data) public pure returns (string memory) {
    return _returnDataToString(_data);
  }
}
