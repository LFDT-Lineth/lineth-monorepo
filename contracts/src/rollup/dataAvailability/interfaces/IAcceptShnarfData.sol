// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;
import { IShnarfDataAcceptorBase } from "./IShnarfDataAcceptorBase.sol";

/**
 * @title Interface to define a simple shnarf acceptance definition.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAcceptShnarfData is IShnarfDataAcceptorBase {
  /**
   * @notice Accepts and stores that a shnarf exists.
   * @dev OPERATOR_ROLE is required to execute.
   * @param _parentShnarf The parent shnarf.
   * @param _shnarf The shnarf to indicate exists.
   * @param _finalStateRootHash The final state root hash in the data.
   */
  function acceptShnarfData(bytes32 _parentShnarf, bytes32 _shnarf, bytes32 _finalStateRootHash) external;
}
