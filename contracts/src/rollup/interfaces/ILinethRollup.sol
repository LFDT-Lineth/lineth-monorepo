// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { ILinethRollupBase } from "./ILinethRollupBase.sol";
/**
 * @title LinethRollup interface for current functions, structs, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ILinethRollup is ILinethRollupBase {
  /**
   * @notice Initialization data structure for the LinethRollup contract.
   * @param baseInitializationData The data used at initialization of the shared base contract.
   * @param livenessRecoveryOperator The account to be given OPERATOR_ROLE on when the time since last finalization lapses.
   */
  struct InitializationData {
    BaseInitializationData baseInitializationData;
    address livenessRecoveryOperator;
  }
}
