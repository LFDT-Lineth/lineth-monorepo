// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.19;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { IPauseManager } from "./interfaces/IPauseManager.sol";

/**
 * @title Contract to manage cross-chain function pausing with limited duration and cooldown mechanic.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract PauseManager is IPauseManager, AccessControlUpgradeable {
  /// @notice This is used to pause all pausable functions.
  bytes32 public constant PAUSE_ALL_ROLE = keccak256("PAUSE_ALL_ROLE");

  /// @notice This is used to unpause all unpausable functions.
  bytes32 public constant UNPAUSE_ALL_ROLE = keccak256("UNPAUSE_ALL_ROLE");

  /// @notice Role assigned to the security council that enables indefinite pausing and bypassing the cooldown period.
  /// @dev Is not a pause or unpause role; a specific pause/unpause role is still required for specific pause/unpause types.
  bytes32 public constant SECURITY_COUNCIL_ROLE = keccak256("SECURITY_COUNCIL_ROLE");

  /// @notice Duration of pauses, after which pauses will expire (except by the SECURITY_COUNCIL_ROLE).
  uint256 public constant PAUSE_DURATION = 72 hours;

  /// @notice Duration of cooldown after a pause expires, during which no pauses (except by the SECURITY_COUNCIL_ROLE) can be enacted.
  /// @dev This prevents indefinite pause chaining by a non-SECURITY_COUNCIL_ROLE.
  uint256 public constant COOLDOWN_DURATION = 24 hours;

  // @dev DEPRECATED. USE _pauseTypeStatusesBitMap INSTEAD
  mapping(bytes32 pauseType => bool pauseStatus) private pauseTypeStatuses_DEPRECATED;

  /// @dev The bitmap containing the pause statuses mapped by type.
  uint256 private _pauseTypeStatusesBitMap;

  /// @dev This maps the pause type to the role that is allowed to pause it.
  mapping(PauseType pauseType => bytes32 role) private _pauseTypeRoles;

  /// @dev This maps the unpause type to the role that is allowed to unpause it.
  mapping(PauseType unPauseType => bytes32 role) private _unPauseTypeRoles;

  /// @notice Unix timestamp of pause expiry.
  /// @dev pauseExpiryTimestamp applies to all pause types. Pausing with one pause type blocks other pause types from being enacted (unless the SECURITY_COUNCIL_ROLE is used).
  /// @dev This prevents indefinite pause chaining by a non-SECURITY_COUNCIL_ROLE.
  uint256 public pauseExpiryTimestamp;

  /// @dev Total contract storage is 12 slots with the gap below.
  /// @dev Keep 6 free storage slots for future implementation updates to avoid storage collision.
  /// @dev Note: This was reduced previously to cater for new functionality.
  uint256[6] private __gap;

  /**
   * @dev Modifier to prevent usage of unused PauseType.
   * @param _pauseType The PauseType value being checked.
   * Requirements:
   *
   * - The type must not be UNUSED.
   */
  modifier onlyUsedPausedTypes(PauseType _pauseType) {
    if (_pauseType == PauseType.UNUSED) {
      revert PauseTypeNotUsed();
    }
    _;
  }

  /**
   * @dev Modifier to make a function callable only when the specific and general types are not paused.
   * @param _pauseType The pause type value being checked.
   * Requirements:
   *
   * - The type must not be paused.
   */
  modifier whenTypeAndGeneralNotPaused(PauseType _pauseType) {
    _requireTypeAndGeneralNotPaused(_pauseType);
    _;
  }

  /**
   * @dev Modifier to make a function callable only when the type is not paused.
   * @param _pauseType The pause type value being checked.
   * Requirements:
   *
   * - The type must not be paused.
   */
  modifier whenTypeNotPaused(PauseType _pauseType) {
    _requireTypeNotPaused(_pauseType);
    _;
  }

  /**
   * @notice Initializes the pause manager with the given pause and unpause roles.
   * @dev This function is called during contract initialization to set up the pause and unpause roles.
   * @param _pauseTypeRoleAssignments An array of PauseTypeRole structs defining the pause types and their associated roles.
   * @param _unpauseTypeRoleAssignments An array of PauseTypeRole structs defining the unpause types and their associated roles.
   */
  function __PauseManager_init(
    PauseTypeRole[] calldata _pauseTypeRoleAssignments,
    PauseTypeRole[] calldata _unpauseTypeRoleAssignments
  ) internal onlyInitializing {
    for (uint256 i; i < _pauseTypeRoleAssignments.length; i++) {
      _pauseTypeRoles[_pauseTypeRoleAssignments[i].pauseType] = _pauseTypeRoleAssignments[i].role;
      emit PauseTypeRoleSet(_pauseTypeRoleAssignments[i].pauseType, _pauseTypeRoleAssignments[i].role);
    }

    for (uint256 i; i < _unpauseTypeRoleAssignments.length; i++) {
      _unPauseTypeRoles[_unpauseTypeRoleAssignments[i].pauseType] = _unpauseTypeRoleAssignments[i].role;
      emit UnPauseTypeRoleSet(_unpauseTypeRoleAssignments[i].pauseType, _unpauseTypeRoleAssignments[i].role);
    }
  }

  /**
   * @dev Throws if the specific or general types are paused.
   * @dev Checks the specific and general pause types.
   * @param _pauseType The pause type value being checked.
   */
  function _requireTypeAndGeneralNotPaused(PauseType _pauseType) internal view virtual {
    uint256 pauseBitMap = _pauseTypeStatusesBitMap;

    if (pauseBitMap & (1 << uint256(_pauseType)) != 0) {
      revert IsPaused(_pauseType);
    }

    if (pauseBitMap & (1 << uint256(PauseType.GENERAL)) != 0) {
      revert IsPaused(PauseType.GENERAL);
    }
  }

  /**
   * @dev Throws if the type is paused.
   * @dev Checks the specific pause type.
   * @param _pauseType The pause type value being checked.
   */
  function _requireTypeNotPaused(PauseType _pauseType) internal view virtual {
    if (isPaused(_pauseType)) {
      revert IsPaused(_pauseType);
    }
  }

  /**
   * @notice Pauses functionality by specific type.
   * @dev Throws if UNUSED pause type is used.
   * @dev Requires the role mapped in `_pauseTypeRoles` for the pauseType.
   * @dev Non-SECURITY_COUNCIL_ROLE can only pause after cooldown has passed.
   * @dev SECURITY_COUNCIL_ROLE can pause without cooldown or expiry restrictions.
   * @param _pauseType The pause type value.
   */
  function pauseByType(
    PauseType _pauseType
  ) external onlyUsedPausedTypes(_pauseType) onlyRole(_pauseTypeRoles[_pauseType]) {
    if (isPaused(_pauseType)) {
      revert IsPaused(_pauseType);
    }
    
    unchecked {
      if (hasRole(SECURITY_COUNCIL_ROLE, _msgSender())) {
        pauseExpiryTimestamp = type(uint256).max - COOLDOWN_DURATION;
      } else {
        if (block.timestamp < pauseExpiryTimestamp + COOLDOWN_DURATION) {
          revert PauseUnavailableDueToCooldown(pauseExpiryTimestamp + COOLDOWN_DURATION);
        }
        pauseExpiryTimestamp = block.timestamp + PAUSE_DURATION;
      }
    }

    _pauseTypeStatusesBitMap |= 1 << uint256(_pauseType);
    emit Paused(_msgSender(), _pauseType);
  }

  /**
   * @notice Unpauses functionality by specific type.
   * @dev Throws if UNUSED pause type is used.
   * @dev Requires the role mapped in `_unPauseTypeRoles` for the pauseType.
   * @dev SECURITY_COUNCIL_ROLE unpause will reset the cooldown, enabling non-SECURITY_COUNCIL_ROLE pausing.
   * @param _pauseType The pause type value.
   */
  function unPauseByType(
    PauseType _pauseType
  ) external onlyUsedPausedTypes(_pauseType) onlyRole(_unPauseTypeRoles[_pauseType]) {
    if (!isPaused(_pauseType)) {
      revert IsNotPaused(_pauseType);
    }

    if (hasRole(SECURITY_COUNCIL_ROLE, _msgSender())) {
      pauseExpiryTimestamp = block.timestamp - COOLDOWN_DURATION;
    }
    _pauseTypeStatusesBitMap &= ~(1 << uint256(_pauseType));
    emit UnPaused(_msgSender(), _pauseType);
  }

  /**
   * @notice Unpauses a specific pause type when the pause has expired.
   * @dev Can be called by anyone.
   * @dev Throws if UNUSED pause type is used, or the pause expiry period has not passed.
   * @param _pauseType The pause type value.
   */
  function unPauseByExpiredType(
    PauseType _pauseType
  ) external onlyUsedPausedTypes(_pauseType) {
    if (!isPaused(_pauseType)) {
      revert IsNotPaused(_pauseType);
    }
    if (block.timestamp < pauseExpiryTimestamp) {
      revert PauseNotExpired(pauseExpiryTimestamp);
    }

    _pauseTypeStatusesBitMap &= ~(1 << uint256(_pauseType));
    emit UnPausedDueToExpiry(_pauseType);
  }

  /**
   * @notice Check if a pause type is enabled.
   * @param _pauseType The pause type value.
   * @return pauseTypeIsPaused Returns true if the pause type if paused, false otherwise.
   */
  function isPaused(PauseType _pauseType) public view returns (bool pauseTypeIsPaused) {
    pauseTypeIsPaused = (_pauseTypeStatusesBitMap & (1 << uint256(_pauseType))) != 0;
  }

  /**
   * @notice Update the pause type role mapping.
   * @dev Throws if UNUSED pause type is used.
   * @dev Throws if role not different.
   * @dev SECURITY_COUNCIL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updatePauseTypeRole(
    PauseType _pauseType,
    bytes32 _newRole
  ) external onlyUsedPausedTypes(_pauseType) onlyRole(SECURITY_COUNCIL_ROLE) {
    bytes32 previousRole = _pauseTypeRoles[_pauseType];
    if (previousRole == _newRole) {
      revert RolesNotDifferent();
    }

    _pauseTypeRoles[_pauseType] = _newRole;
    emit PauseTypeRoleUpdated(_pauseType, _newRole, previousRole);
  }

  /**
   * @notice Update the unpause type role mapping.
   * @dev Throws if UNUSED pause type is used.
   * @dev Throws if role not different.
   * @dev SECURITY_COUNCIL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updateUnpauseTypeRole(
    PauseType _pauseType,
    bytes32 _newRole
  ) external onlyUsedPausedTypes(_pauseType) onlyRole(SECURITY_COUNCIL_ROLE) {
    bytes32 previousRole = _unPauseTypeRoles[_pauseType];
    if (previousRole == _newRole) {
      revert RolesNotDifferent();
    }

    _unPauseTypeRoles[_pauseType] = _newRole;
    emit UnPauseTypeRoleUpdated(_pauseType, _newRole, previousRole);
  }
}
