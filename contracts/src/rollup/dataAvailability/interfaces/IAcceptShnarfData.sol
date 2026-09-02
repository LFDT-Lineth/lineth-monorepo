// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;
import { IShnarfDataAcceptorBase } from "./IShnarfDataAcceptorBase.sol";

/**
 * @title Interface to define a simple dataRollingHash acceptance definition.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptShnarfData is IShnarfDataAcceptorBase {
  /**
   * @notice Accepts and anchors that a dataRollingHash exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentDataRollingHash The parent dataRollingHash used in continuity checks.
   * @param _dataRollingHash The dataRollingHash to anchor.
   */
  function acceptShnarfData(bytes32 _parentDataRollingHash, bytes32 _dataRollingHash) external;
}
