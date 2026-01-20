// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title Upgradeable Consolidation Queue Predeploy Contract
 * @notice Implementation of EIP-7251 execution layer consolidation request contract
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract UpgradeableConsolidationQueuePredeploy is Initializable {
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
   * @notice Initializes the contract state
   */
  function initialize() external initializer {}

  /**
   * @notice Fallback function - noop
   */
  fallback() external payable {}

  /**
   * @notice Receive function - noop
   */
  receive() external payable {}
}
