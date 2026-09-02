// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { LinethRollupBase } from "../LinethRollupBase.sol";
import { IProvideShnarf } from "./interfaces/IProvideShnarf.sol";

/**
 * @title Contract to manage shared functions for querying dataRollingHash existence.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LocalShnarfProvider is IProvideShnarf, LinethRollupBase {
  /**
   * @notice Returns if the dataRollingHash has been anchored.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _dataRollingHash The dataRollingHash being checked for existence.
   * @return dataRollingHashExists The dataRollingHash's existence value.
   */
  function blobShnarfExists(bytes32 _dataRollingHash) public view returns (uint256 dataRollingHashExists) {
    dataRollingHashExists = _blobShnarfExists[_dataRollingHash];
  }
}
