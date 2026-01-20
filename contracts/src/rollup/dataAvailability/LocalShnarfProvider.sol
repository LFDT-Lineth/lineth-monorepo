// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { LineaRollupBase } from "../LineaRollupBase.sol";
import { IProvideShnarf } from "./interfaces/IProvideShnarf.sol";

/**
 * @title Contract to manage shared functions for querying shnarf existence.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LocalShnarfProvider is IProvideShnarf, LineaRollupBase {
  /**
   * @notice Returns if the shnarf exists.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _shnarf The shnarf being checked for existence.
   * @return shnarfExists The shnarf's existence value.
   */
  function blobShnarfExists(bytes32 _shnarf) public view returns (uint256 shnarfExists) {
    shnarfExists = _blobShnarfExists[_shnarf];
  }
}
