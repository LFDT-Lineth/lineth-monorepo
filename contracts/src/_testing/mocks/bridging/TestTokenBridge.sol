// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { TokenBridge } from "../../../bridging/token/TokenBridge.sol";

/// @custom:oz-upgrades-unsafe-allow missing-initializer
contract TestTokenBridge is TokenBridge {
  function testReturnDataToString(bytes memory _data) public pure returns (string memory) {
    return _returnDataToString(_data);
  }

  function setSlotValue(uint256 _slot, uint256 _value) external {
    assembly {
      sstore(_slot, _value)
    }
  }

  function getSlotValue(uint256 _slot) external view returns (uint256 slotValue) {
    assembly {
      slotValue := sload(_slot)
    }
  }
}
