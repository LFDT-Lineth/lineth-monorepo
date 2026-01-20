// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import { IVaultFactory } from "../../../yield/interfaces/vendor/lido/IVaultFactory.sol";
import { IPermissionsManager } from "../../../security/access/interfaces/IPermissionsManager.sol";

contract MockVaultFactory is IVaultFactory {
  address public vaultToReturn;
  address public dashboardToReturn;
  bool public shouldRevert;
  error Revert();

  address public lastDefaultAdmin;
  address public lastNodeOperator;
  address public lastNodeOperatorManager;
  uint256 public lastNodeOperatorFeeBP;
  uint256 public lastConfirmExpiry;
  uint256 public lastCallValue;
  IPermissionsManager.RoleAddress[] public lastRoleAssignments;

  function setVaultReturn(address _vault) external {
    vaultToReturn = _vault;
  }

  function setDashboardReturn(address _dashboard) external {
    dashboardToReturn = _dashboard;
  }

  function setReturnValues(address _vault, address _dashboard) external {
    vaultToReturn = _vault;
    dashboardToReturn = _dashboard;
  }

  function setShouldRevert(bool _shouldRevert) external {
    shouldRevert = _shouldRevert;
  }

  function clearLastRoleAssignments() external {
    delete lastRoleAssignments;
  }

  function getLastRoleAssignments() external view returns (IPermissionsManager.RoleAddress[] memory assignments) {
    assignments = new IPermissionsManager.RoleAddress[](lastRoleAssignments.length);
    for (uint256 i = 0; i < lastRoleAssignments.length; ++i) {
      assignments[i] = lastRoleAssignments[i];
    }
  }

  function createVaultWithDashboard(
    address _defaultAdmin,
    address _nodeOperator,
    address _nodeOperatorManager,
    uint256 _nodeOperatorFeeBP,
    uint256 _confirmExpiry,
    IPermissionsManager.RoleAddress[] calldata _roleAssignments
  ) external payable override returns (address vault, address dashboard) {
    if (shouldRevert) {
      revert Revert();
    }

    lastDefaultAdmin = _defaultAdmin;
    lastNodeOperator = _nodeOperator;
    lastNodeOperatorManager = _nodeOperatorManager;
    lastNodeOperatorFeeBP = _nodeOperatorFeeBP;
    lastConfirmExpiry = _confirmExpiry;
    lastCallValue = msg.value;

    delete lastRoleAssignments;
    for (uint256 i = 0; i < _roleAssignments.length; ++i) {
      lastRoleAssignments.push(_roleAssignments[i]);
    }

    return (vaultToReturn, dashboardToReturn);
  }
}
