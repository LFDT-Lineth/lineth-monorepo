// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { PauseManager } from "./PauseManager.sol";

/**
 * @title Contract to manage pausing roles for the YieldManager.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract YieldManagerPauseManager is PauseManager {
  /// @notice This is used to pause native-yield driven funding of external strategies.
  bytes32 public constant PAUSE_NATIVE_YIELD_STAKING_ROLE = keccak256("PAUSE_NATIVE_YIELD_STAKING_ROLE");

  /// @notice This is used to unpause native-yield driven funding of external strategies.
  bytes32 public constant UNPAUSE_NATIVE_YIELD_STAKING_ROLE = keccak256("UNPAUSE_NATIVE_YIELD_STAKING_ROLE");

  /// @notice This is used to pause operator-led unstaking flows and reserve funding to the L1MessageService.
  bytes32 public constant PAUSE_NATIVE_YIELD_UNSTAKING_ROLE = keccak256("PAUSE_NATIVE_YIELD_UNSTAKING_ROLE");

  /// @notice This is used to unpause operator-led unstaking flows and reserve funding to the L1MessageService.
  bytes32 public constant UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE = keccak256("UNPAUSE_NATIVE_YIELD_UNSTAKING_ROLE");

  /// @notice This is used to pause permissionless actions such as unstaking requests and reserve replenishment flows.
  bytes32 public constant PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE =
    keccak256("PAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE");

  /// @notice This is used to unpause permissionless actions such as unstaking requests and reserve replenishment flows.
  bytes32 public constant UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE =
    keccak256("UNPAUSE_NATIVE_YIELD_PERMISSIONLESS_ACTIONS_ROLE");

  /// @notice This is used to pause donation flows routed through the YieldManager.
  bytes32 public constant PAUSE_NATIVE_YIELD_DONATION_ROLE = keccak256("PAUSE_NATIVE_YIELD_DONATION_ROLE");

  /// @notice This is used to unpause donation flows routed through the YieldManager.
  bytes32 public constant UNPAUSE_NATIVE_YIELD_DONATION_ROLE = keccak256("UNPAUSE_NATIVE_YIELD_DONATION_ROLE");

  /// @notice This is used to pause yield-reporting operations.
  bytes32 public constant PAUSE_NATIVE_YIELD_REPORTING_ROLE = keccak256("PAUSE_NATIVE_YIELD_REPORTING_ROLE");

  /// @notice This is used to unpause yield-reporting operations.
  bytes32 public constant UNPAUSE_NATIVE_YIELD_REPORTING_ROLE = keccak256("UNPAUSE_NATIVE_YIELD_REPORTING_ROLE");
}
