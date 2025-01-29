// SPDX-License-Identifier: AGPL-3.0
pragma solidity >=0.8.19 <=0.8.26;

import { PauseManager } from "./PauseManager.sol";

/**
 * @title Contract to manage cross-chain function pausing roles for the LineaRollup.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract LineaRollupPauseManager is PauseManager {
  /// @notice This is used to pause L1 to L2 communication.
  bytes32 public constant PAUSE_L1_L2_ROLE = keccak256("PAUSE_L1_L2_ROLE");

  /// @notice This is used to unpause L1 to L2 communication.
  bytes32 public constant UNPAUSE_L1_L2_ROLE = keccak256("UNPAUSE_L1_L2_ROLE");

  /// @notice This is used to pause L2 to L1 communication.
  bytes32 public constant PAUSE_L2_L1_ROLE = keccak256("PAUSE_L2_L1_ROLE");

  /// @notice This is used to unpause L2 to L1 communication.
  bytes32 public constant UNPAUSE_L2_L1_ROLE = keccak256("UNPAUSE_L2_L1_ROLE");

  /// @notice This is used to pause blob submission.
  bytes32 public constant PAUSE_BLOB_SUBMISSION_ROLE = keccak256("PAUSE_BLOB_SUBMISSION_ROLE");

  /// @notice This is used to unpause blob submission.
  bytes32 public constant UNPAUSE_BLOB_SUBMISSION_ROLE = keccak256("UNPAUSE_BLOB_SUBMISSION_ROLE");

  /// @notice This is used to pause finalization submission.
  bytes32 public constant PAUSE_FINALIZATION_ROLE = keccak256("PAUSE_FINALIZATION_ROLE");

  /// @notice This is used to unpause finalization submission.
  bytes32 public constant UNPAUSE_FINALIZATION_ROLE = keccak256("UNPAUSE_FINALIZATION_ROLE");
}
