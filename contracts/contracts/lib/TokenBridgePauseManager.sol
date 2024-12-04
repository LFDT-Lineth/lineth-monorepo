// SPDX-License-Identifier: AGPL-3.0
pragma solidity >=0.8.19 <=0.8.26;

import { PauseManager } from "./PauseManager.sol";

/**
 * @title Contract to manage cross-chain function pausing roles for the Token Bridge.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract TokenBridgePauseManager is PauseManager {
  bytes32 public constant PAUSE_INITIATE_TOKEN_BRIDGING_ROLE = keccak256("PAUSE_INITIATE_TOKEN_BRIDGING_ROLE");
  bytes32 public constant UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE = keccak256("UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE");
  bytes32 public constant PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = keccak256("PAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");
  bytes32 public constant UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE = keccak256("UNPAUSE_COMPLETE_TOKEN_BRIDGING_ROLE");
}
