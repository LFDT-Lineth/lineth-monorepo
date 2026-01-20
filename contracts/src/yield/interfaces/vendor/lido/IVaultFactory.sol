// SPDX-FileCopyrightText: 2025 Lido <info@lido.fi>
// SPDX-License-Identifier: GPL-3.0

pragma solidity >=0.8.0;

import { IPermissionsManager } from "../../../../security/access/interfaces/IPermissionsManager.sol";

interface IVaultFactory {
  function createVaultWithDashboard(
    address _defaultAdmin,
    address _nodeOperator,
    address _nodeOperatorManager,
    uint256 _nodeOperatorFeeBP,
    uint256 _confirmExpiry,
    IPermissionsManager.RoleAddress[] calldata _roleAssignments
  ) external payable returns (address vault, address dashboard);
}
