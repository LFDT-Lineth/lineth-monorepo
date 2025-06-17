// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { PauseManager } from "../../../security/pausing/PauseManager.sol";
import { TestSetPauseTypeRoles } from "./TestSetPauseTypeRoles.sol";

contract TestPauseManager is PauseManager, TestSetPauseTypeRoles {
  function initialize(
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) public initializer {
    __AccessControl_init();
    _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
  }
}
