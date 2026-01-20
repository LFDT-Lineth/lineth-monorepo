// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { TokenBridge } from "../../../bridging/token/TokenBridge.sol";

contract MockTokenBridge is TokenBridge {
  function setNativeMappingValue(address token, address value) external {
    nativeToBridgedToken[1][token] = value;
  }
}
