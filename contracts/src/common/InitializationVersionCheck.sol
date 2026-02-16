// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { Initializable } from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";

/**
 * @title Modifier to guard initialize functions against re-initialization.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract InitializationVersionCheck is Initializable, IGenericErrors {
  /**
   * @dev Reverts if the contract's initialized version does not match the expected version.
   * @param _expectedVersion The exact initialized version required.
   */
  modifier onlyInitializedVersion(uint8 _expectedVersion) {
    if (_getInitializedVersion() != _expectedVersion) {
      revert InitializedVersionWrong(_expectedVersion, _getInitializedVersion());
    }
    _;
  }
}
