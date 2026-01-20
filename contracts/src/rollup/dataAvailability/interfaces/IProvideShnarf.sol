// SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.33;

/**
 * @title Interface to define a simple shnarf providing definition.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IProvideShnarf {
  /**
   * @notice Returns if the shnarf exists.
   * @dev Value > 0 means that it exists. Default is 1.
   * @param _shnarf The shnarf being checked for existence.
   * @return shnarfExists The shnarf's existence value.
   */
  function blobShnarfExists(bytes32 _shnarf) external view returns (uint256 shnarfExists);
}
