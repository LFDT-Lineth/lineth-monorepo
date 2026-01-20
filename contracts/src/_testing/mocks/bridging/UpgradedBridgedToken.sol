// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { BridgedToken } from "../../../bridging/token/BridgedToken.sol";

contract UpgradedBridgedToken is BridgedToken {
  function isUpgraded() external pure returns (bool) {
    return true;
  }
}
