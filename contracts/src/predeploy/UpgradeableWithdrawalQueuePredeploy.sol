// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.30;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

/**
 * @title Upgradeable Withdrawal Queue Predeploy Contract
 * @notice Implementation of EIP-7002 execution layer triggerable withdrawals
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract UpgradeableWithdrawalQueuePredeploy is Initializable {
  error NotImplemented();

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
  function initialize() external initializer {
  }

  /**
   * @notice Fallback function - reverts to prevent accidental calls
   */
  fallback() external payable {
    revert NotImplemented();
  }

  /**
   * @notice Receive function - reverts to prevent accidental ETH transfers
   */
  receive() external payable {
    revert NotImplemented();
  }
}