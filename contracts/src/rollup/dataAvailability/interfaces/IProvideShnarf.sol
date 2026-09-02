// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.33;

/**
 * @title Interface to define a simple dataRollingHash existence provider definition.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IProvideShnarf {
  /**
   * @notice Returns if the dataRollingHash has been anchored.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _dataRollingHash The dataRollingHash being checked for existence.
   * @return dataRollingHashExists The dataRollingHash's existence value.
   */
  function blobShnarfExists(bytes32 _dataRollingHash) external view returns (uint256 dataRollingHashExists);
}
