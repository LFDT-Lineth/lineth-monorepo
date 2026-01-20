// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface declaring pre-existing pausing functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IPauseManager {
  /**
   * @notice Structure defining a pause type and its associated role.
   * @dev This struct is used for both the `_pauseTypeRoles` and `_unPauseTypeRoles` mappings.
   * @param pauseType The type of pause.
   * @param role The role associated with the pause type.
   */
  struct PauseTypeRole {
    PauseType pauseType;
    bytes32 role;
  }

  /**
   * @notice Enum defining the different types of pausing.
   * @dev The pause types are used to pause and unpause specific functionality.
   * @dev The UNUSED pause type is used as a default value to avoid accidental general pausing.
   * @dev Enums are uint8 by default.
   */
  enum PauseType {
    UNUSED,
    GENERAL,
    L1_L2,
    L2_L1,
    /// @dev DEPRECATED
    BLOB_SUBMISSION,
    /// @dev DEPRECATED
    CALLDATA_SUBMISSION,
    FINALIZATION,
    INITIATE_TOKEN_BRIDGING,
    COMPLETE_TOKEN_BRIDGING,
    NATIVE_YIELD_STAKING,
    NATIVE_YIELD_UNSTAKING,
    NATIVE_YIELD_PERMISSIONLESS_ACTIONS,
    NATIVE_YIELD_REPORTING,
    STATE_DATA_SUBMISSION
  }

  /**
   * @notice Emitted when a pause type is paused.
   * @param messageSender The address performing the pause.
   * @param pauseType The indexed pause type that was paused.
   */
  event Paused(address messageSender, PauseType indexed pauseType);

  /**
   * @notice Emitted when a pause type is unpaused.
   * @param messageSender The address performing the unpause.
   * @param pauseType The indexed pause type that was unpaused.
   */
  event UnPaused(address messageSender, PauseType indexed pauseType);

  /**
   * @notice Emitted when a pause type is unpaused due to pause expiry passing.
   * @param pauseType The pause type that was unpaused.
   */
  event UnPausedDueToExpiry(PauseType pauseType);

  /**
   * @notice Emitted when a pause type and its associated role are set in the `_pauseTypeRoles` mapping.
   * @param pauseType The indexed type of pause.
   * @param role The indexed role associated with the pause type.
   */
  event PauseTypeRoleSet(PauseType indexed pauseType, bytes32 indexed role);

  /**
   * @notice Emitted when a pause type and its associated role are updated in the `_PauseTypeRoles` mapping.
   * @param pauseType The indexed type of pause.
   * @param role The indexed role associated with the pause type.
   * @param previousRole The indexed previously found role associated with the pause type.
   */
  event PauseTypeRoleUpdated(PauseType indexed pauseType, bytes32 indexed role, bytes32 indexed previousRole);

  /**
   * @notice Emitted when an unpause type and its associated role are set in the `_unPauseTypeRoles` mapping.
   * @param unPauseType The indexed type of unpause.
   * @param role The indexed role associated with the unpause type.
   */
  event UnPauseTypeRoleSet(PauseType indexed unPauseType, bytes32 indexed role);

  /**
   * @notice Emitted when an unpause type and its associated role are updated in the `_unPauseTypeRoles` mapping.
   * @param unPauseType The indexed type of unpause.
   * @param role The indexed role associated with the unpause type.
   * @param previousRole The indexed previously found role associated with the unpause type.
   */
  event UnPauseTypeRoleUpdated(PauseType indexed unPauseType, bytes32 indexed role, bytes32 indexed previousRole);

  /**
   * @dev Thrown when a specific pause type is paused.
   */
  error IsPaused(PauseType pauseType);

  /**
   * @dev Thrown when unpauseDueToExpiry is attempted before a pause has expired.
   */
  error PauseNotExpired(uint256 expiryEnd);

  /**
   * @dev Thrown when a specific pause type is not paused and expected to be.
   */
  error IsNotPaused(PauseType pauseType);

  /**
   * @dev Thrown when pausing is attempted during the cooldown period by a non-SECURITY_COUNCIL_ROLE.
   */
  error PauseUnavailableDueToCooldown(uint256 cooldownEnd);

  /**
   * @dev Thrown when the unused paused type is used.
   */
  error PauseTypeNotUsed();

  /**
   * @dev Thrown when trying to update a pause/unpause type role mapping to the existing role.
   */
  error RolesNotDifferent();

  /**
   * @notice Pauses functionality by specific type.
   * @dev Throws if UNUSED pause type is used.
   * @dev Requires the role mapped in `_pauseTypeRoles` for the pauseType.
   * @dev Non-SECURITY_COUNCIL_ROLE can only pause after cooldown has passed.
   * @dev SECURITY_COUNCIL_ROLE can pause without cooldown or expiry restrictions.
   * @param _pauseType The pause type value.
   */
  function pauseByType(PauseType _pauseType) external;

  /**
   * @notice Unpauses functionality by specific type.
   * @dev Throws if UNUSED pause type is used.
   * @dev Requires the role mapped in `_unPauseTypeRoles` for the pauseType.
   * @dev SECURITY_COUNCIL_ROLE unpause will reset the cooldown, enabling non-SECURITY_COUNCIL_ROLE pausing.
   * @param _pauseType The pause type value.
   */
  function unPauseByType(PauseType _pauseType) external;

  /**
   * @notice Unpauses a specific pause type when the pause has expired.
   * @dev Can be called by anyone.
   * @dev Throws if UNUSED pause type is used, or the pause expiry period has not passed.
   * @param _pauseType The pause type value.
   */
  function unPauseByExpiredType(PauseType _pauseType) external;

  /**
   * @notice Check if a pause type is enabled.
   * @param _pauseType The pause type value.
   * @return pauseTypeIsPaused Returns true if the pause type if paused, false otherwise.
   */
  function isPaused(PauseType _pauseType) external view returns (bool pauseTypeIsPaused);

  /**
   * @notice Update the pause type role mapping.
   * @dev Throws if UNUSED pause type is used.
   * @dev Throws if role not different.
   * @dev SECURITY_COUNCIL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updatePauseTypeRole(PauseType _pauseType, bytes32 _newRole) external;

  /**
   * @notice Update the unpause type role mapping.
   * @dev Throws if UNUSED pause type is used.
   * @dev Throws if role not different.
   * @dev SECURITY_COUNCIL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updateUnpauseTypeRole(PauseType _pauseType, bytes32 _newRole) external;
}
