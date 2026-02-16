// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { TokenBridgeBase } from "./TokenBridgeBase.sol";
import { InitializationVersionCheck } from "../../common/InitializationVersionCheck.sol";

/**
 * @title Linea Canonical Token Bridge
 * @notice Contract to manage cross-chain ERC-20 bridging.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract TokenBridge is InitializationVersionCheck, TokenBridgeBase {
  /// @dev Disable constructor for safety
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes TokenBridge and underlying service dependencies - used for new networks only.
   * @dev Contract will be used as proxy implementation.
   * @param _initializationData The initial data used for initializing the TokenBridge contract.
   */
  function initialize(
    InitializationData calldata _initializationData
  )
    external
    nonZeroAddress(_initializationData.messageService)
    nonZeroAddress(_initializationData.tokenBeacon)
    nonZeroChainId(_initializationData.sourceChainId)
    nonZeroChainId(_initializationData.targetChainId)
    onlyInitializedVersion(0)
    reinitializer(3)
  {
    __TokenBridge_init(_initializationData);
  }

  /**
   * @notice Reinitializes TokenBridge and clears the old reentry slot value.
   */
  function reinitializeV3() external reinitializer(3) {
    uint256 oldReentrancyGuardEntered = 2;
    assembly ("memory-safe") {
      if eq(sload(1), oldReentrancyGuardEntered) {
        mstore(0x00, 0x37ed32e8) //ReentrantCall.selector;
        revert(0x1c, 0x04)
      }
      sstore(1, 0)
    }
  }
}
