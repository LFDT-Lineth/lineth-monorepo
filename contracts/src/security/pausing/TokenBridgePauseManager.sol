// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { PauseManager } from "./PauseManager.sol";

/**
 * @title Contract to manage cross-chain function pausing roles for the Token Bridge.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract TokenBridgePauseManager is PauseManager {
  /// @notice This is used to pause token bridging initiation.
  bytes32 public constant PAUSE_INITIATE_TOKEN_BRIDGING_ROLE = keccak256("PAUSE_INITIATE_TOKEN_BRIDGING_ROLE");

  /// @notice This is used to unpause token bridging initiation.
  bytes32 public constant UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE = keccak256("UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE");

  /// @notice This is used to pause token bridging completion.
  bytes32 public constant PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = keccak256("PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");

  /// @notice This is used to unpause token bridging completion.
  bytes32 public constant UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = keccak256("UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");
}
