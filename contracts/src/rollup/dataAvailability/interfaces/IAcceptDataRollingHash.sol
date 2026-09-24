// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;
import { IDataRollingHashAcceptorBase } from "./IDataRollingHashAcceptorBase.sol";

/**
 * @title Interface to define a simple dataRollingHash acceptance definition.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptDataRollingHash is IDataRollingHashAcceptorBase {
  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _storedDataRollingHash The dataRollingHash to anchor.
   */
  function acceptDataRollingHash(bytes32 _parentDataRollingHash, bytes32 _storedDataRollingHash) external;
}
