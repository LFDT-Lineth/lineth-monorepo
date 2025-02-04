// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { IGenericErrors } from "../../interfaces/IGenericErrors.sol";
import { IPermissionsManager } from "./interfaces/IPermissionsManager.sol";

/**
 * @title Contract to manage permissions initialization.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract PermissionsManager is AccessControlUpgradeable, IPermissionsManager, IGenericErrors {
  /**
   * @notice Sets permissions for a list of addresses and their roles.
   * @param _roleAddresses The list of addresses and roles to assign permissions to.
   */
  function __Permissions_init(RoleAddress[] calldata _roleAddresses) internal onlyInitializing {
    for (uint256 i; i < _roleAddresses.length; i++) {
      if (_roleAddresses[i].addressWithRole == address(0)) {
        revert ZeroAddressNotAllowed();
      }

      if (_roleAddresses[i].role == 0x0) {
        revert ZeroHashNotAllowed();
      }

      _grantRole(_roleAddresses[i].role, _roleAddresses[i].addressWithRole);
    }
  }
}
