// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { ILineaRollupBase } from "./ILineaRollupBase.sol";
/**
 * @title LineaRollup interface for current functions, structs, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ILineaRollup is ILineaRollupBase {
  /**
   * @notice Initialization data structure for the LineaRollup contract.
   * @param baseInitializationData The initial state root hash at initialization used for proof verification.
   * @param livenessRecoveryOperator The account to be given OPERATOR_ROLE on when the time since last finalization lapses.
   */
  struct InitializationData {
    BaseInitializationData baseInitializationData;
    address livenessRecoveryOperator;
  }
}
