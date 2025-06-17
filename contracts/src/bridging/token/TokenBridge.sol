// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.30;

import { TokenBridgeBase } from "./TokenBridgeBase.sol";

/**
 * @title Linea Canonical Token Bridge
 * @notice Contract to manage cross-chain ERC-20 bridging.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract TokenBridge is TokenBridgeBase {
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
    initializer
  {
    __TokenBridge_init(_initializationData);
  }
}
