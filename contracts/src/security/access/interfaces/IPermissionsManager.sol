// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface declaring permissions manager related data types.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IPermissionsManager {
  /**
   * @notice Structure defining a role and its associated address.
   * @param addressWithRole The address with the role.
   * @param role The role associated with the address.
   */
  struct RoleAddress {
    address addressWithRole;
    bytes32 role;
  }
}
