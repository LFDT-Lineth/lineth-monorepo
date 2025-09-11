// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.30;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title Upgradeable Fee Vault Contract.
 * @notice Accepts ETH for later economic functions.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract RollupFeeVault is Initializable {
  /**
   * @dev This empty reserved space is put in place to allow future versions to add new
   * variables without shifting down storage in the inheritance chain.
   * See https://docs.openzeppelin.com/contracts/4.x/upgradeable#storage_gaps
   */
  uint256[50] private __gap;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes the contract state.
   */
  function initialize() external initializer {}

  /**
   * @notice Fallback function - Receives Funds.
   */
  fallback() external payable {}

  /**
   * @notice Receive function - Receives Funds.
   */
  receive() external payable {}
}
