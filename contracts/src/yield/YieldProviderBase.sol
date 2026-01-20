// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { YieldManagerStorageLayout } from "../yield/YieldManagerStorageLayout.sol";
import { IYieldProvider } from "../yield/interfaces/IYieldProvider.sol";
import { ErrorUtils } from "../libraries/ErrorUtils.sol";

/**
 * @title Base contract YieldProvider adaptor.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract YieldProviderBase is YieldManagerStorageLayout, IYieldProvider {
  /// @notice The YieldManager contract address that will delegatecall the YieldProvider adaptor contract.
  address public immutable YIELD_MANAGER;

  /**
   * @notice Sets immutable variables shared by the YieldManager and YieldProvider adaptor.
   * @param _l1MessageService Address of the L1MessageService contract holding bridge reserves.
   * @param _yieldManager Address of the YieldManager that will `delegatecall` the YieldProvider adaptor.
   */
  constructor(address _l1MessageService, address _yieldManager) {
    ErrorUtils.revertIfZeroAddress(_l1MessageService);
    ErrorUtils.revertIfZeroAddress(_yieldManager);
    L1_MESSAGE_SERVICE = _l1MessageService;
    YIELD_MANAGER = _yieldManager;
  }

  /**
   * @dev Modifier to ensure the function is only invoked via delegatecall from YieldManager.
   */
  modifier onlyDelegateCall() {
    if (address(this) != YIELD_MANAGER) {
      revert ContextIsNotYieldManager();
    }
    _;
  }
}
