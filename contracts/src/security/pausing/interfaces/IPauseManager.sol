// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.19;

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
    BLOB_SUBMISSION,
    CALLDATA_SUBMISSION,
    FINALIZATION,
    INITIATE_TOKEN_BRIDGING,
    COMPLETE_TOKEN_BRIDGING
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
   * @dev Thrown when a specific pause type is not paused and expected to be.
   */
  error IsNotPaused(PauseType pauseType);

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
   * @dev Requires the role mapped in pauseTypeRoles for the pauseType.
   * @param _pauseType The pause type value.
   */
  function pauseByType(PauseType _pauseType) external;

  /**
   * @notice Unpauses functionality by specific type.
   * @dev Throws if UNUSED pause type is used.
   * @dev Requires the role mapped in unPauseTypeRoles for the pauseType.
   * @param _pauseType The pause type value.
   */
  function unPauseByType(PauseType _pauseType) external;

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
   * @dev PAUSE_ALL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updatePauseTypeRole(PauseType _pauseType, bytes32 _newRole) external;

  /**
   * @notice Update the unpause type role mapping.
   * @dev Throws if UNUSED pause type is used.
   * @dev Throws if role not different.
   * @dev UNPAUSE_ALL_ROLE role is required to execute this function.
   * @param _pauseType The pause type value to update.
   * @param _newRole The role to update to.
   */
  function updateUnpauseTypeRole(PauseType _pauseType, bytes32 _newRole) external;
}
