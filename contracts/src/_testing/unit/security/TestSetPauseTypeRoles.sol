// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { PauseManager } from "../../../security/pausing/PauseManager.sol";

abstract contract TestSetPauseTypeRoles is PauseManager {
  function initializePauseTypesAndPermissions(
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external initializer {
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
  }
}
