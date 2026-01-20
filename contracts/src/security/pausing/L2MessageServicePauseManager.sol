// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { PauseManager } from "./PauseManager.sol";

/**
 * @title Contract to manage cross-chain function pausing roles for the L2 Message Service.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L2MessageServicePauseManager is PauseManager {
  /// @notice This is used to pause L1 to L2 communication.
  bytes32 public constant PAUSE_L1_L2_ROLE = keccak256("PAUSE_L1_L2_ROLE");

  /// @notice This is used to unpause L1 to L2 communication.
  bytes32 public constant UNPAUSE_L1_L2_ROLE = keccak256("UNPAUSE_L1_L2_ROLE");

  /// @notice This is used to pause L2 to L1 communication.
  bytes32 public constant PAUSE_L2_L1_ROLE = keccak256("PAUSE_L2_L1_ROLE");

  /// @notice This is used to unpause L2 to L1 communication.
  bytes32 public constant UNPAUSE_L2_L1_ROLE = keccak256("UNPAUSE_L2_L1_ROLE");
}
