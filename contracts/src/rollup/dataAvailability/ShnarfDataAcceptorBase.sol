// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.33;

import { IShnarfDataAcceptorBase } from "./interfaces/IShnarfDataAcceptorBase.sol";
import { LineaRollupBase } from "../LineaRollupBase.sol";

/**
 * @title Contract to manage shared functions for shnarf accepting and storing.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract ShnarfDataAcceptorBase is LineaRollupBase, IShnarfDataAcceptorBase {
  /// @dev Value indicating a shnarf exists.
  uint256 internal constant SHNARF_EXISTS_DEFAULT_VALUE = 1;

  /**
   * @notice Accepts and stores that a shnarf exists.
   * @param _parentShnarf The parent shnarf.
   * @param _shnarf The shnarf to indicate exists.
   * @param _finalBlockHash The final L2 block hash in the data.
   */
  function _acceptShnarfData(bytes32 _parentShnarf, bytes32 _shnarf, bytes32 _finalBlockHash) internal virtual {
    require(_blobShnarfExists[_parentShnarf] != 0, ParentShnarfNotSubmitted(_parentShnarf));
    require(_blobShnarfExists[_shnarf] == 0, ShnarfAlreadySubmitted(_shnarf));

    _blobShnarfExists[_shnarf] = SHNARF_EXISTS_DEFAULT_VALUE;

    emit DataSubmittedV4(_parentShnarf, _shnarf, _finalBlockHash);
  }
}
